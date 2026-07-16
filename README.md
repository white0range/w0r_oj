# Gojo OJ

Gojo OJ 是一个全栈算法训练平台，将在线判题、异步任务处理、搜索检索和 AI 辅助学习整合到同一套系统中。Go 后端是业务数据的唯一事实来源，Vue 前端负责交互，Python Agent 服务负责对话式引导、检索增强和学习记忆。

这不是一个只保存题目并返回结果的演示型 OJ。项目已经覆盖真实系统的关键运行问题，包括队列判题、缓存失效、搜索索引、语义检索、会话式 AI、跨服务同步与故障补偿。

## 亮点

- 支持代码提交、编译、沙箱执行与判题结果生成的在线判题系统
- 基于 Redis 的判题、AI 分析、学习计划和异步数据同步后台
- 题目、标签、测试用例与后台管理 CRUD，以及现有缓存失效
- 基于 Elasticsearch 的标题、描述和标签全文检索
- 基于 Qdrant 的题目向量检索和长期学习记忆
- 支持会话上下文、短期摘要压缩和长期记忆召回的 AI 学习助手
- 题目变更到 Elasticsearch、RAG 知识库和排行榜的可重试最终一致性同步
- WebSocket 推送判题结果，SSE 推送 AI 对话轮次状态

## 系统概览

项目由三层运行组件组成：

1. Go 后端
   - 管理用户、题目、标签、测试用例、提交、排行榜、聊天会话和学习计划任务
   - 提供公开 API、管理端 API、WebSocket 通知和 Python Agent 内部工具 API
   - 运行判题、分析、学习计划及异步同步 Worker

2. Python Agent 服务
   - 使用 FastAPI、LangChain 和 DeepSeek 提供 AI 能力
   - 处理学习计划对话、会话摘要、题目向量同步和语义检索
   - 使用 Qdrant 存储题目向量和用户长期记忆

3. 前端应用
   - 基于 Vue 3 和 Vite
   - 覆盖 OJ 做题、后台管理、排行榜、提交记录和 AI 学习工作区

Docker Compose 提供以下基础设施：

- Redis：队列、旁路缓存和异步同步任务
- Elasticsearch：关键词检索
- Kibana：Elasticsearch 查询与索引检查
- Qdrant：向量数据库
- Agent：Python AI 服务

MySQL 是主关系型数据库，Go 后端启动时会通过 GORM AutoMigrate 初始化表结构。

## 架构

~~~text
Vue 3 前端
    |
    v
Go API Server (Gin)
    |- MySQL
    |- Redis
    |- Elasticsearch
    |- Docker Engine（判题沙箱）
    |
    +--> 判题 Worker Pool
    +--> AI 分析 Worker Pool
    +--> 学习计划 Worker Pool
    +--> 聊天轮次 Worker Pool
    +--> 数据同步 Worker Pool
              |
              v
       Python Agent（FastAPI + LangChain）
              |
              +--> DeepSeek LLM
              +--> DashScope Embeddings
              +--> Qdrant
~~~

## 核心流程

### 1. 提交与判题

判题采用异步队列流程：

1. 用户通过 POST /api/submit 提交代码。
2. 后端创建提交记录，并将任务推入 Redis。
3. 判题 Worker 消费队列，调用 internal/judge/service。
4. 服务在 Docker 沙箱内编译用户代码。
5. 测试用例在受限制的运行容器中逐个执行。
6. 系统比较输出并将最终结果写回 MySQL。
7. 结果通过 WebSocket 推送到前端。

支持的结果包括 AC、WA、RE、TLE、MLE、CE 和 SE。

判题完成后还会失效题目缓存，并异步同步题目统计数据和用户排行榜分数。

### 2. 题目搜索与检索

项目采用两层检索模型：

1. 关键词检索
   - 由 Elasticsearch 实现
   - 支持标题、描述匹配和标签过滤
   - 直接服务于题目搜索 API

2. 语义检索
   - 由 DashScope Embedding 和 Qdrant 实现
   - Python Agent 根据用户意图而非关键词查找相关题目

3. 混合检索
   - 在 Agent 层实现
   - 扩展和标准化查询，合并关键词与语义候选，再进行重排序
   - 向量化失败时回退到关键词检索，不会导致整个对话失败

### 3. 对话式 AI 学习助手

学习助手是会话式系统，而不是一次性问答：

1. 前端通过 POST /api/study-plan/sessions 创建聊天会话。
2. 每条用户消息都会在 MySQL 创建待处理聊天轮次，并写入 Redis 队列。
3. 聊天 Worker 读取轮次、构建上下文并调用 Python Agent。
4. Agent 可使用用户 AC 历史、失败提交历史、标签统计和规则、语义、混合候选题目等工具。
5. Worker 保存回复，并通过 SSE 推送轮次状态。

该设计同时支持推荐下一道适合练习的题目，以及在同一会话中进行普通算法问答。

### 4. 短期与长期记忆

短期记忆：

- 存储在 MySQL 聊天表中
- 最近消息保留在活动窗口
- 较早消息归档并压缩为会话摘要
- 当前 Worker 窗口保留最近 8 条消息

长期记忆：

- 存储在 Qdrant 集合 study_plan_memories
- 在成功轮次后保存稳定的用户学习上下文
- 在下次调用 LLM 前召回，作为辅助上下文

### 5. 题目知识与排行榜同步

MySQL 是题目、统计数据和用户解题数的唯一事实来源。业务层修改成功后，会先失效当前 Redis 缓存，再投递异步同步任务。

~~~text
MySQL 业务提交
   ↓
sync:pending
   ↓
ES / RAG / Leaderboard Handler
   ├─ 成功：确认任务
   └─ 失败：延迟重试，超限后进入死信队列
~~~

同步任务使用以下 Redis 键：

- sync:pending：待执行任务
- sync:processing：已被 Worker 领取的任务
- sync:processing:leases：任务租约
- sync:retry_at：按时间调度的重试任务
- sync:dead_letter：超过重试次数的任务

覆盖的业务事件包括：

- 题目创建、更新、删除和标签更新
- 测试用例新增和删除
- 标签删除导致的题目标签关系变化
- 判题完成后的提交数、通过数和 RAG 统计更新
- 用户首次 AC 后的排行榜同步

同步操作按最终状态写入，具备幂等性：

- Elasticsearch 与 RAG 使用题目 ID 覆盖写或删除
- 排行榜按 MySQL 中 solved_count * 10 设置 ZSet 分数，而非重复累加
- Worker 崩溃后的过期租约会将任务重新投递
- 失败任务按退避策略重试，超过上限后进入死信队列
- 服务启动和每 30 分钟会执行题目与排行榜全量校准

## 技术栈

### 后端

- Go 1.26
- Gin
- GORM + MySQL
- Redis
- Docker SDK for Go
- Elasticsearch v8 Client
- JWT
- WebSocket + SSE

### AI 与检索

- FastAPI
- LangChain
- DeepSeek Chat Model
- DashScope Embedding Model
- Qdrant

### 前端

- Vue 3
- Vue Router
- Axios
- Vite

## 仓库结构

~~~text
cmd/server/                  Go 应用入口
cmd/seed_problems/          题目种子数据命令
config/                      YAML 配置与配置加载器
infrastructure/              MySQL、Redis、Elasticsearch、WebSocket 配置
internal/app/                路由注册与中间件
internal/problem/            题目、标签、测试用例、搜索模块
internal/submission/         提交创建与查询
internal/judge/              沙箱、Docker 集成、判题服务与 Worker
internal/analysis/           AI 错误提交分析流程
internal/study_plan/         任务模式与聊天模式学习计划领域
internal/syncer/             ES、RAG 与排行榜异步同步后台
pkg/                         response、jwt、ecode、ai 等共享包
agent/                       Python FastAPI Agent 服务
agent/rag/                   向量索引、检索与记忆工具
vue/                         Vue 前端应用
docker-compose.yml           本地基础设施与 Agent 运行定义
~~~

## 核心领域模型

关系型数据主要围绕以下实体：

- users
- problems
- tags
- testcases
- submissions
- analysis_tasks 和 analysis_feedback
- study_plan_tasks 和 study_plan_feedback
- chat_sessions、chat_messages 和 chat_turns

这些表会在后端启动时由 AutoMigrate 自动创建或迁移。

## 本地开发

### 1. 前置条件

请先安装：

- Go 1.26+
- Node.js 18+
- Docker Desktop
- MySQL 8+

后端依赖可用的 Docker Engine，因为用户代码的编译和执行均在容器中完成。

### 2. 创建数据库

创建一个空的 MySQL 数据库，例如：

~~~sql
CREATE DATABASE w0roj CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
~~~

不需要手动建表。后端启动时会自动迁移数据库结构。

### 3. 准备后端配置

复制示例配置：

~~~powershell
Copy-Item config\config.example.yaml config\config.dev.yaml
~~~

设置 APP_ENV=dev，然后在 config/config.dev.yaml 中填写本地配置。

通常需要确认：

- sql.dsn
- redis.addr
- jwt.secret
- ai.api_key
- study_plan.agent_base_url
- elasticsearch.addresses

### 4. 准备 Docker Compose 根目录 .env

在仓库根目录创建或更新 .env。该文件主要由 Python Agent 容器读取。

必填项：

~~~env
DEEPSEEK_API_KEY=your_deepseek_api_key
DEEPSEEK_API_BASE=https://api.deepseek.com
LLM_MODEL=deepseek-v4-pro
GO_BACKEND_BASE_URL=http://host.docker.internal:8080
DASHSCOPE_API_KEY=your_dashscope_api_key
EMBEDDING_MODEL=text-embedding-v3
EMBEDDING_DIMENSION=1024
QDRANT_URL=http://qdrant:6333
MEMORY_COLLECTION=study_plan_memories
MEMORY_TOP_K=3
AGENT_DEBUG=true
~~~

可选项：

~~~env
EXPORT_API_TOKEN=
~~~

EXPORT_API_TOKEN 仅供部分独立导出脚本使用，不属于正常在线运行链路。

### 5. 启动基础设施和 Agent

~~~powershell
docker compose up -d --build
~~~

默认端口：

- 8000：Python Agent
- 6379：Redis
- 9200：Elasticsearch
- 5601：Kibana
- 6333：Qdrant HTTP
- 6334：Qdrant gRPC

如果本机端口 5601 已被占用，可在 docker-compose.yml 中修改宿主机映射，例如改成 5602:5601。

### 6. 启动 Go 后端

~~~powershell
$env:APP_ENV="dev"
go run .\cmd\server\main.go
~~~

API 服务默认监听 8080 端口。

### 7. 启动前端

~~~powershell
cd vue
npm install
npm run dev
~~~

前端以 /api 作为基础路径，并要求本地可访问 Go 后端。

## 搜索、RAG 和重建操作

### 增量同步

正常的管理端操作会自动触发增量同步：

- MySQL 始终是唯一事实来源
- 题目变更会失效相关 Redis 缓存
- Elasticsearch 与 Qdrant/RAG 通过异步任务最终追平 MySQL
- 同步失败会按退避策略自动重试

### 全量重建向量索引

如需使用当前数据库快照重建向量索引：

~~~powershell
docker compose run --rm agent python -m rag.index_problem_docs
~~~

该命令遍历全部题目并将向量写入 Qdrant。

### 搜索示例

提供了一个简单的语义检索演示：

~~~powershell
docker compose run --rm agent python -m rag.search_demo "two sum"
~~~

题目向量以 problem_id 作为 Qdrant Point ID，因此全量重建采用 upsert 语义。重复执行会更新已有题目，而不会创建同一题目的重复向量。

## 认证与内部信任模型

项目使用 JWT 进行用户认证：

- 登录成功后签发 Access Token 和 Refresh Token
- 受保护 API 使用 Bearer Access Token
- 部分流式接口支持 ?token=，方便浏览器使用 SSE 或 WebSocket
- 面向 Agent 的内部 API 仅用于服务间调用，不应暴露给公共客户端

执行学习计划时，Go Worker 会动态生成管理员 JWT 再调用 Python Agent，而不是在所有运行时请求中依赖固定且长期有效的 Token。

## 重要运行说明

- Redis 不是可选依赖。Redis 不可用时后端会快速失败。
- 提交代码前必须确认 Docker Desktop 正常运行，否则编译或执行沙箱容器会失败。
- Python Agent 需要能够访问 DeepSeek 和 DashScope。
- Qdrant 或 Embedding 不可用时，语义检索会退化到关键词检索；但推荐质量依赖 Qdrant 和 Embedding。
- 不建议直接手动修改或删除 MySQL 中的题目数据，应通过业务 API 保证缓存、搜索索引和向量数据同步。
- 可通过检查 sync:dead_letter 定位超过重试上限的同步任务。

## API 概览

### 公开 API

- 认证：注册、登录、刷新 Token、退出登录
- 题目浏览：列表、详情、标签、排行榜、搜索
- 用户自助：个人资料、我的提交
- 提交：提交代码、查询提交结果、WebSocket 更新

### 受保护的用户 API

- AI 分析任务创建与结果查询
- 学习计划任务模式 API
- 对话式学习助手的聊天会话 API
- AI 回复的 SSE 轮次流

### 管理员 API

- 用户封禁与解封
- 题目 CRUD
- 测试用例 CRUD
- 标签 CRUD
- 题目标签关系更新
- 分析统计
- 学习计划统计

### Agent 内部工具 API

- 用户 AC 历史
- 失败提交记录
- 标签统计
- 候选题目检索
- 题目详情查询

## 面向生产的特征

这个仓库比课堂演示更接近真实系统，不在于单一框架，而在于运行时设计：

- 异步 Worker 将慢任务与请求延迟隔离
- 判题执行在 Docker 沙箱中运行
- 检索分为关键词检索与语义检索
- AI 使用有状态、会话式交互，而非单次 Prompt 输入输出
- 记忆分为短期压缩和长期召回
- 业务数据通过统一同步后台协调到缓存、搜索、RAG 与排行榜
- 失败同步支持租约恢复、重试、死信与周期性校准

## 后续扩展方向

当前代码库已经具备进一步演进的基础，例如：

- 更丰富的工具路由和多 Agent 学习引导
- 除按题目文档外，更细粒度的题目知识分块策略
- 结合难度、通过率和用户薄弱标签等元数据的混合重排序
- 结构化长期记忆提取，而非仅保存文本记忆
- 增强 Agent 决策、队列延迟、死信任务和检索质量的可观测性
- 使用 MySQL Outbox 或删除墓碑，进一步覆盖 Redis 投递失败时的删除同步边界

## License

仓库当前未包含 License 文件。如计划公开发布，请在对外发布前添加明确的开源许可证。
