# Gojo OJ

Gojo OJ 是一个面向算法练习的在线判题系统，提供题库、代码提交、判题结果和学习助手。学习助手可结合用户的提交记录分析错误，或根据练习情况推荐题目。

- **在线判题**　支持 Go 程序。提交进入异步队列，由 Judge Worker 在受限的 Docker 容器中编译和执行。
- **题库与管理**　支持关键词搜索、标签筛选、个人提交记录和排行榜。管理后台提供用户、题目、标签和测试用例管理。
- **学习助手**　支持多轮问答、错题分析和题目推荐。错题分析可对原提交试跑一组自定义输入，辅助验证原因。

## 本地启动

需要 **Go 1.26、Python 3.11、Docker 和 Docker Compose v2**。以下命令在仓库根目录执行。前端、MySQL、Redis、Elasticsearch 和 Qdrant 由 Docker 运行。Go API、Judge Worker 和 Python Agent 在宿主机运行。

### 1. 准备配置

```bash
test -f .env || cp .env.example .env
test -f config/config.dev.yaml || cp config/config.example.yaml config/config.dev.yaml
```

| 文件 | 必填配置 |
| --- | --- |
| `.env` | 填写 `MYSQL_PASSWORD`、`MYSQL_ROOT_PASSWORD`、`REDIS_PASSWORD` 和 `AGENT_SERVICE_TOKEN`。使用 AI 时填写 `DEEPSEEK_API_KEY`，使用向量搜索和记忆时填写 `DASHSCOPE_API_KEY`。 |
| `config/config.dev.yaml` | 填写 `sql.dsn`、`redis.password`、`jwt.secret` 和 `chat.agent_service_token`。MySQL、Redis 和 Agent 的值与 `.env` 对应。 |

保留 Go 配置的其他字段。默认数据库名、用户均为 `gojo`，本机 DSN 格式如下。

```text
gojo:<MYSQL_PASSWORD>@tcp(127.0.0.1:3306)/gojo?charset=utf8mb4&parseTime=True&loc=Local
```

### 2. 启动前端和基础依赖

```bash
docker compose up -d --build
docker compose ps
```

等待 MySQL 就绪，并确认 `curl -fsS http://127.0.0.1:9200` 能返回 Elasticsearch 信息。Vue 容器通过 `host.docker.internal` 访问宿主机上的 Go API。使用原生 Linux Docker Engine 时，可[在宿主机运行 Vue](docs/development.md#在宿主机运行-vue)。

### 3. 启动应用

在三个终端中分别运行 Python Agent、Go API 和 Judge Worker。仅使用题库和判题时可以跳过 Python Agent。

**Python Agent**

```bash
python3.11 -m venv .venv
.venv/bin/python -m pip install -r agent/requirements.txt
.venv/bin/python -m uvicorn app:app --app-dir agent --env-file .env \
  --host 127.0.0.1 --port 8000 --reload --reload-dir agent
```

**Go API**

```bash
APP_ENV=dev go run ./cmd/server
```

**Judge Worker**

```bash
docker pull golang:alpine
mkdir -p "$HOME/.cache/gojo-judge-dev"
APP_ENV=dev DOCKER_HOST="$(docker context inspect --format '{{.Endpoints.docker.Host}}')" \
  TMPDIR="$HOME/.cache/gojo-judge-dev" go run ./cmd/judge_worker
```

打开 [前端](http://localhost:3000) 注册并登录。Go API 和 Agent 的存活检查分别位于 [8080/ping](http://127.0.0.1:8080/ping) 和 [8000/ping](http://127.0.0.1:8000/ping)。

空题库可用 `APP_ENV=dev go run ./cmd/seed_problems` 导入示例题。管理员初始化、试跑开关和测试命令见[本地开发指南](docs/development.md)。

## 生产部署

[docker-compose.prod.yml](docker-compose.prod.yml) 提供单机生产拓扑。[docker-compose.4g.yml](docker-compose.4g.yml) 可叠加使用，为 4 GiB 主机限制容器内存和 Worker 并发。Compose 网关默认只监听 `127.0.0.1:8088`，公网 HTTPS 由宿主机反向代理终止。[生产部署指南](docs/deployment.md)包含配置、初始化、更新和日志查询命令。

## 仓库结构

| 路径 | 职责 |
| --- | --- |
| [vue/](vue/) | Vue 3 用户界面、管理后台和 Vite 开发服务 |
| [cmd/](cmd/) | Go API、Judge Worker、管理员初始化和示例题导入入口 |
| [internal/](internal/) | 业务模块、队列、判题沙箱和 SSE |
| [agent/](agent/) | FastAPI 服务、LangGraph 对话流程、工具调用和向量检索 |
| [config/](config/) | Go 配置加载与环境模板 |
| [infrastructure/](infrastructure/) | MySQL、Redis、Elasticsearch 和 Qdrant 相关配置 |

## 文档

| 文档 | 内容 |
| --- | --- |
| [系统架构](docs/architecture.md) | 组件职责、判题链路、对话编排和存储边界 |
| [本地开发](docs/development.md) | 配置来源、管理员、试跑和测试 |
| [生产部署与运维](docs/deployment.md) | Compose、HTTPS、发布、日志和排障 |
| [API 参考](docs/api.md) | 主要业务接口、SSE 和 Agent 服务接口 |

[文档索引](docs/README.md)保留历次审查和设计记录。
