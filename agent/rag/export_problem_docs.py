import argparse
import json
import os
from pathlib import Path
from typing import Any

import requests
from dotenv import load_dotenv

from rag.problem_doc_service import (
    build_problem_doc_record,
    fetch_problem_detail,
    problem_id_from_item,
)


load_dotenv(Path(__file__).resolve().parents[2] / ".env")


DEFAULT_BASE_URL = os.getenv("GO_BACKEND_BASE_URL", "http://host.docker.internal:8080").rstrip("/")
DEFAULT_OUTPUT = Path(__file__).with_name("problem_docs.json")


def _request_json(url: str, token: str = "") -> dict[str, Any]:
    headers: dict[str, str] = {}
    if token:
        headers["Authorization"] = f"Bearer {token}"

    response = requests.get(url, headers=headers, timeout=30)
    response.raise_for_status()
    return response.json()


def _body_data(payload: dict[str, Any]) -> Any:
    return payload.get("data")


def fetch_problem_ids(base_url: str, page_size: int, token: str = "") -> list[int]:
    page = 1
    total = None
    ids: list[int] = []

    while total is None or len(ids) < total:
        url = f"{base_url}/api/problems?page={page}&limit={page_size}"
        payload = _request_json(url, token=token)
        data = _body_data(payload) or {}
        items = data.get("items", [])

        if total is None:
            total = int(data.get("total", 0) or 0)
            if total == 0:
                break

        if not items:
            break

        ids.extend(problem_id_from_item(item) for item in items if isinstance(item, dict))
        print(f"[export] fetched page={page}, accumulated_ids={len(ids)}/{total}")
        page += 1

    return ids


def export_problem_docs(base_url: str, output_path: Path, page_size: int, token: str = "") -> None:
    problem_ids = fetch_problem_ids(base_url, page_size=page_size, token=token)
    docs: list[dict[str, Any]] = []

    for index, problem_id in enumerate(problem_ids, start=1):
        problem = fetch_problem_detail(base_url, problem_id, token=token)
        docs.append(build_problem_doc_record(problem))
        print(f"[export] built doc {index}/{len(problem_ids)} problem_id={problem_id}")

    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(docs, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"[export] wrote {len(docs)} problem docs to {output_path}")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Export OJ problems into document+metadata JSON for RAG.")
    parser.add_argument("--base-url", default=DEFAULT_BASE_URL, help="Go backend base URL")
    parser.add_argument("--output", default=str(DEFAULT_OUTPUT), help="Output JSON path")
    parser.add_argument("--page-size", type=int, default=100, help="Problem list page size")
    parser.add_argument(
        "--token",
        default=os.getenv("EXPORT_API_TOKEN", ""),
        help="Optional bearer token if the problem API is protected",
    )
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    export_problem_docs(
        base_url=str(args.base_url).rstrip("/"),
        output_path=Path(args.output),
        page_size=args.page_size,
        token=str(args.token),
    )


if __name__ == "__main__":
    main()
