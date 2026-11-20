# w0r_oj

w0r_oj 是一个面向 Go 算法练习的在线判题系统。用户可以从题库选题，在浏览器里编写和试跑代码，提交正式判题，再根据提交结果复盘。

## 功能

| 功能 | 说明 |
| --- | --- |
| 题库 | 按关键词、题号和知识点查找题目。登录后可查看已通过的题目，从详情页返回时会恢复筛选和页码。 |
| 写题与判题 | Go 编辑器支持代码补全、浏览器草稿和自定义输入。提交后异步判题，结果会自动更新。 |
| AI 学习助手 | 根据用户的提交记录分析错题、解释算法和推荐练习。可从失败的提交直接发起分析。 |
| 学习记录与管理 | 提供个人提交记录和排行榜。管理员可以维护用户、题目、知识点及测试用例。 |

前端使用 React、Vite 和 CodeMirror。Go API 处理业务请求与实时通知，独立的 Judge Worker 在 Docker 沙箱中编译和运行提交。学习助手由 Python FastAPI 和 LangGraph 提供，MySQL 持久化业务记录，Redis 承载异步任务，Elasticsearch 和 Qdrant 分别提供关键词与向量检索。[系统架构](docs/architecture.md)

## 本地启动

在仓库根目录执行。需要 **Go 1.26**、**Docker** 和 **Docker Compose v2**。启用 AI 学习助手时还需要 **Python 3.11**。直接在宿主机运行前端时需要 **Node.js 22**。

### 1. 准备配置

```bash
test -f .env || cp .env.example .env
test -f config/config.dev.yaml || cp config/config.example.yaml config/config.dev.yaml
```

| 文件 | 需要填写的配置 |
| --- | --- |
| `.env` | 设置 `MYSQL_PASSWORD`、`MYSQL_ROOT_PASSWORD`、`REDIS_PASSWORD` 和 `AGENT_SERVICE_TOKEN`。使用 AI 时设置 `DEEPSEEK_API_KEY`，启用向量检索与长期记忆时再配置 `DASHSCOPE_API_KEY`。 |
| `config/config.dev.yaml` | 设置 `sql.dsn`、`redis.password`、`jwt.secret` 和 `chat.agent_service_token`。数据库、Redis 和 Agent 的凭据与 `.env` 对应。需要使用「运行代码」时，将 `judge.trial_enabled` 设为 `true`。 |

默认数据库名与用户名均为 `gojo`。将下方的 `<MYSQL_PASSWORD>` 替换为 `.env` 中的业务账号密码，填入 `sql.dsn`。

```text
gojo:<MYSQL_PASSWORD>@tcp(127.0.0.1:3306)/gojo?charset=utf8mb4&parseTime=True&loc=Local
```

### 2. 启动前端和基础依赖

```bash
docker compose up -d --build
docker compose ps
curl -fsS http://127.0.0.1:9200
```

Compose 启动 React 开发服务、MySQL、Redis、Elasticsearch 和 Qdrant。等 MySQL 显示 `healthy` 且 Elasticsearch 可以访问后，再启动 Go API。前端容器通过 `host.docker.internal` 连接宿主机。原生 Linux Docker Engine 可采用宿主机启动方式。[在宿主机运行前端](docs/development.md#在宿主机运行前端)

### 3. 启动 Go API 和 Judge Worker

在一个终端启动 Go API。首次启动会初始化业务表。

```bash
APP_ENV=dev go run ./cmd/server
```

在另一个终端准备判题镜像和临时目录，再启动 Judge Worker。

```bash
docker pull golang:alpine
docker build -f Dockerfile.judge-compiler -t gojo-judge-compiler:go1.26-v1 .
mkdir -p "$HOME/.cache/gojo-judge-dev"
APP_ENV=dev DOCKER_HOST="$(docker context inspect --format '{{.Endpoints.docker.Host}}')" \
  TMPDIR="$HOME/.cache/gojo-judge-dev" go run ./cmd/judge_worker
```

打开前端注册并登录。空题库可另开终端执行 `APP_ENV=dev go run ./cmd/seed_problems` 导入示例题。[本地前端](http://localhost:3000) · [Go API 健康检查](http://127.0.0.1:8080/ping) · [初始化管理员](docs/development.md#管理员和示例题)

### 4. 按需启动学习助手

完成模型配置后，在独立终端启动 Python Agent。

```bash
python3.11 -m venv .venv
.venv/bin/python -m pip install -r agent/requirements.txt
.venv/bin/python -m uvicorn app:app --app-dir agent --env-file .env \
  --host 127.0.0.1 --port 8000 --reload --reload-dir agent
```

Agent 启动后可以检查服务是否存活。[Agent 健康检查](http://127.0.0.1:8000/ping)

## 部署

单机生产环境由 Docker Compose 管理。Nginx 提供 React 静态文件并转发 API 请求，网关默认绑定 `127.0.0.1:8088`，宿主机反向代理提供公网 HTTPS。4 GiB 服务器可叠加资源配置。[生产 Compose](docker-compose.prod.yml) · [4 GiB 配置](docker-compose.4g.yml)

`main` 更新后，GitHub Actions 运行 Go、Python 测试和 React 前端构建。启用自动部署时，通过检查的提交会部署到腾讯云，网关健康检查通过后记录上线版本。[部署与运维指南](docs/deployment.md) · [自动部署配置](docs/deployment.md#github-actions-自动部署)

## 仓库结构

| 路径 | 职责 |
| --- | --- |
| [frontend/](frontend/) | React 页面、路由、Go 编辑器和 Vite 开发服务 |
| [cmd/](cmd/) | Go API、Judge Worker、管理员初始化和示例题导入入口 |
| [internal/](internal/) | Go 业务模块、队列、判题沙箱和实时事件 |
| [agent/](agent/) | FastAPI 服务、LangGraph 对话流程、工具调用和语义检索 |
| [config/](config/) | Go 配置加载与环境模板 |
| [infrastructure/](infrastructure/) | MySQL、Redis、Elasticsearch 和 Qdrant 的接入与配置 |
| [nginx/](nginx/) · [ops/](ops/) | 网关配置和部署脚本 |

## 文档

- [系统架构](docs/architecture.md)
- [本地开发与测试](docs/development.md)
- [生产部署与排障](docs/deployment.md)
- [API 参考](docs/api.md)
- [文档索引](docs/README.md)
