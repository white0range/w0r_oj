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
    # Tool 层和 Runtime 层共用同一个 debug 开关，便于串联完整日志。
    return os.getenv("AGENT_DEBUG", "").strip().lower() in {"1", "true", "yes", "on"}


@dataclass
class ToolExecutionContext:
    # 这个上下文对象是“轻量运行时状态”：
    # 它记录本次请求中 rule retrieval / semantic retrieval 是否已经调用过、
    # 调用时的参数是什么、结果是什么。
    #
    # 目的不是缓存全系统结果，而是限制模型在单次请求里反复乱查，
    # 避免一条 study plan 请求里工具调用无限膨胀。
    rule_candidate_calls: int = 0
    rule_candidate_request_key: str = ""
    rule_candidate_result: dict = field(default_factory=dict)
    semantic_candidate_calls: int = 0
    semantic_candidate_request_key: str = ""
    semantic_candidate_result: dict = field(default_factory=dict)


def _normalize_semantic_candidates(results: list[dict], exclude_ids: list[int]) -> dict:
    # Qdrant 检索层返回的是“面向向量检索”的原始结果，
    # 这里把它们转换成和 Go 候选题接口尽量相似的结构，方便模型统一消费。
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
    # execute_tool 是所有工具的统一分发入口。
    # LangChain / 手写 runtime 最终都可以复用这一层，
    # 这样“工具调用协议”和“工具真实执行逻辑”就解耦了。
    debug_enabled = _is_debug_enabled()

    if debug_enabled:
        print(f"[tool-exec] {name} arguments={arguments}")

    if name == "user_ac_history":
        # 这些用户画像类工具都直接委托给 Go internal API。
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
        # 规则候选题检索会根据 tags / exclude_ids / limit 构造一个请求 key。
        # 如果本次请求里是第一次调用，就真正访问 Go 候选题接口。
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
            # 参数完全一样，直接复用本次请求内的结果，避免重复 HTTP 调用。
            return context.rule_candidate_result

        # 如果模型想在同一轮请求里用不同参数再次查规则候选题，
        # 我们不再真的查第二次，而是返回一条软提示，引导模型基于现有结果继续完成规划。
        return {
            "code": 0,
            "message": "candidate_problems has already been called once; use the existing candidate set or semantic retrieval to finish the study plan",
            "data": context.rule_candidate_result.get("data"),
        }

    if name == "semantic_candidate_problems":
        # 语义候选题检索的保护策略和规则检索一样：
        # 单次请求里只真正查一次，后续同参复用，异参软限制。
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

    raise ValueError(f"unsupported tool: {name}")
