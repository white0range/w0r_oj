# Gojo OJ

Gojo OJ 是一个面向算法学习场景的在线判题与 AI 学习助手平台。项目使用 Go + Gin 提供核心业务 API，以 Redis 承载缓存、异步任务和排行榜，通过 Docker Engine 隔离编译与运行用户代码，并结合 Elasticsearch、FastAPI、LangChain、Qdrant 和 DashScope Embedding 实现题目搜索、语义召回、错题分析及个性化学习建议。

项目覆盖题目管理、代码提交、异步判题、提交记录、排行榜、用户与权限管理、AI 多轮会话和管理后台等完整链路，适合作为 Go 后端工程、异步任务、容器沙箱与 RAG Agent 的综合实践项目。

## 核心能力

- **在线判题**：提交任务进入 Redis 队列，由 Go Worker 调用本地 Docker API 完成编译、逐用例运行与结果汇总。
- **资源隔离**：运行容器关闭网络，并限制 CPU、内存、进程数、执行时间和输出规模。
- **题目系统**：支持题目、标签、测试用例的管理，以及分页列表、详情、标签筛选和提交记录查询。
- **搜索与推荐**：Elasticsearch 服务后端关键词搜索，也是 Agent 混合召回中的词法通道；Qdrant 保存题目向量并提供语义召回。
- **AI 学习助手**：LangChain Agent 可调用 AC 记录、失败提交详情、标签统计和候选题等工具，完成错题分析、薄弱点总结与学习建议。
- **多轮记忆**：MySQL 保存完整会话和结构化回复，历史消息使用窗口与摘要压缩；Qdrant 保存用户长期向量记忆。
- **最终一致性同步**：题目变化后通过 Redis List 异步同步 Elasticsearch 和 RAG，用户 AC 后同步 Redis ZSet 排行榜，并提供重试、租约恢复、死信与定期校准。
- **双 Token 鉴权**：用户侧采用 Access Token + Refresh Token；Go 与 FastAPI 之间同时校验内部 Service Token 和受控 JWT。

## 系统架构

```mermaid
flowchart LR
    U["用户浏览器"] --> V["Vue 3 / Vite"]
    V -->|"HTTP / SSE"| G["Go API / Gin"]

    G --> M[("MySQL<br/>业务事实数据")]
    G --> R[("Redis<br/>缓存、队列、ZSet")]
    G --> E[("Elasticsearch<br/>关键词检索")]

    R --> JW["Judge Worker"]
    JW --> D["Docker Engine"]
    D --> C["编译容器 / 运行沙箱"]
    JW --> M
    JW -->|"判题结果事件"| R
    R -->|"事件订阅"| G
    G -->|"SSE 推送"| V

    R --> CW["Chat Worker"]
    CW -->|"JWT + Service Token"| F["FastAPI / LangChain Agent"]
    F -->|"受控工具调用"| G
    F --> Q[("Qdrant<br/>题目向量、长期记忆")]
    F --> L["DeepSeek Chat"]
    F --> B["DashScope Embedding"]

    R --> SW["Sync Worker"]
    SW --> E
    SW --> F
    SW --> R
```

### 组件职责

| 组件 | 主要职责 |
| --- | --- |
| Vue 3 | 用户端与管理后台，维护双 Token，通过 SSE 接收回合状态和判题通知 |
| Go / Gin | 公网业务 API、认证授权、题目与提交管理、判题入队、Chat 调度、内部 Agent 工具接口；不持有 Docker Socket |
| Judge Worker | 无公网端口的独立 Go 服务，消费判题队列并通过 Docker Engine 创建编译和运行沙箱 |
| MySQL / GORM | 用户、题目、测试用例、提交、会话、消息、回合和反馈等事实数据 |
| Redis | 旁路缓存、提交限流、判题队列、Chat 队列、同步队列和排行榜 ZSet |
| Docker Engine | 编译用户代码，并在受限容器内执行测试用例 |
| Elasticsearch | 题目关键词检索、标签精确筛选，以及 Agent 混合召回中的词法候选 |
| FastAPI / LangChain | Agent 编排、工具调用、结构化输出、会话摘要和 RAG 同步入口 |
| Qdrant | 题目向量集合和按用户隔离的长期会话记忆 |
| DeepSeek | Chat Agent 推理模型 |
| DashScope Embedding | 题目、查询和长期记忆向量化 |

## 核心业务链路

### 异步判题

1. 用户调用 `POST /api/submit`，提交题目 ID、语言和源代码。
2. Go API 先把提交记录写入 MySQL，再将任务写入 Redis List `judge:pending`。
3. 独立 Judge Worker 协程池通过 Lua 原子地将任务从 Pending 移到 Processing 并设置租约，再通过 Docker SDK 连接宿主机 Docker Engine。
4. 编译阶段在 `golang:alpine` 容器内分别编译用户代码和内置判题 Runner。用户代码编译失败返回 `CE`，Runner 编译失败视为系统错误。
5. 编译成功后，为本次提交创建关闭网络的运行沙箱；多个测试用例通过 Docker Exec 依次执行，避免重复创建容器。
6. `ContainerExecAttach` 提供本次 Exec 的实时输出流，`stdcopy.StdCopy` 将 Docker 复用流拆分为 `stdout` 和 `stderr`；Runner 输出 JSON 形式的退出码、信号、耗时、内存与程序输出。
7. 判题服务依次判断系统错误、超时、内存超限、运行错误和输出差异，遇到首个失败用例后停止。
8. 最终状态由 Judge Worker 写回 MySQL，并通过 Redis Pub/Sub 发布 `judge_result`；API 订阅后转发 SSE，提交结果查询接口作为通知丢失时的兜底。

| 状态 | 含义 |
| --- | --- |
| `AC` | Accepted，答案正确 |
| `WA` | Wrong Answer，输出不匹配 |
| `CE` | Compile Error，用户代码编译失败 |
| `RE` | Runtime Error，非零退出或异常信号 |
| `TLE` | Time Limit Exceeded，运行超时 |
| `MLE` | Memory Limit Exceeded，内存超限 |
| `SE` | System Error，Docker、Runner 或基础设施错误 |

判题沙箱当前实施的主要限制：

- `NetworkMode=none`，禁止用户代码访问网络。
- 限制内存，并将 Memory Swap 设置为相同上限，避免额外交换空间。
- 通过 `NanoCPUs` 限制 CPU，通过 `PidsLimit` 限制进程数量。
- 编译和运行均有外层 Context 超时；用例还检查 Runner 上报的 CPU 时间、墙钟时间和最大常驻内存。
- 用户代码与判题 Runner 分开编译，因此能够区分用户编译错误和判题基础设施错误。
- 编译和运行容器带有 `gojo.sandbox=true`、所属环境、类型和提交 ID Label；服务启动、Judge Worker 开始消费前会按当前环境的 Label 强制清理上次异常退出留下的容器，并清理 `TMPDIR` 下的 `judge_*` 工作目录。

> 当前语言适配器只支持 Go，运行镜像为 `golang:alpine`。扩展 C++、Java 或 Python 时，需要为每种语言提供独立镜像、编译命令、运行命令和资源策略。

### 题目搜索与数据同步

MySQL 是题目与用户积分的事实数据源。题目新增、修改、删除或标签变化后，业务服务向 Redis 写入 ES 与 RAG 同步任务；用户首次 AC 后写入排行榜同步任务。

| Redis Key | 数据结构 | 用途 |
| --- | --- | --- |
| `sync:pending` | List | 等待消费的同步任务 |
| `sync:processing` | List | 已领取但尚未确认的任务 |
| `sync:processing:leases` | ZSet | Processing 任务的租约到期时间 |
| `sync:retry_at` | ZSet | 按下次执行时间排序的重试任务 |
| `sync:dead_letter` | List | 超过重试次数或无法解析的任务 |

Worker 通过 Lua 脚本原子地将任务从 Pending 移到 Processing 并设置租约。成功后 ACK；失败后按 `1m、5m、30m、2h、12h` 退避，最多重试 8 次。后台协程定期提升到期重试、恢复租约超时任务，并每 30 分钟从 MySQL 发起全量校准。

- **Elasticsearch**：同步 `problems_current` Alias 指向的 IK 索引。查询对 `title^3` 和 `description` 执行 `multi_match`，通过 `tags` 精确筛选。
- **Qdrant RAG**：Sync Worker 调用 FastAPI 的 `/rag/problems/sync` 或 `/rag/problems/delete`；FastAPI 从 Go API 读取题目，调用 DashScope 生成向量后写入 Qdrant。
- **排行榜**：Redis ZSet `leaderboard:infrastructure` 以用户 ID 为 member，以 `solved_count * 10` 为 score。全量重建先写临时 Key，再通过 `RENAME` 原子替换。

这套机制提供的是**最终一致性**而非分布式事务：MySQL 提交成功不代表 ES、Qdrant 与排行榜立即可见。生产环境应监控 Pending、Retry、Processing 和 Dead Letter，并提供受控死信重放能力。

### AI Chat、错题分析与学习建议

AI 能力统一收敛在 Chat 会话中，不再依赖独立 Study Plan 任务：

1. 用户创建 Chat Session 并发送消息。
2. Go 后端保存用户消息和 `chat_turns`，再把回合写入 `chat_turn_queue`。
3. Chat Worker 领取任务，写入 Processing Token 与租约，并调用 FastAPI `/chat/run`。
4. FastAPI 使用 LangChain `create_agent` 和 `ChatDeepSeek` 执行 Tool Calling。
5. Agent 按意图读取 AC 历史、失败提交、失败代码与实际输出、标签统计和候选题目。
6. 自然语言找题优先融合 Elasticsearch 关键词召回与 Qdrant 语义召回，再执行确定性重排。
7. Go 将展示文本写入 `chat_messages.content`，将原始结构写入 `structured_payload` 和 Turn Result。
8. 前端通过 SSE 监听回合状态，连接异常时回退轮询；回合完成后可继续对话或提交反馈。

Agent 的主要工具：

- `user_ac_history`：读取已通过题目。
- `user_failed_submissions`：读取最近失败提交。
- `failed_submission_detail`：读取失败代码、状态、实际输出、题面和标签，用于具体错题分析。
- `user_tag_stats`：统计不同标签的通过情况。
- `candidate_problems`：按标签获取未完成候选题。
- `semantic_candidate_problems`：从 Qdrant 获取语义候选。
- `hybrid_candidate_problems`：融合 Elasticsearch 和 Qdrant 结果并重排。

结构化输出：

```json
{
  "answer": "面向用户的分析或建议",
  "weak_tags": ["动态规划", "图论"],
  "recommended_problems": [
    {
      "problem_id": 42,
      "title": "示例题目",
      "reason": "推荐原因"
    }
  ],
  "response_type": "analysis"
}
```

Go 端保留最近消息窗口，并将较早消息压缩为 Session Summary；FastAPI 还会从 Qdrant 检索当前用户的长期记忆。长期记忆按 `user_id` 隔离，默认召回 Top 3。Python 生成回答时不直接写入记忆；Go 确认结果落库后，复用 Redis 同步队列调用 `/memory/sync`。记忆使用用户、回合和类型生成固定 UUID，同一回合重试时更新同一个点。

### Agent 工具访问边界

Go 与 FastAPI 使用独立的固定服务密钥鉴权，不再签发或转发管理员 JWT，也不要求存在管理员账号。两个服务之间的请求使用 `X-Agent-Service-Token`，Go 的 `chat.agent_service_token` 与 Python 的 `AGENT_SERVICE_TOKEN` 必须一致。

Agent 工具接口独立放在 `/api/agent/*` 下，使用 `AgentAuth` 中间件，不经过用户登录或管理员中间件。Python 回调时还需携带 `X-Agent-Turn-ID` 与 `X-Agent-Processing-Token`；Go 检查回合正在运行、执行租约未过期、执行编号等于当前 `ProcessingToken`、会话仍有效，并从回合记录确定用户身份。用户工具采用 `/api/agent/me/*`，不接受模型指定用户 ID；失败提交详情仍检查提交归属，并拒绝将 AC 提交作为错题分析目标。

生产 Nginx 对 `/api/agent` 及 `/api/agent/*` 直接返回 404，不转发到 Go。Python 通过 Docker 内网的 `GO_BACKEND_BASE_URL=http://api:8080` 直连 Go；不要将该地址配置为网站域名或网关地址。Nginx 配置打包在 gateway 镜像中，修改后需重新构建并更新 gateway 容器。

`POST /chat/run` 请求必须包含正整数 `turn_id`、`user_id` 和 64 字符的 `processing_token`，由 Go Worker 提供。Python 将执行信息绑定在工具闭包中，不暴露为模型参数；在读取记忆前、模型调用前后及工具调用前校验执行状态。Go 返回 `40302` 表示执行失效，Python 会直接中止并返回 HTTP 409，不交给模型重试。会话摘要和题库向量同步只需要服务密钥，不需要用户 JWT。公共题目搜索仍复用 `/api/problems/search`。

服务密钥具有服务级权限，应只保存在服务端配置中，不能发送给前端。回合编号用于选择业务上下文，复用已有的 `ProcessingToken` 区分重试执行；执行编号不能替代服务密钥，也不进入提示词或普通日志。部署时使用受控内部网络，跨公网调用使用 HTTPS，修改密钥后同步重启 Go 和 Python。

### 记忆同步可靠性

`chat_turns.memory_sync_pending` 与成功结果在同一事务中写入。Go 随后立即投递记忆同步任务；若结果提交后退出或 Redis 入队失败，启动时及每 30 秒的扫描会补发待同步记录。扫描只处理新版本确认成功并设置标记的回合，不回填历史记忆。Redis 按回合去重，防止补发绕过现有重试退避；任务只包含回合 ID，消费者从 MySQL 读取已确认的结果、原始消息和完成时间。Python 确认 Qdrant 写入成功后才清除待同步标记。

超过现有重试上限后进入 `sync:dead_letter`，并保留 `sync:chat_memory:dispatch:<turn_id>` 标记，避免扫描反复重启死信任务。修复外部故障后可从死信中显式重放；不要直接删除正在处理或等待退避的分发标记。具体约定见 `docs/2026-11-17-agent-execution-fencing.md`。

### Agent 执行日志

Go 和 Python 的关键运行信息都带 `turn_id`，直接输出到标准输出，生产由 Docker 收集和轮转。Python 使用 LangChain/LangGraph 回调记录模型、工具的开始、结束、耗时、实际 Token 用量和错误类型，并记录摘要、记忆召回、结果解析与异步记忆同步。重试通过现有 ProcessingToken 的单向摘要 `attempt_id` 区分，原始执行令牌不进入日志。

不需要外部追踪账号、Key、追踪服务器或本地日志脚本。日志不记录提示词、完整对话、源码、工具返回正文或原始异常消息；应用主动关闭框架自动云端追踪。部署与查询方法见 [执行日志说明](docs/2026-11-17-agent-tracing-plan.md)。

## 缓存与排行榜

项目只对明确的热点读取使用旁路缓存：

| Key | TTL | 内容与失效策略 |
| --- | --- | --- |
| `cache:problems:page:{page}:limit:{limit}:tag:{tag}` | 1 小时 | 题目分页；题目或标签变化后批量失效 |
| `cache:problem:detail:{id}` | 24 小时 | 题目详情；题目、标签、测试用例或判题统计变化后失效 |
| `cache:tags:all` | 7 天 | 标签列表；标签新增或删除后失效 |
| `rate_limit:submit:{user_id}` | 5 秒窗口 | 用户提交限流 |
| `leaderboard:infrastructure` | 持久 | 全局排行榜，由增量同步和周期校准维护 |

题目列表缓存不保存用户个性化的 `is_ac`。命中公共缓存后，服务仍会查询当前用户在本页题目的 AC 集合，避免不同用户之间串数据。

### 内测限流

默认配置面向少量熟人内测：单 IP 每小时最多发起 3 次注册，全站每个中国时区自然日最多成功注册 10 人；单用户每 5 秒最多提交 1 次、每小时最多 60 次，AI Chat 每日最多 2 条消息。登录、搜索、SSE Ticket 和同时 SSE 连接也有独立配额，详见 `config/config.production.example.yaml` 中的 `rate_limit`。限流计数保存在 Redis；Redis 不可用时会拒绝受限接口，而不是绕过限流。

## 技术栈

| 层次 | 技术 |
| --- | --- |
| 前端 | Vue 3、Vue Router、Axios、Vite |
| Go 后端 | Go 1.26、Gin、GORM、go-redis、Docker SDK、Elasticsearch Go Client |
| Python Agent | Python、FastAPI、LangChain、langchain-deepseek、Pydantic |
| 数据存储 | MySQL、Redis、Elasticsearch 8.11、Qdrant |
| AI 服务 | DeepSeek Chat、DashScope `text-embedding-v3` |
| 容器与运行 | Docker、Docker Compose |

## 项目结构

```text
.
├── agent/                     # FastAPI、LangChain Agent、工具执行与 RAG
│   ├── app.py                 # Agent HTTP 入口与服务鉴权
│   ├── langchain_runner.py    # Agent、摘要与长期记忆编排
│   ├── tool_executor.py       # 工具边界、混合召回与重排
│   └── rag/                   # Qdrant 索引、检索、记忆和导入工具
├── cmd/
│   ├── server/                # Go HTTP API、Chat/Sync Worker 启动入口
│   ├── judge_worker/          # 无公网端口的独立判题 Worker 入口
│   └── seed_problems/         # 示例题目初始化命令
├── config/                    # 多环境配置加载与示例
├── infrastructure/           # MySQL、Redis、Elasticsearch 客户端
├── internal/
│   ├── app/                   # 路由、中间件、统一响应与错误码
│   ├── chat/                  # Session、Message、Turn、Feedback 与 Worker
│   ├── judge/                 # Docker 编译、Runner、限制和判题服务
│   ├── leaderboard/           # Redis ZSet 排行榜
│   ├── problem/               # 题目、标签、测试用例、缓存和 ES 搜索
│   ├── submission/            # 提交记录与判题任务入队
│   ├── syncer/                # ES、RAG、排行榜最终一致性任务
│   └── user/                  # 双 Token、封禁和用户资料
├── migrations/                # 需人工确认执行的历史 SQL
├── vue/                       # Vue 用户端和管理后台
├── docker-compose.yml         # 本地开发前端与数据库、中间件
├── docker-compose.prod.yml    # 单机生产服务拓扑
├── docker-compose.4g.yml      # 4GB 主机的低内存叠加配置
├── .env.example              # 开发环境变量示例
├── .env.local-production.example # 生产 Compose 环境变量模板
└── go.mod                     # Go 模块与依赖
```

## 本地运行

### 环境要求

- Go 1.26 或兼容版本，以 `go.mod` 为准
- Python 3.11 或兼容版本
- Docker Desktop 或 Docker Engine
- DeepSeek API Key 和 DashScope API Key，用于 AI 对话和向量检索

本地开发时，Go API、Chat Worker、Judge Worker 和 Python Agent 在宿主机运行。`docker-compose.yml` 启动 Vue 开发服务、MySQL、Redis、Elasticsearch 和 Qdrant。Kibana 使用 `debug` profile 按需启动。

### 1. 准备本地配置

在仓库根目录执行，已有私有配置时核对对应字段：

```bash
cp -n .env.example .env
cp -n config/config.example.yaml config/config.dev.yaml
```

编辑 `.env`，填写本地 MySQL 用户密码、Root 密码、Redis 密码、`AGENT_SERVICE_TOKEN`、DeepSeek 和 DashScope Key。Python 使用 `GO_BACKEND_BASE_URL=http://127.0.0.1:8080` 和 `QDRANT_URL=http://127.0.0.1:6333` 连接本机 API 与映射端口。

Go 默认读取 `config/config.dev.yaml`。在已有配置块内填写下面的字段，数据库名和用户与 `.env` 一致，密码与 `MYSQL_PASSWORD` 一致：

```yaml
sql:
  dsn: "gojo:你的本地MySQL密码@tcp(127.0.0.1:3306)/gojo?charset=utf8mb4&parseTime=True&loc=Local"

redis:
  addr: "127.0.0.1:6379"
  password: "与.env中的REDIS_PASSWORD一致"

jwt:
  secret: "填写本地随机密钥"

elasticsearch:
  addresses:
    - "http://127.0.0.1:9200"

chat:
  worker_count: 1
  agent_base_url: "http://127.0.0.1:8000"
  agent_timeout_seconds: 120
  agent_service_token: "与.env中的AGENT_SERVICE_TOKEN一致"

judge:
  worker_count: 1
```

Go 从进程环境中的 `GOJO_*` 变量覆盖 YAML 配置，例如 `GOJO_SQL_DSN`。本机 Go 通过 YAML 或启动环境取得配置；Python 的启动命令用 `--env-file .env` 加载环境文件。

### 2. 启动 Docker 中的依赖和前端

```bash
docker compose up -d --build
docker compose ps
curl -fsS http://127.0.0.1:9200
```

等待 MySQL 变为 `healthy`、Elasticsearch 能响应后继续。MySQL 首次启动会创建 `.env` 中指定的数据库和用户，后续启动会复用 `mysql_data` 数据卷。已有数据卷的账号密码以初始化时的值为准。Go 在初始化管理员或启动 API 时通过 GORM `AutoMigrate` 补齐业务表。

Vue 容器首次启动会执行 `npm ci`，完成后可访问 `http://localhost:3000`。源码挂载到容器，Vite 支持热更新；容器把 `/api` 代理到宿主机的 `http://host.docker.internal:8080`。Mac Docker Desktop 提供这个地址。

如需 Kibana：

```bash
docker compose --profile debug up -d kibana
```

### 3. 在本机启动 Python Agent

以下命令均在仓库根目录执行，虚拟环境只需创建和安装一次：

```bash
python3.11 -m venv .venv
.venv/bin/python -m pip install -r agent/requirements.txt
.venv/bin/python -m uvicorn app:app --app-dir agent --env-file .env \
  --host 127.0.0.1 --port 8000 --reload --reload-dir agent
```

### 4. 初始化本地管理员

如需管理题目、标签或用户，可初始化管理员；Chat Worker 不依赖该账号。普通使用也可以直接通过前端注册账号。在新终端的仓库根目录执行 `read` 后输入管理员密码并回车，密码长度为 12–72 字节：

```bash
read -rs BOOTSTRAP_ADMIN_PASSWORD
export BOOTSTRAP_ADMIN_PASSWORD
APP_ENV=dev BOOTSTRAP_ADMIN_USERNAME=admin go run ./cmd/bootstrap_admin
unset BOOTSTRAP_ADMIN_PASSWORD
```

用户名长度为 3–32 个字符。再次执行时，若同名账号已经是启用的管理员，工具会安全退出。

### 5. 在本机启动 Go API 和 Judge Worker

API 在一个终端运行：

```bash
APP_ENV=dev go run ./cmd/server
```

Judge Worker 在另一个终端运行。Mac 上显式传入当前 Docker context 的 socket，并使用可共享的临时目录：

```bash
docker pull golang:alpine
mkdir -p "$HOME/.cache/gojo-judge-dev"
APP_ENV=dev DOCKER_HOST="$(docker context inspect --format '{{.Endpoints.docker.Host}}')" \
  TMPDIR="$HOME/.cache/gojo-judge-dev" go run ./cmd/judge_worker
```

API 默认监听 `http://localhost:8080`，启动 Chat、Sync Worker 和 Redis 实时事件桥接。Judge Worker 通过 Docker Engine 创建编译、运行沙箱。首次需要示例题时，在仓库根目录执行 `APP_ENV=dev go run ./cmd/seed_problems`。

本地服务直接在终端输出运行信息，项目不再额外保存本地日志文件。Agent 执行日志按回合任务 ID 关联，使用说明见 [Agent 执行日志](docs/2026-11-17-agent-tracing-plan.md)。

### 6. 检查访问入口

| 服务 | 默认地址 |
| --- | --- |
| Vue 开发页面 | `http://localhost:3000` |
| Go API | `http://127.0.0.1:8080/ping` |
| Python Agent | `http://127.0.0.1:8000/ping` |
| MySQL | `127.0.0.1:3306` |
| Redis | `127.0.0.1:6379` |
| Elasticsearch | `http://127.0.0.1:9200` |
| Qdrant HTTP | `http://127.0.0.1:6333/collections` |
| Kibana，启用 debug profile 后 | `http://localhost:5601` |

先验证登录和题库，再提交代码并检查终态，最后测试 AI 错题分析和推荐。

### 生产环境密钥要求

上线时设置 `APP_ENV=prod`（或 `production`）。仓库提供可提交、无密钥的 `config/config.production.example.yaml`：Docker 生产镜像会将它作为 `config.production.yaml` 使用，并由 `GOJO_*` 环境变量注入密钥。若直接运行 Go 程序，请先复制该模板为本机私有的 `config/config.production.yaml`。Go 后端会在连接数据库前拒绝启动，除非下列三个值均为至少 32 个字符的随机值：`jwt.secret`、`chat.agent_service_token`、`redis.password`。Agent 也会拒绝使用空、过短或示例形式的 `AGENT_SERVICE_TOKEN` 启动。

`chat.agent_service_token` 与 `.env` 中的 `AGENT_SERVICE_TOKEN` 必须完全一致；JWT Secret 和 Redis 密码必须分别独立生成，不能复用。可用密码管理器或 `openssl rand -hex 32` 生成随机值，且不要提交本机的 `config.production.yaml` 或 `.env`。

## 4GB 云服务器部署

`docker-compose.prod.yml` 是完整的单机生产拓扑，`docker-compose.4g.yml` 是必须与它同时使用的叠加配置。后者将 Judge 和 Chat Worker 并发数降为 1，收紧 MySQL 连接池、缓存和各容器内存上限，保留 Elasticsearch、Qdrant、AI 与完整判题功能。它不会创建另一套服务，而是按服务名合并并覆盖资源参数。

推荐主机基线为 4 vCPU、4 GiB 内存、40 GB 磁盘和 2 GB Swap。该配置面向少量用户的内测，不代表高并发或强隔离的多租户生产方案。首次部署：

```bash
git clone https://github.com/white0range/w0r_oj.git
cd w0r_oj
cp .env.local-production.example .env.local-production
stat -c '%g' /var/run/docker.sock
```

编辑 `.env.local-production`：替换 MySQL 密码和三个至少 32 字符的独立 Secret，填写 `DEEPSEEK_API_KEY`、`DASHSCOPE_API_KEY` 和管理员初始密码，再将上一条命令的数字结果写入 `DOCKER_GID`。不要将这个私有环境文件提交到 Git。

```bash
docker pull golang:alpine
docker compose --env-file .env.local-production \
  -f docker-compose.prod.yml \
  -f docker-compose.4g.yml \
  config --quiet
docker compose --env-file .env.local-production \
  -f docker-compose.prod.yml \
  -f docker-compose.4g.yml \
  up -d --build
docker compose --env-file .env.local-production \
  -f docker-compose.prod.yml \
  -f docker-compose.4g.yml \
  --profile tools run --rm bootstrap-admin
```

初始化完成后立即从环境文件清空 `BOOTSTRAP_ADMIN_PASSWORD`，然后检查：

```bash
docker compose --env-file .env.local-production \
  -f docker-compose.prod.yml \
  -f docker-compose.4g.yml \
  ps
curl -fsS http://127.0.0.1:8088/ping
docker stats --no-stream
```

Compose 网关默认只绑定 `127.0.0.1:8088`。公网上线时应在宿主机配置 Nginx 或 Caddy，将域名的 HTTPS 请求反向代理到该地址；安全组只开放必要的 `22`、`80`、`443` 端口，不要暴露 API 容器、MySQL、Redis、Elasticsearch、Qdrant 或 Docker Socket。

> 生产模板默认不信任任何代理 CIDR，因此在 Compose 网关后所有访客可能共享同一个 IP 限流配额。这对小规模内测偏保守但安全；如果需要精确的用户源 IP 限流，应先为 Docker 网络固定子网，再仅将网关的 IP/CIDR 加入 `server.trusted_proxy_cidrs`，不要盲目信任所有代理。

## 关键配置

### Go 配置

| 配置前缀 | 说明 |
| --- | --- |
| `server` | API 端口和 HTTP 超时 |
| `sql` | MySQL DSN 与连接池 |
| `redis` | Redis 地址、密码和 DB |
| `jwt` | Access Token 与内部 Agent JWT 的签名密钥 |
| `elasticsearch` | Elasticsearch 节点 |
| `judge` | 判题 Worker 数和编译超时 |
| `chat` | Chat Worker 数、Agent 地址、超时与内部 Service Token |

### Agent 环境变量

| 变量 | 默认值或用途 |
| --- | --- |
| `DEEPSEEK_API_KEY` | DeepSeek API Key，必填 |
| `DEEPSEEK_API_BASE` | 默认 `https://api.deepseek.com` |
| `LLM_MODEL` | 默认 `deepseek-chat` |
| `GO_BACKEND_BASE_URL` | Agent 容器访问 Go API，默认 `http://host.docker.internal:8080` |
| `AGENT_SERVICE_TOKEN` | 内部密钥，必须与 Go 配置一致 |
| `DASHSCOPE_API_KEY` | DashScope Embedding Key，必填 |
| `EMBEDDING_MODEL` | 默认 `text-embedding-v3` |
| `EMBEDDING_DIMENSION` | 默认 `1024`，修改后需重建 Qdrant Collection |
| `QDRANT_URL` | Compose 内默认 `http://qdrant:6333` |
| `MEMORY_COLLECTION` | 默认 `chat_memories` |
| `MEMORY_TOP_K` | 默认 `3` |
| `AGENT_DEBUG` | 调试日志开关，生产环境应关闭 |

## API 概览

受保护接口需携带 `Authorization: Bearer <access_token>`。

| 模块 | 方法与路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| 健康检查 | `GET /ping` | 公开 | Go 服务健康检查 |
| 认证 | `POST /api/register` | 公开 | 注册 |
| 认证 | `POST /api/login` | 公开 | 登录并获取双 Token |
| 认证 | `POST /api/refresh` | 公开 | 刷新 Token |
| 认证 | `POST /api/logout` | 公开 | 注销 Refresh Session |
| 题目 | `GET /api/problems` | 公开/可选登录 | 分页、标签筛选和用户 AC 状态 |
| 题目 | `GET /api/problems/:id` | 公开 | 题目详情 |
| 搜索 | `POST /api/problems/search` | 公开 | ES 关键词与标签搜索 |
| 提交 | `POST /api/submit` | 登录 | 创建提交并进入判题队列 |
| 提交 | `GET /api/submissions/:id` | 登录 | 查询自己的提交 |
| 提交 | `GET /api/my-submissions` | 登录 | 个人提交记录 |
| 排行榜 | `GET /api/leaderboard` | 公开/可选登录 | Top 50 与个人排名 |
| 实时通知 | `POST /api/events/ticket` | 登录 | 获取 60 秒有效、一次性的判题通知 SSE Ticket |
| 实时通知 | `GET /api/events?ticket=...` | Ticket | SSE 推送判题结果 |
| Chat | `POST /api/chat/sessions` | 登录 | 创建会话 |
| Chat | `GET /api/chat/sessions` | 登录 | 会话列表 |
| Chat | `GET /api/chat/sessions/:id/messages` | 登录 | 会话消息 |
| Chat | `POST /api/chat/sessions/:id/messages` | 登录 | 发送消息并创建回合 |
| Chat | `GET /api/chat/turns/:id` | 登录 | 查询回合状态 |
| Chat | `POST /api/chat/turns/:id/stream-ticket` | 登录 | 获取绑定该回合的一次性 SSE Ticket |
| Chat | `GET /api/chat/turns/:id/stream?ticket=...` | Ticket | SSE 监听回合 |
| Chat | `POST /api/chat/turns/:id/feedback` | 登录 | 提交 ChatPlanFeedback |
| 管理后台 | `/api/admin/*` | 管理员 | 用户、题目、标签与测试用例管理 |
| Agent 工具 | `/api/agent/*` | 服务密钥 + 有效回合上下文 | FastAPI 内部工具调用；独立于用户 JWT |

## 数据模型

- `users`：用户、角色、状态、已解决题数与 Token Version。
- `problems`、`tags`、`test_cases`：题目、标签关系与测试数据。
- `submissions`：代码、语言、状态、输出、耗时与内存。
- `chat_sessions`：会话元数据和历史摘要。
- `chat_messages`：用户/助手消息与结构化 Payload。
- `chat_turns`：异步回合、状态、租约和结果。
- `chat_plan_feedbacks`：用户对已完成 Chat 回合的反馈。

旧版 Study Plan 表不再由当前代码使用。确认旧表没有有效数据后，可执行：

```bash
mysql -u root -p gojo < migrations/20260718_remove_legacy_study_plan_tables.sql
```

## 测试与构建

### Go

```bash
go test -count=1 ./...
go build ./...
```

判题集成测试需要本地 Docker Engine 和 `golang:alpine` 镜像。

### Python

```bash
python -m compileall agent
```

宿主机直接运行 Agent：

```bash
python -m pip install -r agent/requirements.txt
uvicorn app:app --app-dir agent --host 0.0.0.0 --port 8000
```

### 前端

```bash
cd vue
npm run build
```

## 运维与排障

### RAG 同步返回 401

确认 `.env` 中存在 `AGENT_SERVICE_TOKEN`，并与 Go 配置完全相同。修改后重建 Agent：

```bash
docker compose up -d --build agent
```

### 判题无法连接 Docker

- 确认 Docker Desktop/Engine 正在运行。
- 确认独立 Judge Worker 正在运行；API 本身不应连接 Docker。
- 确认 Judge Worker 有权限访问 Docker API。
- Linux 环境检查 Judge Worker 对 `/var/run/docker.sock` 的组权限。
- 使用 `docker version` 和 `docker image inspect golang:alpine` 验证连接与镜像。

### Chat 长时间停留在 Pending 或 Running

- 检查 `chat_turn_queue` 和 `chat_turn_processing`。
- 检查 Go 到 Agent 的连通性，以及 Agent 到 `GO_BACKEND_BASE_URL` 的反向连通性。
- 检查 Go 和 Python 的 Service Token 是否一致，以及回合编号、执行编号、运行状态和租约是否有效。
- Chat Worker 以数据库状态为事实来源，并使用 Processing Token、租约续期和超时扫描恢复陈旧回合。

### 中文搜索召回较弱

ES 使用与 `elasticsearch:8.11.0` 匹配的 IK 插件。`title` 和 `description` 显式使用 `ik_max_word` 建立索引、`ik_smart` 查询；应用通过 `problems_current` Alias 读写版本化的 `problems_ik_v1` 索引。首次升级会从 MySQL 全量重新同步题目到新索引；确认搜索正常后，旧的动态 Mapping `problems` 索引可手动删除。

### 同步队列积压

```bash
redis-cli LLEN sync:pending
redis-cli LLEN sync:processing
redis-cli ZCARD sync:retry_at
redis-cli LLEN sync:dead_letter
redis-cli ZCARD leaderboard:infrastructure
```

不要直接删除 Processing 或 Dead Letter。应先定位 ES、Agent、Embedding 或 Qdrant 错误，再通过受控流程重放。

## 安全与生产化边界

- Docker Socket 是高权限系统接口。公网 API 已移除 Socket，只有不开放端口的 Judge Worker 持有；同机部署仍共享宿主机内核，公网多租户阶段应继续评估独立 Judge 节点和更严格的容器运行时。
- 当前判题只支持 Go，并依赖公共 `golang:alpine`。生产环境应固定镜像 Digest 并预拉取到 Judge 节点。
- Redis List 是项目内实现的轻量可靠队列，不等同于 Kafka 或 RabbitMQ；需要补充监控、死信重放、幂等审计和容量规划。
- Elasticsearch 与 Qdrant 是最终一致性副本，业务写入和搜索可见之间存在延迟。
- API Key、JWT Secret、MySQL 密码和 Service Token 不应提交 Git，应由 Secret Manager 或部署平台注入。
- `AGENT_DEBUG` 在生产环境应关闭，避免日志包含模型输入、工具结果或用户代码。
- FastAPI Agent 当前使用同步请求模式，高并发场景应评估异步 HTTP、模型限流、熔断、超时与成本预算。
- GORM AutoMigrate 适合开发环境，正式部署应使用可审计、可回滚的版本化迁移。

## 判题服务隔离与后续改进

公网 API 与 Judge Worker 已拆分为两个独立进程。API 只创建 `Pending` 提交并写入 Redis 可靠判题队列，不初始化 Docker Client，也不挂载 `/var/run/docker.sock`。Judge Worker 不开放公网端口，按固定沙箱配置创建容器、写回 MySQL，并通过 Redis Pub/Sub 发布轻量结果事件；API 订阅事件后转发 SSE，前端查询提交结果作为兜底。

当前架构：

```text
Browser -> Gateway -> API -> Redis judge queue -> Judge Worker -> Docker Engine
                           |                    |
                           v                    v
                         MySQL <----------- judge result

Judge Worker -> Redis Pub/Sub event -> API -> SSE / polling -> Browser
```

后续强化项：

1. 将 Judge Worker 与 Docker Engine 迁移到独立主机或私有网络节点，使容器逃逸不直接影响业务 API 和数据库所在主机。
2. 为 Worker 配置最小权限数据库账号，只允许读取题目和测试用例、更新判题相关字段。
3. 在现有固定 Label 与单实例启动清理基础上补充周期性清理、容器数量与资源监控、任务审计和受控死信重放；多 Worker 部署时清理必须结合实例归属和任务租约。
4. 评估 rootless Docker、user namespace、定制 seccomp/AppArmor、gVisor 或其他更强隔离运行时。
5. 如果判题通知需要离线重放，将当前最佳努力的 Redis Pub/Sub 升级为 Redis Stream；MySQL 仍保持最终结果的事实来源。
