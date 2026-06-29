# Gojo OJ

Gojo OJ is a full-stack algorithm training platform that combines online judging, asynchronous evaluation, search and retrieval, and AI-assisted study workflows into a single system. The repository is organized around a Go backend as the source of truth, a Vue frontend for user interaction, and a Python agent service for conversational guidance, retrieval augmentation, and memory.

Unlike a toy OJ that only stores problems and returns verdicts, this project already covers the core runtime concerns of a real system: queue-based judging, cache invalidation, search indexing, semantic retrieval, session-oriented AI interaction, and cross-service coordination.

## Highlights

- Online Judge for code submission, compilation, sandboxed execution, and verdict generation
- Redis-backed asynchronous workers for judging, AI analysis, and study-plan turns
- Problem management with tags, test cases, admin CRUD, and cache invalidation
- Elasticsearch-based lexical search for title, description, and tag filtering
- Qdrant-based vector retrieval for problem recommendations and long-term memory
- Conversational AI study assistant with session context, short-term summary compression, and long-term memory recall
- Incremental synchronization from problem CRUD to both search index and RAG knowledge base
- WebSocket and SSE based result delivery for submissions and AI turn streaming

## System Overview

The project is split into three runtime layers:

1. Go backend
   - Owns users, problems, tags, test cases, submissions, ranking, chat sessions, and study-plan tasks
   - Exposes public APIs, admin APIs, WebSocket notifications, and internal tool APIs for the Python agent
   - Runs worker pools for judge, analysis, and study-plan processing

2. Python agent service
   - Provides the AI execution layer through FastAPI + LangChain + DeepSeek
   - Handles conversational study-plan turns, session summarization, problem vector sync, and semantic retrieval
   - Uses Qdrant for both problem embeddings and user long-term memory

3. Frontend application
   - Built with Vue 3 + Vite
   - Covers OJ usage, admin console, ranking, submissions, and the AI study workspace

Supporting infrastructure is provided through Docker Compose:

- Redis: queues and cache
- Elasticsearch: lexical retrieval
- Kibana: search inspection
- Qdrant: vector database
- Agent: Python AI service

MySQL is the primary relational store and is initialized by GORM auto-migration during backend startup.

## Architecture

```text
Vue 3 Frontend
    |
    v
Go API Server (Gin)
    |- MySQL
    |- Redis
    |- Elasticsearch
    |- Docker Engine (judge sandbox)
    |
    +--> Judge Worker Pool
    +--> Analysis Worker Pool
    +--> Study Plan Worker Pool
    +--> Chat Turn Worker Pool
              |
              v
       Python Agent (FastAPI + LangChain)
              |
              +--> DeepSeek LLM
              +--> DashScope Embeddings
              +--> Qdrant
```

## Core Workflows

### 1. Submission and Judging

The judging pipeline is queue-based and asynchronous.

1. The user submits code through `POST /api/submit`.
2. The backend creates a submission record and pushes a task into Redis.
3. Judge workers consume the queue and invoke `internal/judge/service`.
4. The service compiles user code inside a Docker-based sandbox.
5. Test cases are executed one by one in an isolated runtime container.
6. The system compares outputs and writes the final verdict back to MySQL.
7. Submission updates are pushed to the frontend through WebSocket.

Supported verdicts include `AC`, `WA`, `RE`, `TLE`, `MLE`, `CE`, and `SE`.

### 2. Problem Search and Retrieval

The project uses a two-layer retrieval model.

1. Lexical retrieval
   - Implemented with Elasticsearch
   - Supports title and description matching, plus tag filtering
   - Used directly by the problem search API

2. Semantic retrieval
   - Implemented with DashScope embeddings + Qdrant
   - Used by the Python agent when looking for related problems by intent rather than exact wording

3. Hybrid retrieval
   - Implemented in the agent layer
   - Expands and normalizes the user query, merges lexical and semantic candidates, and reranks them
   - If semantic embedding fails, the agent falls back to lexical retrieval instead of failing the whole turn

### 3. Conversational AI Study Assistant

The current study assistant is session-based rather than one-shot.

1. The frontend creates a chat session with `POST /api/study-plan/sessions`.
2. Each user message creates a pending chat turn in MySQL and enqueues a Redis task.
3. Chat turn workers fetch the turn, prepare context, and call the Python agent.
4. The agent decides whether to use tools such as:
   - user AC history
   - failed submission history
   - tag statistics
   - rule-based candidate problems
   - semantic candidate problems
   - hybrid candidate problems
5. The worker stores the assistant reply and streams turn status via SSE.

This gives the system two capabilities at the same time:

- practical recommendation of next problems to solve
- ordinary conversation-style algorithm Q&A in the same session

### 4. Short-Term and Long-Term Memory

The AI workflow uses two memory layers.

Short-term memory:
- Stored in MySQL chat tables
- Recent messages are kept in an active window
- Older messages are archived and merged into a session summary
- The current worker window size is 8 recent messages

Long-term memory:
- Stored in Qdrant collection `study_plan_memories`
- Saves durable user-specific learning context after successful turns
- Recalled before LLM invocation as auxiliary context for future turns

### 5. Problem Knowledge Synchronization

Problem data is synchronized incrementally when admins change the source of truth in MySQL.

- Problem create/update/tag update triggers:
  - Redis cache invalidation
  - Elasticsearch document upsert
  - Python agent RAG sync endpoint call
- Problem delete triggers:
  - relational cleanup through backend business logic
  - Elasticsearch delete
  - Python agent vector delete

This design keeps search and vector knowledge aligned with the business layer instead of encouraging direct database edits.

## Tech Stack

### Backend

- Go 1.26
- Gin
- GORM + MySQL
- Redis
- Docker SDK for Go
- Elasticsearch v8 client
- JWT authentication
- WebSocket + SSE

### AI and Retrieval

- FastAPI
- LangChain
- DeepSeek chat model
- DashScope embedding model
- Qdrant vector database

### Frontend

- Vue 3
- Vue Router
- Axios
- Vite

## Repository Structure

```text
cmd/server/                  Go application entrypoint
config/                      YAML configuration files and config loader
infrastructure/              MySQL, Redis, Elasticsearch, WebSocket, Qdrant config
internal/app/                route registration and middleware
internal/problem/            problem, tag, testcase, search modules
internal/submission/         submission creation and query
internal/judge/              sandbox, Docker integration, judge service, workers
internal/analysis/           AI incorrect-submission analysis pipeline
internal/study_plan/         task mode + chat mode study-plan domain
pkg/                         shared packages such as response, jwt, ecode, ai
agent/                       Python FastAPI agent service
agent/rag/                   vector indexing, retrieval, and memory helpers
vue/                         Vue frontend application
docker-compose.yml           local infra and agent runtime definition
```

## Key Domain Models

The relational model is centered around the following entities:

- `users`
- `problems`
- `tags`
- `testcases`
- `submissions`
- `analysis_tasks` and `analysis_feedback`
- `study_plan_tasks` and `study_plan_feedback`
- `chat_sessions`, `chat_messages`, and `chat_turns`

These tables are created automatically by `AutoMigrate` during backend startup.

## Local Development

### 1. Prerequisites

Install the following locally:

- Go 1.26+
- Node.js 18+
- Docker Desktop
- MySQL 8+

The backend depends on a running Docker Engine because code compilation and execution are performed in containers.

### 2. Create the Database

Create an empty MySQL database, for example:

```sql
CREATE DATABASE w0roj CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

You do not need to create tables manually. The backend will auto-migrate schema on startup.

### 3. Prepare Backend Config

Copy the example file:

```powershell
Copy-Item config\config.example.yaml config\config.dev.yaml
```

Set `APP_ENV=dev`, then edit `config/config.dev.yaml` with your local values.

Typical fields you need to confirm:

- `sql.dsn`
- `redis.addr`
- `jwt.secret`
- `ai.api_key`
- `study_plan.agent_base_url`
- `elasticsearch.addresses`

### 4. Prepare Root `.env` for Docker Compose

Create or update the repository root `.env` file. This file is mainly consumed by the Python agent container.

Required fields:

```env
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
```

Optional field:

```env
EXPORT_API_TOKEN=
```

`EXPORT_API_TOKEN` is only needed for some standalone export scripts and is not part of the normal online runtime path.

### 5. Start Infrastructure and Agent

```powershell
docker compose up -d --build
```

Default ports:

- `8000`: Python agent
- `6379`: Redis
- `9200`: Elasticsearch
- `5601`: Kibana
- `6333`: Qdrant HTTP
- `6334`: Qdrant gRPC

If port `5601` is already occupied on your machine, adjust the host-side mapping in `docker-compose.yml`, for example `5602:5601`.

### 6. Start the Go Backend

```powershell
$env:APP_ENV="dev"
go run .\cmd\server\main.go
```

The API server listens on port `8080` by default.

### 7. Start the Frontend

```powershell
cd vue
npm install
npm run dev
```

The frontend uses `/api` as its base path and expects the Go backend to be reachable locally.

## Search, RAG, and Rebuild Operations

### Incremental Sync

Normal admin operations already trigger incremental synchronization automatically:

- MySQL remains the source of truth
- Elasticsearch is updated on problem create/update/delete
- Qdrant problem vectors are updated through agent sync endpoints

### Full Reindex

If you need to rebuild the vector index from the current database snapshot:

```powershell
docker compose run --rm agent python -m rag.index_problem_docs
```

This command performs a full pass over problem data and writes vectors into Qdrant.

### Search Demo

A small semantic retrieval demo is also available:

```powershell
docker compose run --rm agent python -m rag.search_demo "two sum"
```

Problem vectors are written to Qdrant with `problem_id` as the point ID, so full reindex uses upsert semantics. Running it again updates existing points instead of appending duplicate entries for the same problem.

## Authentication and Internal Trust Model

The project uses JWT for user authentication.

- Public login issues access token and refresh token
- Protected APIs use bearer access tokens
- Some streaming endpoints also accept `?token=` for browser-based SSE or WebSocket usage
- Agent-facing internal APIs are protected and intended for service-to-service calls, not public clients

For study-plan execution, the Go worker generates an admin JWT dynamically before calling the Python agent, instead of depending on a long-lived fixed token for every runtime request.

## Important Runtime Notes

- Redis is not optional. The backend will fail fast if Redis is unavailable.
- Docker Desktop must be healthy before submitting code, otherwise sandbox compilation or runtime containers will fail.
- The Python agent depends on outbound access to DeepSeek and DashScope.
- Semantic retrieval can degrade gracefully to lexical retrieval, but the best recommendation quality depends on Qdrant and embeddings being available.
- Direct manual deletion from MySQL is not recommended for problem data, because business-layer deletion also coordinates cache, search, and vector cleanup.

## API Surface Summary

### Public APIs

- auth: register, login, refresh, logout
- problem browsing: list, detail, tags, ranking, search
- user self-service: profile, my submissions
- submission: submit code, query submission result, WebSocket updates

### Protected User APIs

- AI analysis task creation and feedback
- study-plan task mode APIs
- chat session APIs for the conversational assistant
- SSE turn streaming for AI responses

### Admin APIs

- user ban and unban
- problem CRUD
- testcase CRUD
- tag CRUD
- problem-tag relation updates
- analysis statistics
- study-plan statistics

### Internal Agent Tool APIs

- user AC history
- failed submissions
- tag statistics
- candidate problem retrieval
- problem detail lookup

## Production-Oriented Characteristics

What makes this repository closer to an actual system than a classroom demo is not any single framework choice, but the shape of the runtime design:

- asynchronous workers isolate slow tasks from request latency
- judge execution is sandboxed and externalized to Docker
- retrieval is split between lexical search and semantic search
- AI interaction is session-based and stateful, not prompt-in prompt-out only
- memory is layered into short-term compaction and long-term recall
- data synchronization is coordinated through business services instead of ad hoc scripts

## Future Extension Directions

The current codebase is already a strong foundation for further evolution, for example:

- richer tool-routing and multi-agent orchestration for study guidance
- stronger chunking strategies for problem knowledge beyond the current per-problem document model
- hybrid reranking with additional metadata signals such as difficulty, acceptance rate, or user weakness tags
- structured long-term memory extraction instead of purely text-oriented memory writes
- more complete observability around agent decisions, queue latency, and retrieval quality

## License

No license file is currently included in this repository. If the project is intended for public distribution, add an explicit license before external release.

