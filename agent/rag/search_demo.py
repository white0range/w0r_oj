import argparse
from typing import Any

from rag.search_service import DEFAULT_COLLECTION, DEFAULT_QDRANT_URL, search_problem_docs


def _preview(text: str, limit: int = 140) -> str:
    normalized = " ".join(text.split())
    if len(normalized) <= limit:
        return normalized
    return normalized[:limit].rstrip() + "..."


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Search indexed problem docs from Qdrant.")
    parser.add_argument("query", help="Natural language query for semantic retrieval")
    parser.add_argument("--qdrant-url", default=DEFAULT_QDRANT_URL, help="Qdrant base URL")
    parser.add_argument("--collection", default=DEFAULT_COLLECTION, help="Qdrant collection name")
    parser.add_argument("--limit", type=int, default=5, help="Top-k results")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    results = search_problem_docs(
        query=args.query,
        qdrant_url=args.qdrant_url,
        collection_name=args.collection,
        limit=args.limit,
    )

    print(f"[search] query={args.query!r} results={len(results)}")
    for index, result in enumerate(results, start=1):
        title = result.get("title", "")
        tags = result.get("tags", [])
        document = str(result.get("document", ""))
        print(f"\n[{index}] score={float(result.get('score', 0.0)):.6f}")
        print(f"problem_id: {result.get('problem_id')}")
        print(f"title: {title}")
        print(f"tags: {', '.join(tags) if isinstance(tags, list) else tags}")
        print(f"preview: {_preview(document)}")


if __name__ == "__main__":
    main()
