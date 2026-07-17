import json
import os
import re
from dataclasses import dataclass, field
from typing import Any

from client import (
    get_candidate_problems,
    get_user_ac_history,
    get_user_failed_submissions,
    get_user_tag_stats,
    search_problems,
)
from rag.search_service import search_problem_docs


FILLER_PATTERNS = [
    r"^[\s,.;:!?，。？！、]*(帮我|给我|想要|请|麻烦)?",
    r"(推荐|一些|几道|题目|题|练习|刷题|oj|算法)",
]

STOPWORDS = {
    "the",
    "a",
    "an",
    "to",
    "for",
    "of",
    "and",
    "with",
    "problem",
    "problems",
    "practice",
}

COMMON_TAG_HINTS = {
    "dynamic programming": "dp",
    "shortest path": "graph shortest path",
    "minimum spanning tree": "mst graph",
    "union find": "disjoint set union dsu",
    "binary indexed tree": "fenwick tree",
    "segment tree": "segment tree",
    "topological sort": "topological sort dag",
}


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
    hybrid_candidate_calls: int = 0
    hybrid_candidate_request_key: str = ""
    hybrid_candidate_result: dict = field(default_factory=dict)


def _collapse_whitespace(text: str) -> str:
    return " ".join(str(text or "").strip().split())


def _normalize_query(query: str) -> str:
    normalized = _collapse_whitespace(query)
    for pattern in FILLER_PATTERNS:
        normalized = re.sub(pattern, " ", normalized, flags=re.I)
    normalized = re.sub(r"\s+", " ", normalized).strip(" ,.;:!?，。？！、")
    return normalized or _collapse_whitespace(query)


def _abstract_query(normalized: str) -> str:
    lowered = normalized.lower()
    for source, target in COMMON_TAG_HINTS.items():
        if source in lowered:
            return target

    tokens = re.findall(r"[A-Za-z0-9_+#-]+|[\u4e00-\u9fff]+", lowered)
    kept = [token for token in tokens if token not in STOPWORDS]
    return " ".join(kept[:8]).strip()


def build_query_variants(query: str) -> list[str]:
    raw = _collapse_whitespace(query)
    normalized = _normalize_query(raw)
    abstracted = _abstract_query(normalized)

    variants: list[str] = []
    for candidate in [raw, normalized, abstracted]:
        candidate = _collapse_whitespace(candidate)
        if candidate and candidate not in variants:
            variants.append(candidate)
    return variants or [raw or query or "general oj practice"]


def _safe_float(value: Any, default: float = 0.0) -> float:
    try:
        return float(value)
    except (TypeError, ValueError):
        return default


def _safe_int(value: Any, default: int = 0) -> int:
    try:
        return int(value)
    except (TypeError, ValueError):
        return default


def _tokenize_for_overlap(text: str) -> list[str]:
    lowered = _collapse_whitespace(text).lower()
    tokens = re.findall(r"[A-Za-z0-9_+#-]+|[\u4e00-\u9fff]+", lowered)
    return [token for token in tokens if token and token not in STOPWORDS]


def _normalize_semantic_candidates(results: list[dict], exclude_ids: list[int]) -> dict:
    exclude_set = set(exclude_ids)
    items = []
    for item in results:
        problem_id = _safe_int(item.get("problem_id"), 0)
        if not problem_id or problem_id in exclude_set:
            continue
        items.append(
            {
                "problem_id": problem_id,
                "title": item.get("title", ""),
                "description": item.get("document", ""),
                "tag_names": item.get("tags", []),
                "submit_count": _safe_int(item.get("submit_count"), 0),
                "accepted_count": _safe_int(item.get("accepted_count"), 0),
                "score": _safe_float(item.get("score"), 0.0),
                "source": "semantic",
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


def _normalize_lexical_candidates(payload: dict, exclude_ids: list[int]) -> dict:
    exclude_set = set(exclude_ids)
    body = payload.get("data") or payload
    raw_items = body.get("data", [])
    items = []

    for item in raw_items:
        problem_id = _safe_int(item.get("id") or item.get("ID") or item.get("problem_id"), 0)
        if not problem_id or problem_id in exclude_set:
            continue
        items.append(
            {
                "problem_id": problem_id,
                "title": item.get("title", ""),
                "description": item.get("description", ""),
                "tag_names": item.get("tags", []),
                "submit_count": _safe_int(item.get("submit_count"), 0),
                "accepted_count": _safe_int(item.get("accepted_count"), 0),
                "score": _safe_float(item.get("_score"), 0.0),
                "source": "lexical",
            }
        )

    return {
        "code": 0,
        "message": "ok",
        "data": {
            "items": items,
        },
    }


def _semantic_search_best_effort(query: str, exclude_ids: list[int], limit: int) -> tuple[list[dict[str, Any]], str]:
    try:
        payload = _normalize_semantic_candidates(
            search_problem_docs(query=query, limit=limit),
            exclude_ids=exclude_ids,
        )
        return payload["data"]["items"], ""
    except Exception as exc:
        warning = f"semantic retrieval unavailable: {exc}"
        if _is_debug_enabled():
            print(f"[tool-exec] {warning}")
        return [], warning


def _merge_hybrid_candidates(lexical_items: list[dict], semantic_items: list[dict], limit: int) -> list[dict]:
    merged: dict[int, dict[str, Any]] = {}

    for rank, item in enumerate(lexical_items, start=1):
        problem_id = _safe_int(item.get("problem_id"), 0)
        if not problem_id:
            continue
        entry = merged.setdefault(problem_id, dict(item))
        entry["hybrid_score"] = entry.get("hybrid_score", 0.0) + 2.0 / rank + min(_safe_float(item.get("score")), 5.0) * 0.05
        sources = set(entry.get("sources", []))
        sources.add("lexical")
        entry["sources"] = sorted(sources)

    for rank, item in enumerate(semantic_items, start=1):
        problem_id = _safe_int(item.get("problem_id"), 0)
        if not problem_id:
            continue
        entry = merged.setdefault(problem_id, dict(item))
        entry["hybrid_score"] = entry.get("hybrid_score", 0.0) + 1.5 / rank + _safe_float(item.get("score"))
        if not entry.get("title"):
            entry["title"] = item.get("title", "")
        if not entry.get("description"):
            entry["description"] = item.get("description", "")
        if not entry.get("tag_names"):
            entry["tag_names"] = item.get("tag_names", [])
        sources = set(entry.get("sources", []))
        sources.add("semantic")
        entry["sources"] = sorted(sources)

    items = sorted(
        merged.values(),
        key=lambda item: (
            -_safe_float(item.get("hybrid_score")),
            -len(item.get("sources", [])),
            item.get("problem_id", 0),
        ),
    )
    return items[:limit]


def _rerank_hybrid_candidates(query: str, items: list[dict], limit: int) -> list[dict]:
    query_terms = set(_tokenize_for_overlap(query))
    lowered_query = _collapse_whitespace(query).lower()

    ranked = []
    for item in items:
        title = str(item.get("title", ""))
        tags = item.get("tag_names", []) or []
        title_lower = title.lower()
        tag_lower = " ".join(str(tag) for tag in tags).lower()

        title_overlap = sum(1 for token in query_terms if token in title_lower)
        tag_overlap = sum(1 for token in query_terms if token in tag_lower)
        exact_bonus = 0.8 if lowered_query and lowered_query in title_lower else 0.0
        source_bonus = 0.35 * len(item.get("sources", []))
        quality_bonus = 0.0
        submit_count = _safe_int(item.get("submit_count"), 0)
        accepted_count = _safe_int(item.get("accepted_count"), 0)
        if submit_count > 0:
            quality_bonus = min(accepted_count / submit_count, 1.0) * 0.15

        rerank_score = (
            _safe_float(item.get("hybrid_score"))
            + title_overlap * 0.6
            + tag_overlap * 0.3
            + exact_bonus
            + source_bonus
            + quality_bonus
        )

        next_item = dict(item)
        next_item["rerank_score"] = rerank_score
        ranked.append(next_item)

    ranked.sort(
        key=lambda item: (
            -_safe_float(item.get("rerank_score")),
            -_safe_float(item.get("hybrid_score")),
            item.get("problem_id", 0),
        )
    )
    return ranked[:limit]


def execute_tool(name: str, arguments: dict, token: str, context: ToolExecutionContext, bound_user_id: int) -> dict:
    debug_enabled = _is_debug_enabled()

    if debug_enabled:
        print(f"[tool-exec] {name} arguments={arguments}")

    if name == "user_ac_history":
        return get_user_ac_history(bound_user_id, token)

    if name == "user_failed_submissions":
        return get_user_failed_submissions(
            bound_user_id,
            token,
            limit=arguments.get("limit", 10),
        )

    if name == "user_tag_stats":
        return get_user_tag_stats(bound_user_id, token)

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
            "message": "candidate_problems has already been called once; use the existing candidate set or hybrid retrieval to finish the reply",
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
            semantic_items, warning = _semantic_search_best_effort(
                query=arguments.get("query", ""),
                exclude_ids=arguments.get("exclude_ids", []),
                limit=arguments.get("limit", 10),
            )
            result = {
                "code": 0,
                "message": warning or "ok",
                "data": {
                    "requested_query": arguments.get("query", ""),
                    "items": semantic_items,
                },
            }
            context.semantic_candidate_calls = 1
            context.semantic_candidate_request_key = request_key
            context.semantic_candidate_result = result
            return result

        if request_key == context.semantic_candidate_request_key:
            return context.semantic_candidate_result

        return {
            "code": 0,
            "message": "semantic_candidate_problems has already been called once; use the existing semantic candidate set to finish the reply",
            "data": context.semantic_candidate_result.get("data"),
        }

    if name == "hybrid_candidate_problems":
        request_key = json.dumps(
            {
                "query": arguments.get("query", ""),
                "exclude_ids": arguments.get("exclude_ids", []),
                "limit": arguments.get("limit", 10),
            },
            ensure_ascii=False,
            sort_keys=True,
        )

        if context.hybrid_candidate_calls == 0:
            raw_query = arguments.get("query", "")
            exclude_ids = arguments.get("exclude_ids", [])
            limit = arguments.get("limit", 10)
            variants = build_query_variants(raw_query)

            lexical_items: list[dict[str, Any]] = []
            semantic_items: list[dict[str, Any]] = []
            warnings: list[str] = []
            for query in variants[:2]:
                lexical_payload = search_problems(token, keyword=query)
                lexical_items.extend(_normalize_lexical_candidates(lexical_payload, exclude_ids)["data"]["items"])

            for query in variants:
                next_items, warning = _semantic_search_best_effort(
                    query=query,
                    exclude_ids=exclude_ids,
                    limit=limit,
                )
                semantic_items.extend(next_items)
                if warning:
                    warnings.append(warning)

            merged_items = _merge_hybrid_candidates(lexical_items, semantic_items, limit * 2)
            reranked_items = _rerank_hybrid_candidates(raw_query, merged_items, limit)
            if not reranked_items and warnings:
                raise RuntimeError(warnings[0])

            message = "ok"
            if warnings:
                message = f"partial fallback to lexical retrieval: {warnings[0]}"

            result = {
                "code": 0,
                "message": message,
                "data": {
                    "requested_query": raw_query,
                    "normalized_queries": variants,
                    "items": reranked_items,
                },
            }
            context.hybrid_candidate_calls = 1
            context.hybrid_candidate_request_key = request_key
            context.hybrid_candidate_result = result
            return result

        if request_key == context.hybrid_candidate_request_key:
            return context.hybrid_candidate_result

        return {
            "code": 0,
            "message": "hybrid_candidate_problems has already been called once; use the existing merged candidate set to finish the reply",
            "data": context.hybrid_candidate_result.get("data"),
        }

    raise ValueError(f"unsupported tool: {name}")

