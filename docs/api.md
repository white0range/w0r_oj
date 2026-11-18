# API 参考

Go API 提供 HTTP JSON 接口和 SSE。登录响应在 JSON 中返回 Access Token，并将 Refresh Token 写入名为 `gojo_refresh_token` 的 HttpOnly Cookie。受保护接口使用 `Authorization: Bearer <access_token>`，管理员接口还校验用户角色。

## 业务接口

| 方法与路径 | 权限 | 用途 |
| --- | --- | --- |
| `GET /ping` | 公开 | Go API 存活检查 |
| `POST /api/register` | 公开 | 注册账号 |
| `POST /api/login` | 公开 | 登录，获取 Access Token 和 Refresh Token |
| `POST /api/refresh` | Refresh Cookie | 轮换 Refresh Token 并返回新的 Access Token |
| `POST /api/logout` | 可选 Refresh Cookie | 注销登录会话并清除 Cookie |
| `GET /api/profile` | 已登录 | 读取个人资料 |
| `GET /api/problems` | 公开 | 分页列出题目，支持按标签筛选 |
| `GET /api/problems/:id` | 公开 | 读取题目详情 |
| `GET /api/tags` | 公开 | 读取题目标签 |
| `POST /api/problems/search` | 公开 | 关键词与标签搜索 |
| `POST /api/submit` | 已登录 | 提交代码并创建判题任务 |
| `GET /api/submissions/:id` | 已登录 | 查询自己的单条提交 |
| `GET /api/my-submissions` | 已登录 | 查询个人提交历史 |
| `GET /api/leaderboard` | 公开，可选登录 | 读取排行榜与个人排名 |
| `/api/admin/*` | 管理员 | 管理用户、题目、标签和测试用例 |

## 对话与实时通知

| 方法与路径 | 权限 | 用途 |
| --- | --- | --- |
| `GET /api/chat/sessions` | 已登录 | 会话列表 |
| `POST /api/chat/sessions` | 已登录 | 创建会话 |
| `GET /api/chat/sessions/:session_id` | 已登录 | 会话详情 |
| `DELETE /api/chat/sessions/:session_id` | 已登录 | 归档会话 |
| `GET /api/chat/sessions/:session_id/messages` | 已登录 | 查询会话消息 |
| `POST /api/chat/sessions/:session_id/messages` | 已登录 | 发送消息并创建异步回合 |
| `GET /api/chat/turns/:turn_id` | 已登录 | 查询回合状态和结果 |
| `GET /api/chat/turns/:turn_id/feedback` | 已登录 | 查询回合反馈 |
| `POST /api/chat/turns/:turn_id/feedback` | 已登录 | 提交回合反馈 |
| `POST /api/events/ticket` | 已登录 | 申请判题通知的 SSE Ticket |
| `GET /api/events?ticket=…` | SSE Ticket | 订阅判题通知 |
| `POST /api/chat/turns/:turn_id/stream-ticket` | 已登录 | 申请绑定当前回合的 SSE Ticket |
| `GET /api/chat/turns/:turn_id/stream?ticket=…` | SSE Ticket | 订阅回合状态 |

SSE Ticket 有效期为 60 秒，使用一次后即失效。重新订阅需要申请新 Ticket。提交和回合查询接口可用于连接中断后的状态确认。

## Agent 内部接口

生产网关对 `/api/agent` 和 `/api/agent/*` 返回 404。Python 通过 Compose 内网地址 `http://api:8080` 调用 Go，并携带以下请求头。

| 请求头 | 用途 |
| --- | --- |
| `X-Agent-Service-Token` | 服务密钥，与 `chat.agent_service_token` 一致 |
| `X-Agent-Turn-ID` | 当前对话回合 |
| `X-Agent-Processing-Token` | 回合本次领取的执行令牌 |

| 方法与路径 | 用途 |
| --- | --- |
| `GET /api/agent/context` | 校验当前回合并返回归属 |
| `GET /api/agent/me/ac-history` | 读取当前用户的通过记录 |
| `GET /api/agent/me/failed-submissions` | 读取当前用户的近期失败提交 |
| `GET /api/agent/me/failed-submissions/:submission_id` | 读取当前用户的提交源码、判题信息与题面 |
| `GET /api/agent/me/tag-stats` | 读取当前用户的标签统计 |
| `GET /api/agent/problems/candidates` | 获取候选题目 |
| `GET /api/agent/problems/:id` | 获取题目上下文 |
| `POST /api/agent/trials` | 对当前用户的一条失败 Go 提交运行一组自定义输入 |

试跑请求体包含 `submission_id` 和字符串 `input`。空字符串表示无输入。成功响应的 `data` 包括 `status`、`output` 和 `duration_ms`，输出截断时附带 `truncated`。`status=ok` 表示自定义输入运行完成。队列满返回 HTTP 429，同回合重复提交返回 409，等待超时返回 504。回合执行失效时返回 HTTP 403 和业务码 `40302`，Python 中止该回合的当前执行。

Go 调用 Python 的 `POST /chat/run`、`POST /chat/summarize-session`、`POST /memory/sync` 以及 `POST /rag/problems/sync`、`POST /rag/problems/delete` 时使用 `X-Agent-Service-Token`。`/chat/run` 的请求体额外携带 `turn_id`、`user_id` 和 `processing_token` 用于对话执行。Python 的 `GET /ping` 用于内部存活检查。

接口注册见 [Go 路由](../internal/app/routes.go)和 [Python 路由](../agent/app.py)。
