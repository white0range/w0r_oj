import os

import requests

GO_BACKEND_BASE_URL = os.getenv("GO_BACKEND_BASE_URL", "http://host.docker.internal:8080").rstrip("/")


def _build_headers(token: str) -> dict:
    # Python Agent 自己不维护用户体系，它通过 Go 的 Bearer Token 访问 internal tool API。
    return {
        "Authorization": f"Bearer {token}",
    }


def _get(path: str, token: str, params: dict | None = None) -> dict:
    # 所有对 Go 的读取请求统一走这个小函数：
    # - 统一拼接 base URL
    # - 统一带 token
    # - 统一做超时和状态码检查
    url = f"{GO_BACKEND_BASE_URL}{path}"
    resp = requests.get(url, headers=_build_headers(token), params=params, timeout=10)
    resp.raise_for_status()
    return resp.json()


def get_user_ac_history(user_id: int, token: str) -> dict:
    # 查询用户 AC 历史，帮助模型知道“这个用户已经做过什么题”。
    return _get(f"/api/admin/agent/users/{user_id}/ac-history", token)


def get_user_failed_submissions(user_id: int, token: str, limit: int = 10) -> dict:
    # 查询最近失败提交，帮助模型定位“当前薄弱点”。
    return _get(
        f"/api/admin/agent/users/{user_id}/failed-submissions",
        token,
        params={"limit": limit},
    )


def get_user_tag_stats(user_id: int, token: str) -> dict:
    # 按标签聚合统计，用于给模型提供更浓缩的用户画像。
    return _get(f"/api/admin/agent/users/{user_id}/tag-stats", token)


def get_candidate_problems(
    token: str,
    tags: list[str] | None = None,
    exclude_ids: list[int] | None = None,
    limit: int = 10,
) -> dict:
    # 这是规则候选题检索的 Go 侧入口：
    # 按标签筛题、排除已做题，并把结果作为一个较可控的候选集返回给模型。
    params = {"limit": limit}

    if tags:
        params["tags"] = ",".join(tags)

    if exclude_ids:
        params["exclude_ids"] = ",".join(str(item) for item in exclude_ids)

    return _get("/api/admin/agent/problems/candidates", token, params=params)
