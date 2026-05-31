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
        "model": os.getenv("LLM_MODEL", "deepseek-chat"),
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


def _memory_context_text(memories: list[dict[str, Any]]) -> str:
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
    try:
        point_id = save_study_plan_memory(user_id=user_id, goal=goal, result=result)
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

    return [
        StructuredTool.from_function(func=user_ac_history, args_schema=UserIDArgs),
        StructuredTool.from_function(func=user_failed_submissions, args_schema=FailedSubmissionArgs),
        StructuredTool.from_function(func=user_tag_stats, args_schema=UserIDArgs),
        StructuredTool.from_function(func=candidate_problems, args_schema=RuleCandidateArgs),
        StructuredTool.from_function(func=semantic_candidate_problems, args_schema=SemanticCandidateArgs),
    ]


def _extract_result_text(result: dict[str, Any]) -> str:
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


def run_study_plan_with_langchain(user_id: int, goal: str, token: str) -> StudyPlanResult:
    debug_enabled = _is_debug_enabled()
    memory_context = _load_memory_context(user_id=user_id, goal=goal, debug_enabled=debug_enabled)
    context = ToolExecutionContext()

    agent = create_agent(
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

    user_prompt = (
        f"Generate a study plan for user_id={user_id}. "
        f"User goal: {goal or 'No explicit goal provided.'}"
    )

    messages: list[dict[str, str]] = [{"role": "user", "content": user_prompt}]
    if memory_context:
        messages.append({"role": "user", "content": memory_context})

    if debug_enabled:
        print(f"[agent] start langchain deepseek agent user_id={user_id} goal={goal!r}")

    result = agent.invoke({"messages": messages})

    if debug_enabled:
        print(f"[agent] invoke finished keys={list(result.keys())}")

    final_text = _extract_result_text(result)
    parsed = _parse_plain_text_result(final_text)
    _save_memory(user_id=user_id, goal=goal, result=parsed, debug_enabled=debug_enabled)
    return parsed
