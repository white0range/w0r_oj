# 本地开发

首次启动见[项目首页](../README.md#本地启动)。开发配置由 Compose 启动前端和基础依赖，应用进程可在宿主机直接调试。

## 配置来源

| 进程 | 配置来源 | 关键字段 |
| --- | --- | --- |
| Docker Compose | `.env`，以 [开发模板](../.env.example)为起点 | MySQL 用户与密码、Redis 密码 |
| Go API 与 Judge Worker | `APP_ENV` 选择 `config/config.<环境>.yaml`，默认使用 `config/config.dev.yaml`。进程环境可通过已绑定的 `GOJO_*` 变量覆盖 YAML | `sql.dsn`、`redis.password`、`jwt.secret`、`chat.agent_service_token`、`judge.worker_count` |
| Python Agent | 通过 Uvicorn 的 `--env-file .env` 加载环境变量 | `AGENT_SERVICE_TOKEN`、`GO_BACKEND_BASE_URL`、`DEEPSEEK_API_KEY`、`LLM_MODEL`、`DASHSCOPE_API_KEY`、`QDRANT_URL` |

Go 在宿主机运行时，从 YAML 和 Go 进程环境获取配置。`.env` 中的 MySQL 账号和密码须与 Go DSN 对应，`REDIS_PASSWORD` 对应 `redis.password`，`AGENT_SERVICE_TOKEN` 对应 `chat.agent_service_token`。MySQL 数据卷已有数据时，账号仍沿用初始化时的值。

Go API 启动时通过 GORM 初始化业务表。Judge Worker 使用同一套数据库配置读取题目和写入判题结果。若需测试 Agent 试跑，在 **Go API 和 Judge Worker** 的启动环境里都设置 `GOJO_JUDGE_TRIAL_ENABLED=true`，或在两者共用的开发 YAML 中设置 `judge.trial_enabled: true`。

## 开发端口

| 服务 | 默认入口 |
| --- | --- |
| Vue | `http://localhost:3000` |
| Go API | `http://127.0.0.1:8080/ping` |
| Python Agent | `http://127.0.0.1:8000/ping` |
| MySQL | `127.0.0.1:3306` |
| Redis | `127.0.0.1:6379` |
| Elasticsearch | `http://127.0.0.1:9200` |
| Qdrant | `http://127.0.0.1:6333/collections` |
| Kibana | `http://127.0.0.1:5601`，通过 `docker compose --profile debug up -d kibana` 启动 |

### 在宿主机运行 Vue

使用 Node.js 22。适用于需要直接在宿主机调试前端，或 Docker Engine 无法解析 `host.docker.internal` 的场景。

```bash
docker compose stop frontend
docker compose up -d --build mysql redis elasticsearch qdrant
cd vue
npm ci
GOJO_API_PROXY=http://127.0.0.1:8080 npm run dev -- --strictPort
```

## 管理员和示例题

用初始化工具创建管理员。用户名为 3–32 个字符，密码为 12–72 字节。在 Bash 或 Zsh 中运行 `read` 后输入密码并回车。

```bash
read -rs BOOTSTRAP_ADMIN_PASSWORD
export BOOTSTRAP_ADMIN_PASSWORD
APP_ENV=dev BOOTSTRAP_ADMIN_USERNAME=admin go run ./cmd/bootstrap_admin
unset BOOTSTRAP_ADMIN_PASSWORD
```

导入示例题时使用 `APP_ENV=dev go run ./cmd/seed_problems`。工具按标题跳过已有题目。

## 测试与构建

在仓库根目录执行。Python 测试使用本地虚拟环境。

```bash
go test ./...
go build ./...
.venv/bin/python -m unittest discover -s agent
(cd vue && npm ci && npm run build)
```

MySQL、Redis 和 Qdrant 的集成测试通过测试专用环境变量启用，默认跳过。Redis 测试需要可丢弃的实例。测试条件见 [Go 同步队列集成测试](../internal/syncer/chat_memory_integration_test.go)和 [Python 记忆集成测试](../agent/test_execution_fencing.py)。

开发进程在终端输出日志。判题和 Agent 的故障定位见[日志与排障](deployment.md#日志与排障)。
