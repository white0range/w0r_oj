import os
from http import HTTPStatus
from pathlib import Path
from typing import Any

import dashscope
from dotenv import load_dotenv
from qdrant_client import QdrantClient


load_dotenv(Path(__file__).resolve().parents[2] / ".env")


# 默认检索 collection 是题目索引 problems。
DEFAULT_COLLECTION = "problems"
DEFAULT_QDRANT_URL = os.getenv("QDRANT_URL", "http://localhost:6333")


def _embedding_api_key() -> str:
    # 检索和写入都依赖 DashScope embedding 能力，缺 key 时直接失败，避免悄悄退化。
    key = os.getenv("DASHSCOPE_API_KEY", "")
    if not key:
        raise ValueError("DASHSCOPE_API_KEY is not set")
    return key


def _embedding_model() -> str:
    return os.getenv("EMBEDDING_MODEL", "text-embedding-v3")


def _embedding_dimension() -> int:
    return int(os.getenv("EMBEDDING_DIMENSION", "1024"))


def embed_query(text: str) -> list[float]:
    # 把自然语言 query 映射到向量空间。
    # 对学习规划来说，这一步的输入通常是“用户目标”的改写版本。
    dashscope.api_key = _embedding_api_key()
    response = dashscope.TextEmbedding.call(
        model=_embedding_model(),
        input=text,
        text_type="query",
        dimension=_embedding_dimension(),
    )
    if response.status_code != HTTPStatus.OK:
        raise ValueError(
            "dashscope query embedding failed: "
            f"status_code={response.status_code} code={response.code} message={response.message}"
        )

    embeddings = response.output.get("embeddings", [])
    if not embeddings:
        raise ValueError("dashscope query embedding returned no embeddings")
    return embeddings[0]["embedding"]


def search_problem_docs(
    query: str,
    qdrant_url: str = DEFAULT_QDRANT_URL,
    collection_name: str = DEFAULT_COLLECTION,
    limit: int = 5,
) -> list[dict[str, Any]]:
    # 语义检索主流程：
    # 1. 把 query embedding 成向量
    # 2. 去 Qdrant 查最相似的题目
    # 3. 统一整理成上层工具可消费的结构
    client = QdrantClient(url=qdrant_url)
    vector = embed_query(query)

    if hasattr(client, "search"):
        # 兼容不同版本 qdrant-client 的查询方法差异。
        results = client.search(
            collection_name=collection_name,
            query_vector=vector,
            limit=limit,
            with_payload=True,
        )
    else:
        response = client.query_points(
            collection_name=collection_name,
            query=vector,
            limit=limit,
            with_payload=True,
        )
        results = list(getattr(response, "points", []))

    normalized: list[dict[str, Any]] = []
    for result in results:
        # payload 是建索引时一并存进去的 metadata；
        # score 是向量相似度，越高通常越相关。
        payload = result.payload or {}
        normalized.append(
            {
                "score": float(result.score),
                "problem_id": payload.get("problem_id"),
                "title": payload.get("title", ""),
                "tags": payload.get("tags", []),
                "document": payload.get("document", ""),
                "submit_count": payload.get("submit_count", 0),
                "accepted_count": payload.get("accepted_count", 0),
                "time_limit": payload.get("time_limit", 0),
                "memory_limit": payload.get("memory_limit", 0),
            }
        )
    return normalized
