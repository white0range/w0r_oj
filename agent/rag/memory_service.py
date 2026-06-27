import json
import os
import uuid
from datetime import datetime, timezone
from pathlib import Path

from dotenv import load_dotenv
from qdrant_client.models import FieldCondition, Filter, MatchValue, PointStruct

from rag.index_service import ensure_collection, embed_documents, qdrant_client
from rag.search_service import DEFAULT_QDRANT_URL, embed_query
from schemas import StudyPlanResult


load_dotenv(Path(__file__).resolve().parents[2] / ".env")


DEFAULT_MEMORY_COLLECTION = os.getenv("MEMORY_COLLECTION", "study_plan_memories")
DEFAULT_MEMORY_TOP_K = int(os.getenv("MEMORY_TOP_K", "3"))


def _memory_text(user_id: int, goal: str, result: StudyPlanResult) -> str:
    # memory 存的不是原始对话，而是“学习规划结果摘要文本”。
    # 这样后续检索时，模型拿到的是高信息密度、和学习规划任务直接相关的历史上下文。
    recommended_titles = ", ".join(item.title for item in result.recommended_problems) or "none"
    weak_tags = ", ".join(result.weak_tags) or "none"
    return (
        f"User ID: {user_id}\n"
        f"Goal: {goal or 'No explicit goal provided.'}\n"
        f"Weak Tags: {weak_tags}\n"
        f"Recommended Problems: {recommended_titles}\n"
        f"Summary: {result.study_plan_summary}"
    )


def search_user_memories(
    user_id: int,
    query: str,
    qdrant_url: str = DEFAULT_QDRANT_URL,
    collection_name: str = DEFAULT_MEMORY_COLLECTION,
    limit: int = DEFAULT_MEMORY_TOP_K,
) -> list[dict]:
    # memory 检索流程：
    # 1. 当前 goal -> query embedding
    # 2. 在 study_plan_memories collection 里按 user_id 过滤
    # 3. 只取这个用户自己最相关的历史规划
    client = qdrant_client(qdrant_url)
    vector = embed_query(query)

    search_filter = Filter(
        must=[
            FieldCondition(
                key="user_id",
                match=MatchValue(value=user_id),
            )
        ]
    )

    if hasattr(client, "search"):
        # 同样兼容不同版本 qdrant-client 的接口差异。
        results = client.search(
            collection_name=collection_name,
            query_vector=vector,
            query_filter=search_filter,
            limit=limit,
            with_payload=True,
        )
    else:
        response = client.query_points(
            collection_name=collection_name,
            query=vector,
            query_filter=search_filter,
            limit=limit,
            with_payload=True,
        )
        results = list(getattr(response, "points", []))

    normalized = []
    for result in results:
        payload = result.payload or {}
        normalized.append(
            {
                "score": float(result.score),
                "goal": payload.get("goal", ""),
                "memory_text": payload.get("memory_text", ""),
                "created_at": payload.get("created_at", ""),
            }
        )
    return normalized


def save_study_plan_memory(
    user_id: int,
    goal: str,
    result: StudyPlanResult,
    qdrant_url: str = DEFAULT_QDRANT_URL,
    collection_name: str = DEFAULT_MEMORY_COLLECTION,
) -> str:
    # 保存 memory 的流程：
    # 1. 把本次规划结果压成文本
    # 2. embedding 成向量
    # 3. 确保 memory collection 存在
    # 4. 以 payload + vector 的形式写进 Qdrant
    client = qdrant_client(qdrant_url)
    memory_text = _memory_text(user_id, goal, result)
    vector = embed_documents([memory_text])[0]
    ensure_collection(client, collection_name, len(vector))

    point_id = uuid.uuid4().hex
    created_at = datetime.now(timezone.utc).isoformat()
    payload = {
        "user_id": user_id,
        "goal": goal,
        "memory_text": memory_text,
        "study_plan_result": json.loads(result.model_dump_json()),
        "created_at": created_at,
    }

    client.upsert(
        collection_name=collection_name,
        points=[PointStruct(id=point_id, vector=vector, payload=payload)],
    )
    return point_id
