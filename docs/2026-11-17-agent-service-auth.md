# Agent 专用服务鉴权

日期：2026-11-17

## 调用方式

- 浏览器调用 Go 继续使用用户 JWT。
- Go 调用 Python 使用 `X-Agent-Service-Token`，不再生成管理员 JWT。
- Python 调用 Go 工具接口使用相同的服务密钥，并携带 `X-Agent-Turn-ID` 和 `X-Agent-Processing-Token`。
- Go 配置 `chat.agent_service_token` 与 Python 环境变量 `AGENT_SERVICE_TOKEN` 必须一致。修改后两个服务需同步重启。

## 工具接口

旧的 `/api/admin/agent/*` 路由已移除。新路由由 `AgentAuth`、`AgentTurnContext` 两个中间件保护：

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `/api/agent/context` | 校验当前执行并返回回合与用户编号 |
| GET | `/api/agent/me/ac-history` | 当前回合用户的通过记录 |
| GET | `/api/agent/me/failed-submissions` | 当前回合用户的失败提交 |
| GET | `/api/agent/me/failed-submissions/:submission_id` | 失败代码和题目上下文 |
| GET | `/api/agent/me/tag-stats` | 当前回合用户的标签统计 |
| GET | `/api/agent/problems/candidates` | 候选题目 |
| GET | `/api/agent/problems/:id` | 题目详情 |

Go 校验回合正在运行、租约未过期、执行编号等于当前 ProcessingToken、会话有效且归属一致，并从数据库确定用户；工具接口不接受用户 ID 路径参数。失败提交详情另外校验提交归属。用户 JWT 不能替代服务密钥，服务密钥不能用于用户或管理员接口。

Python 的 `/chat/run` 请求需要正整数 `turn_id`、`user_id` 和 64 字符 `processing_token`，由 Go Worker 提供。回合编号由 Python 绑定在工具闭包中，不属于模型工具参数。摘要、题库同步继续使用服务密钥，公共题目搜索继续使用原公开接口。

## 权限边界与部署

Nginx 网关已拦截精确路径 `/api/agent` 以及前缀 `/api/agent/`，直接返回 404。Python 继续通过 `http://api:8080` 直连内部 Go 服务，工具请求仍执行服务密钥和回合校验。Go API 不应单独映射到公网端口。网站普通 API 继续使用原有转发规则。

`nginx/default.conf` 在构建 gateway 镜像时复制，发布配置需要更新 gateway；可单独执行下列命令，不启动或重建依赖服务：

```bash
docker compose --env-file .env.local-production \
  -f docker-compose.prod.yml -f docker-compose.4g.yml \
  up -d --build --no-deps gateway
```

这是固定 API Key 方案，没有增加每轮 JWT。服务密钥标识受信任的 Agent 服务；回合编号是业务上下文，不是凭据。执行编号复用已有的 ProcessingToken，用于拒绝过期执行；它不能替代固定服务密钥。密钥只保存在服务端，生产使用受控内部网络，跨公网调用使用 HTTPS。

Go 与 Python 的接口契约同时改变，发布时应同步更新两个服务。Chat Worker 初始化不再依赖管理员账号。记忆同步迭代在 `chat_turns` 增加 `memory_sync_pending` 字段，由 Go 启动时的 AutoMigrate 创建，没有新增表；详见同目录的执行隔离文档。

## 验证

- `go test ./...`：覆盖中间件、实际路由注册、Worker 请求及原有业务测试。
- `PYTHONPATH=agent .venv/bin/python -m unittest discover -s agent -p 'test_*.py' -v`：覆盖 Python 入站鉴权、摘要鉴权、必填回合上下文、工具请求和模型参数隔离；模型调用使用替身。

- 本地实际联调：前端登录、创建回合、真实模型调用新工具接口并完成 A+B 示例错题分析；回合结束后工具请求返回 403。服务密钥不能访问用户或管理接口，旧路由返回 404。
