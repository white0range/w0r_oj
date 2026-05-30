import os
from http import HTTPStatus
from pathlib import Path
from typing import Any

import dashscope
from dotenv import load_dotenv
from qdrant_client import QdrantClient
from qdrant_client.models import Distance, PointIdsList, PointStruct, VectorParams


load_dotenv(Path(__file__).resolve().parents[2] / ".env")


DEFAULT_COLLECTION = "problems"
DEFAULT_QDRANT_URL = os.getenv("QDRANT_URL", "http://localhost:6333")


def embedding_api_key() -> str:
    key = os.getenv("DASHSCOPE_API_KEY", "")
    if not key:
        raise ValueError("DASHSCOPE_API_KEY is not set")
    return key


def embedding_model() -> str:
    return os.getenv("EMBEDDING_MODEL", "text-embedding-v3")


def embedding_dimension() -> int:
    return int(os.getenv("EMBEDDING_DIMENSION", "1024"))


def embed_documents(texts: list[str]) -> list[list[float]]:
    dashscope.api_key = embedding_api_key()
    response = dashscope.TextEmbedding.call(
        model=embedding_model(),
        input=texts,
        text_type="document",
        dimension=embedding_dimension(),
    )
    if response.status_code != HTTPStatus.OK:
        raise ValueError(
            "dashscope embedding failed: "
            f"status_code={response.status_code} code={response.code} message={response.message}"
        )

    embeddings = response.output.get("embeddings", [])
    if len(embeddings) != len(texts):
        raise ValueError(
            f"embedding response count mismatch: expected {len(texts)}, got {len(embeddings)}"
        )
    return [item["embedding"] for item in embeddings]


def ensure_collection(client: QdrantClient, collection_name: str, vector_size: int) -> None:
    existing = {collection.name for collection in client.get_collections().collections}
    if collection_name in existing:
        return

    client.create_collection(
        collection_name=collection_name,
        vectors_config=VectorParams(size=vector_size, distance=Distance.COSINE),
    )


def qdrant_client(qdrant_url: str = DEFAULT_QDRANT_URL) -> QdrantClient:
    return QdrantClient(url=qdrant_url)


def upsert_problem_docs(
    docs: list[dict[str, Any]],
    qdrant_url: str = DEFAULT_QDRANT_URL,
    collection_name: str = DEFAULT_COLLECTION,
) -> int:
    if not docs:
        return 0

    client = qdrant_client(qdrant_url)
    vectors = embed_documents([str(item.get("document", "")) for item in docs])
    ensure_collection(client, collection_name, len(vectors[0]))

    points: list[PointStruct] = []
    for item, vector in zip(docs, vectors):
        problem_id = int(item["problem_id"])
        payload = {
            "problem_id": problem_id,
            "document": item.get("document", ""),
            **item.get("metadata", {}),
        }
        points.append(PointStruct(id=problem_id, vector=vector, payload=payload))

    client.upsert(collection_name=collection_name, points=points)
    return len(points)


def upsert_problem_doc(
    doc: dict[str, Any],
    qdrant_url: str = DEFAULT_QDRANT_URL,
    collection_name: str = DEFAULT_COLLECTION,
) -> int:
    return upsert_problem_docs([doc], qdrant_url=qdrant_url, collection_name=collection_name)


def delete_problem_doc(
    problem_id: int,
    qdrant_url: str = DEFAULT_QDRANT_URL,
    collection_name: str = DEFAULT_COLLECTION,
) -> None:
    client = qdrant_client(qdrant_url)
    client.delete(
        collection_name=collection_name,
        points_selector=PointIdsList(points=[problem_id]),
    )
