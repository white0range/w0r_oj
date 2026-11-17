import os
from dataclasses import dataclass, field

import requests

GO_BACKEND_BASE_URL = os.getenv("GO_BACKEND_BASE_URL", "http://host.docker.internal:8080").rstrip("/")


class AgentExecutionExpired(RuntimeError):
    """Terminal execution failure, never a tool error for the model to repair."""


@dataclass(frozen=True)
class AgentToolAuth:
    # Execution context stays in Python closures, outside model tool arguments.
    turn_id: int
    processing_token: str = field(repr=False)


def _build_headers(auth: AgentToolAuth) -> dict:
    service_token = os.getenv("AGENT_SERVICE_TOKEN", "").strip()
    if not service_token:
        raise RuntimeError("AGENT_SERVICE_TOKEN must be configured")
    return {
        "X-Agent-Service-Token": service_token,
        "X-Agent-Turn-ID": str(auth.turn_id),
        "X-Agent-Processing-Token": auth.processing_token,
    }


def _get(path: str, auth: AgentToolAuth, params: dict | None = None) -> dict:
    url = f"{GO_BACKEND_BASE_URL}{path}"
    resp = requests.get(url, headers=_build_headers(auth), params=params, timeout=10)
    return _response_json(resp)


def _post(path: str, auth: AgentToolAuth, payload: dict | None = None) -> dict:
    url = f"{GO_BACKEND_BASE_URL}{path}"
    resp = requests.post(url, headers=_build_headers(auth), json=payload or {}, timeout=10)
    return _response_json(resp)


def _response_json(resp: requests.Response) -> dict:
    if resp.status_code == 403:
        try:
            if resp.json().get("code") == 40302:
                raise AgentExecutionExpired("agent execution is no longer current")
        except ValueError:
            pass
    resp.raise_for_status()
    return resp.json()


def require_active_execution(auth: AgentToolAuth) -> dict:
    return _get("/api/agent/context", auth)["data"]


def get_user_ac_history(auth: AgentToolAuth) -> dict:
    return _get("/api/agent/me/ac-history", auth)


def get_user_failed_submissions(auth: AgentToolAuth, limit: int = 10) -> dict:
    return _get(
        "/api/agent/me/failed-submissions",
        auth,
        params={"limit": limit},
    )


def get_failed_submission_detail(submission_id: int, auth: AgentToolAuth) -> dict:
    return _get(f"/api/agent/me/failed-submissions/{submission_id}", auth)


def get_user_tag_stats(auth: AgentToolAuth) -> dict:
    return _get("/api/agent/me/tag-stats", auth)


def get_candidate_problems(
    auth: AgentToolAuth,
    tags: list[str] | None = None,
    exclude_ids: list[int] | None = None,
    limit: int = 10,
) -> dict:
    params = {"limit": limit}

    if tags:
        params["tags"] = ",".join(tags)

    if exclude_ids:
        params["exclude_ids"] = ",".join(str(item) for item in exclude_ids)

    return _get("/api/agent/problems/candidates", auth, params=params)


def search_problems(auth: AgentToolAuth, keyword: str, tags: list[str] | None = None) -> dict:
    return _post(
        "/api/problems/search",
        auth,
        payload={
            "keyword": keyword,
            "difficulty": 0,
            "tags": tags or [],
        },
    )
