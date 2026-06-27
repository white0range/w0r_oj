import os
import re
from typing import Any

from langchain.agents import create_agent
from langchain_deepseek import ChatDeepSeek
from langchain_core.tools import StructuredTool
from pydantic import BaseModel, Field

from rag.memory_service import save_study_plan_memory, search_user_memories
from schemas import RecommendedProblem, StudyPlanResult
from tool_executor import ToolExecutionContext, execute_tool


def _is_debug_enabled() -> bool:
    # 统一读取调试开关，避免日志逻辑散落在代码里难以维护。
    return os.getenv("AGENT_DEBUG", "").strip().lower() in {"1", "true", "yes", "on"}


def _normalize_base_url() -> str | None:
    # DeepSeek/OpenAI 兼容网关有时会带 /beta 后缀，但不同 SDK 是否接受这个后缀并不一致。
    # 这里统一做一次归一化，尽量降低 provider URL 差异带来的运行时问题。
    base_url = os.getenv("DEEPSEEK_API_BASE") or os.getenv("OPENAI_API_BASE") or ""
    base_url = base_url.strip().rstrip("/")
    if not base_url:
        return None
    if base_url.endswith("/beta"):
        return base_url[: -len("/beta")]
    return base_url


def _build_model() -> ChatDeepSeek:
    # 构建模型客户端是 Runtime 的第一步：
    # 这里读取 key / model / base_url，并把温度固定为 0，
    # 让 Tool Calling 路径更稳定、更可复现。
    api_key = os.getenv("DEEPSEEK_API_KEY") or os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise ValueError("Please set DEEPSEEK_API_KEY or OPENAI_API_KEY first.")

    kwargs: dict[str, Any] = {
        "model": os.getenv("LLM_MODEL", "deepseek-chat"),
        "api_key": api_key,
        "temperature": 0,
    }
    base_url = _normalize_base_url()
    if base_url:
        kwargs["base_url"] = base_url
    return ChatDeepSeek(**kwargs)


class UserIDArgs(BaseModel):
    # LangChain 的 StructuredTool 需要显式参数 schema，
    # 这样模型在调用 tool 时才能知道每个参数的名字和含义。
    user_id: int = Field(description="User id")


class FailedSubmissionArgs(BaseModel):
    user_id: int = Field(description="User id")
    limit: int = Field(default=10, ge=1, le=20, description="Maximum number of failed submissions to return")


class RuleCandidateArgs(BaseModel):
    tags: list[str] = Field(description="Target tags for rule-based candidate retrieval")
    exclude_ids: list[int] = Field(description="Problem ids to exclude")
    limit: int = Field(default=10, ge=1, le=20, description="Maximum number of candidate problems to return")


class SemanticCandidateArgs(BaseModel):
    query: str = Field(description="Natural language retrieval query")
    exclude_ids: list[int] = Field(description="Problem ids to exclude")
    limit: int = Field(default=10, ge=1, le=20, description="Maximum number of semantic candidate problems to return")


def _memory_context_text(memories: list[dict[str, Any]]) -> str:
    # 把向量检索回来的 memory 结果压成一段纯文本上下文，
    # 再交给模型使用。这里不直接把原始 payload 暴露给模型，
    # 是为了控制上下文格式并减少提示词噪声。
    if not memories:
        return ""

    lines = ["Relevant past study-plan memories for this user:"]
    for index, memory in enumerate(memories, start=1):
        lines.append(
            f"{index}. score={memory.get('score', 0):.4f}, "
            f"goal={memory.get('goal', '')}, "
            f"memory={memory.get('memory_text', '')}"
        )
    return "\n".join(lines)


def _load_memory_context(user_id: int, goal: str, debug_enabled: bool) -> str:
    # memory 是 best-effort 能力：
    # - 查到了就增强上下文
    # - 查不到或者 collection 不存在，也不应阻断主业务
    # 这样可以保证学习规划主链对 memory 故障不敏感。
    query = goal.strip() or "general study plan"
    try:
        memories = search_user_memories(user_id=user_id, query=query)
    except Exception as e:
        if debug_enabled:
            print(f"[agent] memory lookup skipped due to error: {e}")
        return ""

    if debug_enabled:
        print(f"[agent] memory hits={len(memories)}")

    return _memory_context_text(memories)


def _save_memory(user_id: int, goal: str, result: StudyPlanResult, debug_enabled: bool) -> None:
    # 保存 memory 同样是 best-effort：
    # 当前请求的核心目标是返回 study plan，
    # memory 写入失败不应该让用户请求整体失败。
    try:
        point_id = save_study_plan_memory(user_id=user_id, goal=goal, result=result)
    except Exception as e:
        if debug_enabled:
            print(f"[agent] memory save skipped due to error: {e}")
        return

    if debug_enabled:
        print(f"[agent] memory saved point_id={point_id}")


def _build_tools(token: str, context: ToolExecutionContext) -> list[StructuredTool]:
    # 这里是“把现有系统能力暴露给模型”的关键步骤。
    # 每个内层函数都很薄：只负责把 LangChain 的 tool 调用转成统一的 execute_tool(...) 分发。
    # 这样 Tool 定义层和 Tool 执行层就解耦了。
    def user_ac_history(user_id: int) -> dict:
        """Get the user's solved history, including solved count and solved problem ids."""
        return execute_tool("user_ac_history", {"user_id": user_id}, token, context)

    def user_failed_submissions(user_id: int, limit: int = 10) -> dict:
        """Get the user's recent failed submissions."""
        return execute_tool("user_failed_submissions", {"user_id": user_id, "limit": limit}, token, context)

    def user_tag_stats(user_id: int) -> dict:
        """Get the user's training statistics grouped by tag."""
        return execute_tool("user_tag_stats", {"user_id": user_id}, token, context)

    def candidate_problems(tags: list[str], exclude_ids: list[int], limit: int = 10) -> dict:
        """Get candidate problems by explicit tags and exclude solved problem ids."""
        return execute_tool(
            "candidate_problems",
            {
                "tags": tags,
                "exclude_ids": exclude_ids,
                "limit": limit,
            },
            token,
            context,
        )

    def semantic_candidate_problems(query: str, exclude_ids: list[int], limit: int = 10) -> dict:
        """Retrieve semantically related candidate problems from the vector index using a natural language query."""
        return execute_tool(
            "semantic_candidate_problems",
            {
                "query": query,
                "exclude_ids": exclude_ids,
                "limit": limit,
            },
            token,
            context,
        )

    return [
        # StructuredTool.from_function 会把 Python 函数 + 参数 schema 封装成 LangChain 可调用工具。
        # 这些工具最终会出现在 create_agent(...) 的可用工具集合里。
        StructuredTool.from_function(func=user_ac_history, args_schema=UserIDArgs),
        StructuredTool.from_function(func=user_failed_submissions, args_schema=FailedSubmissionArgs),
        StructuredTool.from_function(func=user_tag_stats, args_schema=UserIDArgs),
        StructuredTool.from_function(func=candidate_problems, args_schema=RuleCandidateArgs),
        StructuredTool.from_function(func=semantic_candidate_problems, args_schema=SemanticCandidateArgs),
    ]


def _extract_result_text(result: dict[str, Any]) -> str:
    # LangChain invoke 的返回结果里通常会带 messages。
    # 我们只关心最后一条模型消息里的纯文本内容，再把它交给本地解析器。
    messages = result.get("messages") or []
    if not messages:
        return ""
    content = getattr(messages[-1], "content", "")
    if isinstance(content, str):
        return content.strip()
    if isinstance(content, list):
        parts = []
        for item in content:
            if isinstance(item, dict) and item.get("type") == "text":
                parts.append(item.get("text", ""))
        return "\n".join(parts).strip()
    return ""


def _parse_plain_text_result(text: str) -> StudyPlanResult:
    # 这一段是“去掉框架级 structured output 后，仍然拿到结构化结果”的关键。
    # 策略是：
    # 1. 用 prompt 强约束输出固定模板
    # 2. 在本地用正则 + 字段解析，把文本结果转回 Pydantic 对象
    # 这样通常比 provider 原生 schema 输出更稳。
    text = text.strip()
    if not text:
        raise ValueError("empty final response")

    # 先按大段切出三个区域：
    # - WEAK_TAGS
    # - RECOMMENDED_PROBLEMS
    # - SUMMARY
    weak_match = re.search(r"WEAK_TAGS:\s*(.*?)(?:\nRECOMMENDED_PROBLEMS:|\Z)", text, flags=re.S)
    recommended_match = re.search(r"RECOMMENDED_PROBLEMS:\s*(.*?)(?:\nSUMMARY:|\Z)", text, flags=re.S)
    summary_match = re.search(r"SUMMARY:\s*(.*)\Z", text, flags=re.S)

    weak_tags: list[str] = []
    if weak_match:
        # 允许模型用逗号分隔多个弱项标签，NONE 表示没有明确弱项。
        raw = weak_match.group(1).strip()
        if raw and raw.upper() != "NONE":
            weak_tags = [item.strip() for item in raw.split(",") if item.strip()]

    recommended_problems: list[RecommendedProblem] = []
    if recommended_match:
        block = recommended_match.group(1).strip()
        for line in block.splitlines():
            line = line.strip()
            if not line.startswith("-"):
                continue
            # 每道推荐题都要求按：
            # - problem_id=1; title=...; reason=...
            # 这种结构输出，便于本地解析。
            payload = line.removeprefix("-").strip()
            parts = [part.strip() for part in payload.split(";") if part.strip()]
            item: dict[str, str] = {}
            for part in parts:
                if "=" not in part:
                    continue
                key, value = part.split("=", 1)
                item[key.strip()] = value.strip()
            if {"problem_id", "title", "reason"} <= item.keys():
                recommended_problems.append(
                    RecommendedProblem(
                        problem_id=int(item["problem_id"]),
                        title=item["title"],
                        reason=item["reason"],
                    )
                )

    summary = summary_match.group(1).strip() if summary_match else text

    return StudyPlanResult(
        weak_tags=weak_tags,
        recommended_problems=recommended_problems,
        study_plan_summary=summary,
    )


def run_study_plan_with_langchain(user_id: int, goal: str, token: str) -> StudyPlanResult:
    # 这是学习规划 Agent 的主流程：
    # 1. 读调试开关
    # 2. 读取该用户的相关历史记忆
    # 3. 初始化本次请求的 tool 执行上下文
    # 4. 创建 LangChain Agent（模型 + tools + system prompt）
    # 5. 组织 user prompt 和 memory 上下文
    # 6. 调 agent.invoke 让模型自己决定如何调用工具
    # 7. 从最终消息里取文本结果并解析成 StudyPlanResult
    # 8. 把这次结果写回 memory，供未来请求使用
    debug_enabled = _is_debug_enabled()
    memory_context = _load_memory_context(user_id=user_id, goal=goal, debug_enabled=debug_enabled)
    context = ToolExecutionContext()

    agent = create_agent(
        # create_agent 是 LangChain 帮我们管理 Tool Calling 循环的入口。
        # 它并不决定具体业务逻辑；业务数据、工具语义、RAG 和 memory 仍然由我们自己控制。
        model=_build_model(),
        tools=_build_tools(token, context),
        system_prompt=(
            "You are an OJ study plan assistant. "
            "Decide which tools are needed to build a personalized study plan. "
            "Available tools include user history tools, a rule-based candidate retrieval tool, and a semantic vector retrieval tool. "
            "Use tools only when necessary, avoid repeated calls with different parameters unless the previous result is clearly insufficient. "
            "If past study-plan memory is provided, use it as auxiliary context but still prioritize the latest tool results. "
            "When you finish, do not call any special finish tool. "
            "Instead, respond in this exact plain-text format:\n"
            "WEAK_TAGS: tag1, tag2\n"
            "RECOMMENDED_PROBLEMS:\n"
            "- problem_id=1; title=Title A; reason=Why it is recommended\n"
            "- problem_id=2; title=Title B; reason=Why it is recommended\n"
            "SUMMARY:\n"
            "One concise paragraph summary.\n"
            "If there are no recommended problems, keep the RECOMMENDED_PROBLEMS section empty."
        ),
    )

    # 第一条 user prompt 负责明确本次任务目标。
    user_prompt = (
        f"Generate a study plan for user_id={user_id}. "
        f"User goal: {goal or 'No explicit goal provided.'}"
    )

    messages: list[dict[str, str]] = [{"role": "user", "content": user_prompt}]
    if memory_context:
        # memory 不作为 system prompt，而是作为额外 user message 注入，
        # 这样模型能把它当作“辅助上下文”，而不是硬约束。
        messages.append({"role": "user", "content": memory_context})

    if debug_enabled:
        print(f"[agent] start langchain deepseek agent user_id={user_id} goal={goal!r}")

    # 这里是整个 Agent 真正开始“思考 + 调工具”的地方。
    # Agent 会根据 system prompt 和当前消息，自主决定先查什么、要不要走 rule retrieval、要不要走 semantic retrieval。
    result = agent.invoke({"messages": messages})

    if debug_enabled:
        print(f"[agent] invoke finished keys={list(result.keys())}")

    # invoke 返回后，再由我们自己接管最终结果抽取和解析，
    # 这就是当前这套“轻量 LangChain runtime”的核心稳定性策略。
    final_text = _extract_result_text(result)
    parsed = _parse_plain_text_result(final_text)
    _save_memory(user_id=user_id, goal=goal, result=parsed, debug_enabled=debug_enabled)
    return parsed
