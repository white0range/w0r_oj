import os

import requests

GO_BACKEND_BASE_URL = os.getenv("GO_BACKEND_BASE_URL", "http://host.docker.internal:8080").rstrip("/")


def _build_headers(token: str) -> dict:
    return {
        "Authorization": f"Bearer {token}",
    }


def _get(path: str, token: str, params: dict | None = None) -> dict:
    url = f"{GO_BACKEND_BASE_URL}{path}"
    resp = requests.get(url, headers=_build_headers(token), params=params, timeout=10)
    resp.raise_for_status()
    return resp.json()


def _post(path: str, token: str, payload: dict | None = None) -> dict:
    url = f"{GO_BACKEND_BASE_URL}{path}"
    resp = requests.post(url, headers=_build_headers(token), json=payload or {}, timeout=10)
    resp.raise_for_status()
    return resp.json()


def get_user_ac_history(user_id: int, token: str) -> dict:
    return _get(f"/api/admin/agent/users/{user_id}/ac-history", token)


def get_user_failed_submissions(user_id: int, token: str, limit: int = 10) -> dict:
    return _get(
        f"/api/admin/agent/users/{user_id}/failed-submissions",
        token,
        params={"limit": limit},
    )


def get_user_tag_stats(user_id: int, token: str) -> dict:
    return _get(f"/api/admin/agent/users/{user_id}/tag-stats", token)


def get_candidate_problems(
    token: str,
    tags: list[str] | None = None,
    exclude_ids: list[int] | None = None,
    limit: int = 10,
) -> dict:
    params = {"limit": limit}

    if tags:
        params["tags"] = ",".join(tags)

    if exclude_ids:
        params["exclude_ids"] = ",".join(str(item) for item in exclude_ids)

    return _get("/api/admin/agent/problems/candidates", token, params=params)


def search_problems(token: str, keyword: str, tags: list[str] | None = None) -> dict:
    return _post(
        "/api/problems/search",
        token,
        payload={
            "keyword": keyword,
            "difficulty": 0,
            "tags": tags or [],
        },
    )
