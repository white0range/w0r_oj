import os
import re
from typing import Any

from langchain.agents import create_agent
from langchain_core.tools import StructuredTool
from langchain_deepseek import ChatDeepSeek
from pydantic import BaseModel, Field

from rag.memory_service import save_study_plan_memory, search_user_memories
from schemas import RecommendedProblem, StudyPlanResult
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


class UserIDArgs(BaseModel):
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


class HybridCandidateArgs(BaseModel):
    query: str = Field(description="Natural language retrieval query that should be normalized and searched with hybrid recall")
    exclude_ids: list[int] = Field(description="Problem ids to exclude")
    limit: int = Field(default=10, ge=1, le=20, description="Maximum number of candidate problems to return")


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
        return "Assistant"
    if role == "system":
        return "System"
    return "User"


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

    lines = ["Relevant long-term memories for this user:"]
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


def _save_memory(user_id: int, query_text: str, result: StudyPlanResult, debug_enabled: bool) -> None:
    try:
        point_id = save_study_plan_memory(
            user_id=user_id,
            query_text=query_text,
            result=result,
            memory_kind="study_plan_chat",
        )
    except Exception as e:
        if debug_enabled:
            print(f"[agent] memory save skipped due to error: {e}")
        return

    if debug_enabled:
        print(f"[agent] memory saved point_id={point_id}")


def _build_tools(token: str, context: ToolExecutionContext) -> list[StructuredTool]:
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

    def hybrid_candidate_problems(query: str, exclude_ids: list[int], limit: int = 10) -> dict:
        """Normalize the query and retrieve candidate problems with lexical plus semantic hybrid recall."""
        return execute_tool(
            "hybrid_candidate_problems",
            {
                "query": query,
                "exclude_ids": exclude_ids,
                "limit": limit,
            },
            token,
            context,
        )

    return [
        StructuredTool.from_function(func=user_ac_history, args_schema=UserIDArgs),
        StructuredTool.from_function(func=user_failed_submissions, args_schema=FailedSubmissionArgs),
        StructuredTool.from_function(func=user_tag_stats, args_schema=UserIDArgs),
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


def _parse_plain_text_result(text: str) -> StudyPlanResult:
    text = text.strip()
    if not text:
        raise ValueError("empty final response")

    weak_match = re.search(r"WEAK_TAGS:\s*(.*?)(?:\nRECOMMENDED_PROBLEMS:|\Z)", text, flags=re.S)
    recommended_match = re.search(r"RECOMMENDED_PROBLEMS:\s*(.*?)(?:\nSUMMARY:|\Z)", text, flags=re.S)
    summary_match = re.search(r"SUMMARY:\s*(.*)\Z", text, flags=re.S)

    weak_tags: list[str] = []
    if weak_match:
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
                    f"Continue the OJ learning conversation for user_id={user_id}. "
                    "Answer the latest user message directly, and only recommend problems when it is useful."
                ),
            }
        )
        if session_summary.strip():
            runtime_messages.append(
                {
                    "role": "user",
                    "content": f"Short-term session summary:\n{session_summary.strip()}",
                }
            )
        if memory_context:
            runtime_messages.append({"role": "user", "content": memory_context})
        runtime_messages.extend(_normalize_chat_messages(messages))
        return runtime_messages

    runtime_messages.append(
        {
            "role": "user",
            "content": f"Generate a study plan for user_id={user_id}. User goal: {goal or 'No explicit goal provided.'}",
        }
    )
    if memory_context:
        runtime_messages.append({"role": "user", "content": memory_context})
    return runtime_messages


def summarize_study_plan_session(existing_summary: str = "", messages: list[dict[str, Any]] | None = None) -> str:
    normalized_messages = _normalize_chat_messages(messages or [])
    transcript = _messages_to_transcript(normalized_messages)
    if not transcript:
        return _truncate_text(existing_summary.strip())

    prompt = (
        "You compress older OJ tutoring conversation turns into short-term memory. "
        "Merge the existing summary and the newly archived messages into one concise plain-text summary. "
        "Keep only durable facts that will help future turns: user goals, weak topics, constraints, solved issues, preferences, and any concrete recommended directions. "
        "Do not mention timestamps, pleasantries, or that this is a summary. "
        "Output plain text only.\n\n"
        f"Existing summary:\n{existing_summary.strip() or 'none'}\n\n"
        f"New archived messages:\n{transcript}\n"
    )

    response = _build_model().invoke(prompt)
    summary = _truncate_text(_message_content_to_text(getattr(response, "content", response)))
    if summary:
        return summary

    fallback = existing_summary.strip()
    if transcript:
        fallback = f"{fallback}\n{transcript}".strip()
    return _truncate_text(fallback)


def run_study_plan_with_langchain(
    user_id: int,
    goal: str,
    token: str,
    session_summary: str = "",
    messages: list[dict[str, Any]] | None = None,
) -> StudyPlanResult:
    debug_enabled = _is_debug_enabled()
    normalized_messages = messages or []
    latest_user_query = _latest_user_query(goal, normalized_messages) or "general oj study chat"
    memory_context = _load_memory_context(user_id=user_id, query_text=latest_user_query, debug_enabled=debug_enabled)
    context = ToolExecutionContext()

    agent = create_agent(
        model=_build_model(),
        tools=_build_tools(token, context),
        system_prompt=(
            "You are an OJ chat assistant. "
            "Support both direct algorithm/OJ Q&A and personalized training recommendations. "
            "If the latest user message is a basic question, answer it clearly and keep recommendations empty unless they truly help. "
            "If the latest user message asks for practice suggestions or a study plan, personalize the reply with tools. "
            "Available tools include user history tools, a rule-based candidate retrieval tool, a semantic retrieval tool, and a hybrid lexical+semantic retrieval tool. "
            "Prefer hybrid_candidate_problems for natural-language retrieval, candidate_problems when tags are explicit, and semantic_candidate_problems only when hybrid retrieval is still insufficient. "
            "Use session summary and long-term memory only as auxiliary context, and prioritize the latest user request. "
            "Helpful normalized query variants may include: "
            f"{', '.join(build_query_variants(latest_user_query))}. "
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
