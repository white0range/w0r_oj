import json
import os
from dataclasses import dataclass, field

from client import (
    get_candidate_problems,
    get_user_ac_history,
    get_user_failed_submissions,
    get_user_tag_stats,
)
from rag.search_service import search_problem_docs


def _is_debug_enabled() -> bool:
    return os.getenv("AGENT_DEBUG", "").strip().lower() in {"1", "true", "yes", "on"}


@dataclass
class ToolExecutionContext:
    rule_candidate_calls: int = 0
    rule_candidate_request_key: str = ""
    rule_candidate_result: dict = field(default_factory=dict)
    semantic_candidate_calls: int = 0
    semantic_candidate_request_key: str = ""
    semantic_candidate_result: dict = field(default_factory=dict)


def _normalize_semantic_candidates(results: list[dict], exclude_ids: list[int]) -> dict:
    exclude_set = set(exclude_ids)
    items = []
    for item in results:
        problem_id = item.get("problem_id")
        if isinstance(problem_id, int) and problem_id in exclude_set:
            continue
        items.append(
            {
                "problem_id": problem_id,
                "title": item.get("title", ""),
                "description": item.get("document", ""),
                "tag_names": item.get("tags", []),
                "submit_count": item.get("submit_count", 0),
                "accepted_count": item.get("accepted_count", 0),
                "score": item.get("score", 0.0),
            }
        )

    return {
        "code": 0,
        "message": "ok",
        "data": {
            "requested_query": "",
            "items": items,
        },
    }


def execute_tool(name: str, arguments: dict, token: str, context: ToolExecutionContext) -> dict:
    debug_enabled = _is_debug_enabled()

    if debug_enabled:
        print(f"[tool-exec] {name} arguments={arguments}")

    if name == "user_ac_history":
        return get_user_ac_history(arguments["user_id"], token)

    if name == "user_failed_submissions":
        return get_user_failed_submissions(
            arguments["user_id"],
            token,
            limit=arguments.get("limit", 10),
        )

    if name == "user_tag_stats":
        return get_user_tag_stats(arguments["user_id"], token)

    if name == "candidate_problems":
        request_key = json.dumps(
            {
                "tags": arguments.get("tags", []),
                "exclude_ids": arguments.get("exclude_ids", []),
                "limit": arguments.get("limit", 10),
            },
            ensure_ascii=False,
            sort_keys=True,
        )

        if context.rule_candidate_calls == 0:
            result = get_candidate_problems(
                token,
                tags=arguments.get("tags", []),
                exclude_ids=arguments.get("exclude_ids", []),
                limit=arguments.get("limit", 10),
            )
            context.rule_candidate_calls = 1
            context.rule_candidate_request_key = request_key
            context.rule_candidate_result = result
            return result

        if request_key == context.rule_candidate_request_key:
            return context.rule_candidate_result

        return {
            "code": 0,
            "message": "candidate_problems has already been called once; use the existing candidate set or semantic retrieval to finish the study plan",
            "data": context.rule_candidate_result.get("data"),
        }

    if name == "semantic_candidate_problems":
        request_key = json.dumps(
            {
                "query": arguments.get("query", ""),
                "exclude_ids": arguments.get("exclude_ids", []),
                "limit": arguments.get("limit", 10),
            },
            ensure_ascii=False,
            sort_keys=True,
        )

        if context.semantic_candidate_calls == 0:
            result = _normalize_semantic_candidates(
                search_problem_docs(
                    query=arguments.get("query", ""),
                    limit=arguments.get("limit", 10),
                ),
                exclude_ids=arguments.get("exclude_ids", []),
            )
            result["data"]["requested_query"] = arguments.get("query", "")
            context.semantic_candidate_calls = 1
            context.semantic_candidate_request_key = request_key
            context.semantic_candidate_result = result
            return result

        if request_key == context.semantic_candidate_request_key:
            return context.semantic_candidate_result

        return {
            "code": 0,
            "message": "semantic_candidate_problems has already been called once; use the existing semantic candidate set to finish the study plan",
            "data": context.semantic_candidate_result.get("data"),
        }

    if name == "finish_study_plan":
        return arguments

    raise ValueError(f"unsupported tool: {name}")
