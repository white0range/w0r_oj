import json
import os
from typing import Any

import requests

from deepseek_tools import build_deepseek_tools
from rag.memory_service import save_study_plan_memory, search_user_memories
from schemas import StudyPlanResult
from tool_executor import ToolExecutionContext, execute_tool


def _is_debug_enabled() -> bool:
    return os.getenv("AGENT_DEBUG", "").strip().lower() in {"1", "true", "yes", "on"}


def _build_base_url() -> str:
    base_url = os.getenv("DEEPSEEK_API_BASE") or os.getenv("OPENAI_API_BASE") or "https://api.deepseek.com/beta"
    base_url = base_url.rstrip("/")
    if base_url == "https://api.deepseek.com":
        return base_url + "/beta"
    return base_url


def _build_headers() -> dict:
    api_key = os.getenv("DEEPSEEK_API_KEY") or os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise ValueError("Please set DEEPSEEK_API_KEY or OPENAI_API_KEY first.")

    return {
        "Authorization": f"Bearer {api_key}",
        "Content-Type": "application/json",
    }


def _post_chat_completion(messages: list[dict], tools: list[dict]) -> dict:
    payload = {
        "model": os.getenv("LLM_MODEL", "deepseek-chat"),
        "messages": messages,
        "tools": tools,
        "tool_choice": "auto",
        "temperature": 0,
    }

    response = requests.post(
        _build_base_url() + "/chat/completions",
        headers=_build_headers(),
        json=payload,
        timeout=60,
    )
    response.raise_for_status()
    return response.json()


def _extract_tool_calls(message: dict) -> list[dict]:
    return message.get("tool_calls") or []


def _append_assistant_message(messages: list[dict], message: dict) -> None:
    assistant_message = {
        "role": "assistant",
        "content": message.get("content") or "",
    }
    if message.get("tool_calls"):
        assistant_message["tool_calls"] = message["tool_calls"]
    messages.append(assistant_message)


def _coerce_finish_arguments(arguments: dict) -> StudyPlanResult:
    return StudyPlanResult.model_validate(arguments)


def _parse_tool_arguments(tool_call: dict) -> dict:
    raw_arguments = tool_call["function"]["arguments"]
    if isinstance(raw_arguments, str):
        return json.loads(raw_arguments)
    if isinstance(raw_arguments, dict):
        return raw_arguments
    raise ValueError("tool arguments are neither string nor dict")


def _message_text(message: dict) -> str:
    content = message.get("content")
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts: list[str] = []
        for item in content:
            if isinstance(item, dict) and item.get("type") == "text":
                parts.append(item.get("text", ""))
        return "\n".join(parts).strip()
    return ""


def _fallback_parse_final_text(message: dict) -> StudyPlanResult:
    text = _message_text(message).strip()
    if not text:
        raise ValueError("model returned neither tool_calls nor final text")

    return StudyPlanResult(
        weak_tags=[],
        recommended_problems=[],
        study_plan_summary=text,
    )


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


def run_study_plan_with_deepseek(user_id: int, goal: str, token: str) -> StudyPlanResult:
    debug_enabled = _is_debug_enabled()
    tools = build_deepseek_tools()
    context = ToolExecutionContext()
    memory_context = _load_memory_context(user_id=user_id, goal=goal, debug_enabled=debug_enabled)

    if debug_enabled:
        print(f"[agent] start handwritten deepseek loop user_id={user_id} goal={goal!r}")

    messages: list[dict[str, Any]] = [
        {
            "role": "system",
            "content": (
                "You are an OJ study plan assistant. "
                "Your job is to decide which tools are needed to build a personalized study plan. "
                "Available tools include user history tools, a rule-based candidate retrieval tool, and a semantic vector retrieval tool. "
                "Use tools only when necessary, avoid repeated calls with different parameters unless the previous result is clearly insufficient, "
                "and when you have enough information, call finish_study_plan. "
                "If past study-plan memory is provided, use it as auxiliary context but still prioritize the latest tool results. "
                "Do not output the final answer in plain text. "
                "Only return the final study plan by calling finish_study_plan."
            ),
        },
        {
            "role": "user",
            "content": (
                f"Generate a study plan for user_id={user_id}. "
                f"User goal: {goal or 'No explicit goal provided.'}"
            ),
        },
    ]
    if memory_context:
        messages.append(
            {
                "role": "user",
                "content": memory_context,
            }
        )

    for round_index in range(8):
        if debug_enabled:
            print(f"[agent] round={round_index + 1} sending messages={len(messages)}")

        completion = _post_chat_completion(messages, tools)
        choice = completion["choices"][0]
        message = choice["message"]
        tool_calls = _extract_tool_calls(message)

        if not tool_calls:
            if debug_enabled:
                print("[agent] no tool_calls returned, fallback to final text")
            result = _fallback_parse_final_text(message)
            _save_memory(user_id=user_id, goal=goal, result=result, debug_enabled=debug_enabled)
            return result

        _append_assistant_message(messages, message)

        for tool_call in tool_calls:
            tool_name = tool_call["function"]["name"]
            arguments = _parse_tool_arguments(tool_call)

            if debug_enabled:
                print(f"[agent] tool_call name={tool_name} arguments={arguments}")

            if tool_name == "finish_study_plan":
                result = _coerce_finish_arguments(arguments)
                _save_memory(user_id=user_id, goal=goal, result=result, debug_enabled=debug_enabled)
                return result

            tool_result = execute_tool(tool_name, arguments, token, context)
            messages.append(
                {
                    "role": "tool",
                    "tool_call_id": tool_call["id"],
                    "content": json.dumps(tool_result, ensure_ascii=False),
                }
            )

    raise ValueError("study plan agent exceeded the maximum number of rounds")
