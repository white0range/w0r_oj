import os
from pathlib import Path
from typing import Any

import requests
from dotenv import load_dotenv


load_dotenv(Path(__file__).resolve().parents[2] / ".env")


DEFAULT_BASE_URL = os.getenv("GO_BACKEND_BASE_URL", "http://host.docker.internal:8080").rstrip("/")


def _request_json(url: str, token: str = "") -> dict[str, Any]:
    headers: dict[str, str] = {}
    if token:
        headers["Authorization"] = f"Bearer {token}"

    response = requests.get(url, headers=headers, timeout=30)
    response.raise_for_status()
    return response.json()


def _body_data(payload: dict[str, Any]) -> Any:
    return payload.get("data")


def problem_id_from_item(item: dict[str, Any]) -> int:
    raw_id = item.get("ID", item.get("id"))
    if raw_id is None:
        raise ValueError(f"problem item missing id: {item}")
    return int(raw_id)


def tag_names(problem: dict[str, Any]) -> list[str]:
    tags = problem.get("tags", [])
    names: list[str] = []
    for tag in tags:
        if not isinstance(tag, dict):
            continue
        name = tag.get("name", tag.get("Name"))
        if isinstance(name, str) and name.strip():
            names.append(name.strip())
    return names


def build_problem_document(problem: dict[str, Any]) -> str:
    title = str(problem.get("title", "")).strip()
    description = str(problem.get("description", "")).strip()
    tags = tag_names(problem)
    time_limit = problem.get("time_limit", 0)
    memory_limit = problem.get("memory_limit", 0)
    submit_count = problem.get("submit_count", 0)
    accepted_count = problem.get("accepted_count", 0)

    parts = [
        f"Title: {title}",
        f"Tags: {', '.join(tags) if tags else 'none'}",
        f"Description: {description or 'none'}",
        f"Time Limit: {time_limit} ms",
        f"Memory Limit: {memory_limit} MB",
        f"Submit Count: {submit_count}",
        f"Accepted Count: {accepted_count}",
    ]
    return "\n".join(parts)


def build_problem_doc_record(problem: dict[str, Any]) -> dict[str, Any]:
    problem_id = problem_id_from_item(problem)
    title = str(problem.get("title", "")).strip()
    tags = tag_names(problem)

    return {
        "problem_id": problem_id,
        "document": build_problem_document(problem),
        "metadata": {
            "title": title,
            "tags": tags,
            "submit_count": int(problem.get("submit_count", 0) or 0),
            "accepted_count": int(problem.get("accepted_count", 0) or 0),
            "time_limit": int(problem.get("time_limit", 0) or 0),
            "memory_limit": int(problem.get("memory_limit", 0) or 0),
        },
    }


def fetch_problem_detail(base_url: str, problem_id: int, token: str = "") -> dict[str, Any]:
    url = f"{base_url.rstrip('/')}/api/problems/{problem_id}"
    payload = _request_json(url, token=token)
    data = _body_data(payload)
    if not isinstance(data, dict):
        raise ValueError(f"unexpected problem detail payload for problem_id={problem_id}: {payload}")
    return data


def fetch_problem_doc_record(problem_id: int, base_url: str = DEFAULT_BASE_URL, token: str = "") -> dict[str, Any]:
    problem = fetch_problem_detail(base_url=base_url, problem_id=problem_id, token=token)
    return build_problem_doc_record(problem)
