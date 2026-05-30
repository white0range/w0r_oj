import argparse
import json
import os
from pathlib import Path
from typing import Any

from dotenv import load_dotenv

from rag.index_service import upsert_problem_docs


load_dotenv(Path(__file__).resolve().parents[2] / ".env")


DEFAULT_DOCS_PATH = Path(__file__).with_name("problem_docs.json")
DEFAULT_COLLECTION = "problems"
DEFAULT_QDRANT_URL = os.getenv("QDRANT_URL", "http://localhost:6333")


def _load_docs(path: Path) -> list[dict[str, Any]]:
    if not path.exists():
        raise FileNotFoundError(f"problem docs file not found: {path}")
    data = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(data, list):
        raise ValueError("problem docs json must be a list")
    return data


def _batched(items: list[dict[str, Any]], batch_size: int) -> list[list[dict[str, Any]]]:
    return [items[i : i + batch_size] for i in range(0, len(items), batch_size)]


def index_problem_docs(
    docs_path: Path,
    qdrant_url: str,
    collection_name: str,
    batch_size: int,
) -> None:
    docs = _load_docs(docs_path)
    if not docs:
        print("[index] no problem docs found, nothing to index")
        return

    for batch_index, batch in enumerate(_batched(docs, batch_size), start=1):
        count = upsert_problem_docs(
            batch,
            qdrant_url=qdrant_url,
            collection_name=collection_name,
        )
        if batch_index == 1 and count > 0:
            print(
                f"[index] collection={collection_name} vector_size={os.getenv('EMBEDDING_DIMENSION', '1024')}"
            )
        print(f"[index] upserted batch={batch_index} size={count}")

    print(f"[index] completed indexing {len(docs)} problem docs into {collection_name}")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Index exported problem docs into Qdrant.")
    parser.add_argument("--docs", default=str(DEFAULT_DOCS_PATH), help="Path to problem_docs.json")
    parser.add_argument("--qdrant-url", default=DEFAULT_QDRANT_URL, help="Qdrant base URL")
    parser.add_argument("--collection", default=DEFAULT_COLLECTION, help="Qdrant collection name")
    parser.add_argument("--batch-size", type=int, default=32, help="Embedding batch size")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    index_problem_docs(
        docs_path=Path(args.docs),
        qdrant_url=args.qdrant_url,
        collection_name=args.collection,
        batch_size=args.batch_size,
    )


if __name__ == "__main__":
    main()
