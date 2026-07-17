import os
import re
from typing import Any

from langchain.agents import create_agent
from langchain_core.tools import StructuredTool
from langchain_deepseek import ChatDeepSeek
from pydantic import BaseModel, Field

from rag.memory_service import save_chat_memory, search_user_memories
from schemas import ChatResult, RecommendedProblem
from tool_executor import ToolExecutionContext, build_query_variants, execute_tool


SUMMARY_MAX_CHARS = 1800


def _is_debug_enabled() -> bool:
    return os.getenv("AGENT_DEBUG", "").strip().lower() in {"1", "true", "yes", "on"}


def _normalize_base_url() -> str | None:
    base_url = os.getenv("DEEPSEEK_API_BASE") or os.getenv("OPENAI_API_BASE") or ""
    base_url = base_url.strip().rstrip("/")
    if not base_url:
        return None
    if base_url.endswith("/beta"):
        return base_url[: -len("/beta")]
    return base_url


def _build_model() -> ChatDeepSeek:
    api_key = os.getenv("DEEPSEEK_API_KEY") or os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise ValueError("Please set DEEPSEEK_API_KEY or OPENAI_API_KEY first.")

    kwargs: dict[str, Any] = {
        "model": os.getenv("LLM_MODEL", "deepseek-v4-pro"),
        "api_key": api_key,
        "temperature": 0,
    }
    base_url = _normalize_base_url()
    if base_url:
        kwargs["base_url"] = base_url
    return ChatDeepSeek(**kwargs)


class EmptyArgs(BaseModel):
    pass


class FailedSubmissionArgs(BaseModel):
    limit: int = Field(default=10, ge=1, le=20, description="返回的失败提交记录最大数量")


class RuleCandidateArgs(BaseModel):
    tags: list[str] = Field(description="用于规则候选题检索的目标标签")
    exclude_ids: list[int] = Field(description="需要排除的题目 ID")
    limit: int = Field(default=10, ge=1, le=20, description="返回的候选题目最大数量")


class SemanticCandidateArgs(BaseModel):
    query: str = Field(description="自然语言检索查询")
    exclude_ids: list[int] = Field(description="需要排除的题目 ID")
    limit: int = Field(default=10, ge=1, le=20, description="返回的语义候选题目最大数量")


class HybridCandidateArgs(BaseModel):
    query: str = Field(description="应进行标准化并使用混合召回检索的自然语言查询")
    exclude_ids: list[int] = Field(description="需要排除的题目 ID")
    limit: int = Field(default=10, ge=1, le=20, description="返回的候选题目最大数量")


def _message_content_to_text(content: Any) -> str:
    if isinstance(content, str):
        return content.strip()
    if isinstance(content, list):
        parts = []
        for item in content:
            if isinstance(item, dict) and item.get("type") == "text":
                parts.append(item.get("text", ""))
        return "\n".join(parts).strip()
    return ""


def _collapse_whitespace(text: str) -> str:
    return " ".join(str(text or "").strip().split())


def _truncate_text(text: str, limit: int = SUMMARY_MAX_CHARS) -> str:
    text = text.strip()
    if len(text) <= limit:
        return text
    if limit <= 3:
        return text[:limit]
    return text[: limit - 3].rstrip() + "..."


def _normalize_chat_messages(messages: list[dict[str, Any]]) -> list[dict[str, str]]:
    normalized: list[dict[str, str]] = []
    for message in messages:
        role = str(message.get("role", "user")).strip().lower()
        if role not in {"user", "assistant", "system"}:
            role = "user"
        normalized.append(
            {
                "role": role,
                "content": str(message.get("content", "")).strip(),
            }
        )
    return normalized


def _role_label(role: str) -> str:
    if role == "assistant":
        return "助手"
    if role == "system":
        return "系统"
    return "用户"


def _messages_to_transcript(messages: list[dict[str, Any]]) -> str:
    lines = []
    for message in _normalize_chat_messages(messages):
        content = _collapse_whitespace(message.get("content", ""))
        if not content:
            continue
        lines.append(f"{_role_label(message['role'])}: {content}")
    return "\n".join(lines)


def _memory_context_text(memories: list[dict[str, Any]]) -> str:
    if not memories:
        return ""

    lines = ["该用户的相关长期记忆："]
    for index, memory in enumerate(memories, start=1):
        lines.append(
            f"{index}. score={memory.get('score', 0):.4f}, "
            f"kind={memory.get('memory_kind', '')}, "
            f"query={memory.get('query_text', '')}, "
            f"memory={memory.get('memory_text', '')}"
        )
    return "\n".join(lines)


def _load_memory_context(user_id: int, query_text: str, debug_enabled: bool) -> str:
    query = query_text.strip() or "general oj study chat"
    try:
        memories = search_user_memories(user_id=user_id, query=query)
    except Exception as e:
        if debug_enabled:
            print(f"[agent] memory lookup skipped due to error: {e}")
        return ""

    if debug_enabled:
        print(f"[agent] memory hits={len(memories)}")

    return _memory_context_text(memories)


def _save_memory(user_id: int, query_text: str, result: ChatResult, debug_enabled: bool) -> None:
    try:
        point_id = save_chat_memory(
            user_id=user_id,
            query_text=query_text,
            result=result,
            memory_kind="chat",
        )
    except Exception as e:
        if debug_enabled:
            print(f"[agent] memory save skipped due to error: {e}")
        return

    if debug_enabled:
        print(f"[agent] memory saved point_id={point_id}")


def _build_tools(token: str, context: ToolExecutionContext, bound_user_id: int) -> list[StructuredTool]:
    def user_ac_history() -> dict:
        """获取用户的已解决历史，包括解题数量和已解决题目 ID。"""
        return execute_tool("user_ac_history", {}, token, context, bound_user_id)

    def user_failed_submissions(limit: int = 10) -> dict:
        """获取用户最近的失败提交记录。"""
        return execute_tool("user_failed_submissions", {"limit": limit}, token, context, bound_user_id)

    def user_tag_stats() -> dict:
        """获取按标签聚合的用户训练统计。"""
        return execute_tool("user_tag_stats", {}, token, context, bound_user_id)

    def candidate_problems(tags: list[str], exclude_ids: list[int], limit: int = 10) -> dict:
        """根据明确标签获取候选题目，并排除已解决题目 ID。"""
        return execute_tool(
            "candidate_problems",
            {
                "tags": tags,
                "exclude_ids": exclude_ids,
                "limit": limit,
            },
            token,
            context,
            bound_user_id,
        )

    def semantic_candidate_problems(query: str, exclude_ids: list[int], limit: int = 10) -> dict:
        """使用自然语言查询从向量索引检索语义相关的候选题目。"""
        return execute_tool(
            "semantic_candidate_problems",
            {
                "query": query,
                "exclude_ids": exclude_ids,
                "limit": limit,
            },
            token,
            context,
            bound_user_id,
        )

    def hybrid_candidate_problems(query: str, exclude_ids: list[int], limit: int = 10) -> dict:
        """标准化查询，并通过关键词加语义的混合召回检索候选题目。"""
        return execute_tool(
            "hybrid_candidate_problems",
            {
                "query": query,
                "exclude_ids": exclude_ids,
                "limit": limit,
            },
            token,
            context,
            bound_user_id,
        )

    return [
        StructuredTool.from_function(func=user_ac_history, args_schema=EmptyArgs),
        StructuredTool.from_function(func=user_failed_submissions, args_schema=FailedSubmissionArgs),
        StructuredTool.from_function(func=user_tag_stats, args_schema=EmptyArgs),
        StructuredTool.from_function(func=candidate_problems, args_schema=RuleCandidateArgs),
        StructuredTool.from_function(func=semantic_candidate_problems, args_schema=SemanticCandidateArgs),
        StructuredTool.from_function(func=hybrid_candidate_problems, args_schema=HybridCandidateArgs),
    ]


def _extract_result_text(result: dict[str, Any]) -> str:
    messages = result.get("messages") or []
    if not messages:
        return ""
    content = getattr(messages[-1], "content", "")
    return _message_content_to_text(content)


def _parse_plain_text_result(text: str) -> ChatResult:
    """Parse the agent's constrained text response without making sections mandatory."""
    answer_match = re.search(
        r"^ANSWER:\s*(.*?)(?=^WEAK_TAGS:|\Z)", text, flags=re.MULTILINE | re.DOTALL
    )
    weak_match = re.search(
        r"^WEAK_TAGS:\s*(.*?)(?=^RECOMMENDED_PROBLEMS:|\Z)",
        text,
        flags=re.MULTILINE | re.DOTALL,
    )
    recommended_match = re.search(
        r"^RECOMMENDED_PROBLEMS:\s*(.*)\Z", text, flags=re.MULTILINE | re.DOTALL
    )

    answer = (answer_match.group(1).strip() if answer_match else text.strip())
    weak_tags = [
        tag.strip(" -\t")
        for tag in (weak_match.group(1) if weak_match else "").replace("\n", ",").split(",")
        if tag.strip(" -\t")
    ]

    recommended_problems: list[RecommendedProblem] = []
    for line in (recommended_match.group(1) if recommended_match else "").splitlines():
        parts = [part.strip() for part in line.lstrip("- ").split("|")]
        if len(parts) != 3:
            continue
        try:
            problem_id = int(parts[0])
        except ValueError:
            continue
        recommended_problems.append(
            RecommendedProblem(problem_id=problem_id, title=parts[1], reason=parts[2])
        )

    return ChatResult(
        answer=answer or "暂时无法生成回答，请稍后重试。",
        weak_tags=weak_tags,
        recommended_problems=recommended_problems,
        response_type="practice_plan" if recommended_problems else "qa",
    )
def _latest_user_query(goal: str, messages: list[dict[str, Any]]) -> str:
    for message in reversed(messages):
        if str(message.get("role", "")).strip().lower() == "user":
            content = str(message.get("content", "")).strip()
            if content:
                return content
    return goal.strip()


def _build_runtime_messages(
    user_id: int,
    goal: str,
    session_summary: str,
    messages: list[dict[str, Any]],
    memory_context: str,
) -> list[dict[str, str]]:
    runtime_messages: list[dict[str, str]] = []

    if messages:
        runtime_messages.append(
            {
                "role": "user",
                "content": (
                    f"继续 user_id={user_id} 的 OJ 学习对话。"
                    "直接回答最新的用户消息，仅在确有帮助时推荐题目。"
                ),
            }
        )
        if session_summary.strip():
            runtime_messages.append(
                {
                    "role": "user",
                    "content": f"短期会话摘要：\n{session_summary.strip()}",
                }
            )
        if memory_context:
            runtime_messages.append({"role": "user", "content": memory_context})
        runtime_messages.extend(_normalize_chat_messages(messages))
        return runtime_messages

    runtime_messages.append(
        {
            "role": "user",
            "content": f"继续 user_id={user_id} 的 OJ 学习对话。用户目标：{goal or '未提供明确目标。'}",
        }
    )
    if memory_context:
        runtime_messages.append({"role": "user", "content": memory_context})
    return runtime_messages


def summarize_chat_session(existing_summary: str = "", messages: list[dict[str, Any]] | None = None) -> str:
    normalized_messages = _normalize_chat_messages(messages or [])
    transcript = _messages_to_transcript(normalized_messages)
    if not transcript:
        return _truncate_text(existing_summary.strip())

    prompt = (
        "请将较早的 OJ 辅导对话压缩为短期记忆。"
        "合并已有摘要和新归档消息，生成一段简洁的纯文本摘要。"
        "只保留对后续对话有长期帮助的信息：用户目标、薄弱主题、约束条件、已解决问题、偏好和明确的推荐方向。"
        "不要提及时间戳、客套话，也不要说明这是一份摘要。"
        "只输出纯文本。\n\n"
        f"已有摘要：\n{existing_summary.strip() or '无'}\n\n"
        f"新归档消息：\n{transcript}\n"
    )

    response = _build_model().invoke(prompt)
    summary = _truncate_text(_message_content_to_text(getattr(response, "content", response)))
    if summary:
        return summary

    fallback = existing_summary.strip()
    if transcript:
        fallback = f"{fallback}\n{transcript}".strip()
    return _truncate_text(fallback)


def run_chat_with_langchain(
    user_id: int,
    goal: str,
    token: str,
    session_summary: str = "",
    messages: list[dict[str, Any]] | None = None,
) -> ChatResult:
    debug_enabled = _is_debug_enabled()
    normalized_messages = messages or []
    latest_user_query = _latest_user_query(goal, normalized_messages) or "general oj study chat"
    memory_context = _load_memory_context(user_id=user_id, query_text=latest_user_query, debug_enabled=debug_enabled)
    context = ToolExecutionContext()

    agent = create_agent(
        model=_build_model(),
        tools=_build_tools(token, context, user_id),
        system_prompt=(
            "你是一名 OJ 对话助手。"
            "你既要回答直接的算法或 OJ 问题，也要提供个性化训练建议。"
            "如果最新用户消息是基础问题，请清晰直接地回答；除非推荐确实有帮助，否则保持推荐部分为空。"
            "如果最新用户消息要求练习建议或学习计划，请使用工具生成个性化回复。"
            "可用工具包括用户历史工具、基于规则的候选题检索工具、语义检索工具，以及关键词加语义的混合检索工具。"
            "自然语言检索优先使用 hybrid_candidate_problems；标签明确时使用 candidate_problems；只有混合检索仍不足时才使用 semantic_candidate_problems。"
            "会话摘要和长期记忆只能作为辅助上下文，应优先满足最新用户请求。"
            "可参考的标准化查询变体包括："
            f"{', '.join(build_query_variants(latest_user_query))}。"
            "完成后不要调用任何特殊的结束工具。"
            "请严格使用以下纯文本格式回复：\n"
            "ANSWER:\n"
            "直接回答用户的问题。\n"
            "WEAK_TAGS: 标签1, 标签2\n"
            "RECOMMENDED_PROBLEMS:\n"
            "- 题目 ID | 题目标题 | 推荐原因\n"
            "如果没有推荐题目，请保持 RECOMMENDED_PROBLEMS 区域为空。"
        ),
    )

    runtime_messages = _build_runtime_messages(
        user_id=user_id,
        goal=goal,
        session_summary=session_summary,
        messages=normalized_messages,
        memory_context=memory_context,
    )

    if debug_enabled:
        print(f"[agent] start langchain deepseek agent user_id={user_id} latest_query={latest_user_query!r}")

    result = agent.invoke({"messages": runtime_messages})

    if debug_enabled:
        print(f"[agent] invoke finished keys={list(result.keys())}")

    final_text = _extract_result_text(result)
    parsed = _parse_plain_text_result(final_text)
    _save_memory(user_id=user_id, query_text=latest_user_query, result=parsed, debug_enabled=debug_enabled)
    return parsed
