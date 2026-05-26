# Gojo OJ Demo

一个以在线判题系统（OJ）为主题的后端练手项目。项目包含用户注册登录、题目管理、代码提交、异步判题、排行榜、标签筛选、搜索，以及 AI 辅助分析等功能。

这个仓库的目标不是做一个完整商业化平台，而是尽量用一个中等复杂度的题目，把常见后端能力串起来，例如：

- 分层设计：`handler / service / repository`
- 鉴权与权限控制：JWT、管理员接口
- 外部依赖接入：MySQL、Redis、Elasticsearch、Docker
- 异步任务：提交入队、后台 worker 判题
- 工程化整理：统一响应结构、错误码、环境配置

如果你是面试官或同学，建议优先看：

- [项目结构](#项目结构)
- [核心功能](#核心功能)
- [本地启动](#本地启动)
- [后端设计说明](#后端设计说明)

## 技术栈

后端：

- Go
- Gin
- Gorm
- MySQL
- Redis
- Elasticsearch
- Docker SDK

前端：

- Vue 3
- Vite
- Axios
- Vue Router

## 项目结构

```text
gojo/
├── cmd/server/                    # 后端启动入口
├── config/                        # 配置文件与配置加载逻辑
├── infrastructure/                # MySQL / Redis / ES / WebSocket 等基础设施接入
├── internal/
│   ├── app/                       # 路由装配、统一响应、错误码、中间件
│   ├── judge/                     # 判题相关逻辑
│   ├── leaderboard/               # 排行榜模块
│   ├── problem/                   # 题目、标签、测试用例、搜索
│   ├── submission/                # 提交记录与 AI 辅助
│   └── user/                      # 用户模块
├── pkg/                           # JWT、密码、AI client 等通用组件
├── vue/                           # 前端项目
└── docker-compose.yml             # Redis / Elasticsearch / Kibana
```

## 核心功能

- 用户注册、登录、个人信息查询
- JWT 鉴权
- 管理员题目创建、修改、删除
- 标签管理
- 测试用例管理
- 代码提交
- Redis 队列异步判题
- 提交结果查询
- Elasticsearch 搜索题目
- 排行榜
- AI 辅助分析提交结果

## 后端设计说明

### 1. 分层结构

项目整体采用 `handler / service / repository` 分层：

- `handler`：处理 HTTP 请求、参数绑定、返回统一响应
- `service`：承载业务逻辑
- `repository`：访问数据库、Redis、Elasticsearch 等外部依赖

这样做的好处是：

- handler 不直接写 SQL
- service 不直接依赖 Gin
- repo 不关心 HTTP 细节

### 2. 统一响应与错误处理

项目已经做了第一版规范化：

- 统一响应结构放在 `internal/app/response`
- 统一错误码放在 `internal/app/ecode`
- 常见业务错误放在 `internal/app/apperror`

接口成功时统一返回：

```json
{
  "code": 0,
  "message": "ok",
  "data": {}
}
```

接口失败时统一返回：

```json
{
  "code": 40401,
  "message": "problem not found",
  "data": null
}
```

### 3. 异步判题链路

提交代码后，主流程大致如下：

1. 创建 submission 记录
2. 将判题任务推入 Redis 队列
3. 后台 worker 消费队列
4. 调用 Docker 判题
5. 回写 submission 状态与结果
6. 更新相关统计信息

这部分是整个项目里相对更有后端特点的一条链路。

### 4. 配置管理

项目已经开始按环境管理配置：

- `config/config.example.yaml`：配置模板，提交到仓库
- `config/config.dev.yaml`：本地开发配置，不提交
- 后续可以继续扩展：
  - `config/config.test.yaml`
  - `config/config.prod.yaml`

程序启动时会读取 `APP_ENV`：

- `APP_ENV=dev` -> 读取 `config/config.dev.yaml`
- `APP_ENV=test` -> 读取 `config/config.test.yaml`
- `APP_ENV=prod` -> 读取 `config/config.prod.yaml`

如果没有设置 `APP_ENV`，默认使用 `dev`。

此外，也支持环境变量覆盖配置。例如：

```powershell
$env:GOJO_SERVER_PORT="9090"
```

表示临时覆盖 `server.port`。

## 本地启动

### 1. 环境准备

请先准备：

- Go
- Node.js
- MySQL
- Docker Desktop

说明：

- MySQL 使用本地安装，不在 Docker 中
- Redis / Elasticsearch / Kibana 通过 `docker-compose.yml` 启动

### 2. 创建数据库

确保本地 MySQL 已启动，然后创建数据库：

```sql
CREATE DATABASE gin_demo CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
```

### 3. 准备配置文件

先复制模板：

```powershell
Copy-Item config\config.example.yaml config\config.dev.yaml
```

然后修改 `config/config.dev.yaml` 中的关键配置：

- `sql.dsn`
- `jwt.secret`
- `ai.api_key`

一个开发环境示例：

```yaml
app:
  env: dev

server:
  port: 8080
  read_timeout_seconds: 10
  write_timeout_seconds: 10
  idle_timeout_seconds: 60

sql:
  dsn: "root:123456@tcp(127.0.0.1:3306)/gin_demo?charset=utf8mb4&parseTime=True&loc=Local"
  max_open_conns: 20
  max_idle_conns: 10
  conn_max_lifetime_seconds: 3600

redis:
  addr: "localhost:6379"
  password: ""
  db: 0

jwt:
  secret: "replace_with_your_secret"

ai:
  api_key: "replace_with_your_api_key"
  base_url: "https://api.deepseek.com"
  model: "deepseek-chat"
  timeout_seconds: 30

elasticsearch:
  addresses:
    - "http://localhost:9200"

judge:
  worker_count: 3
```

### 4. 启动 Redis / Elasticsearch / Kibana

在项目根目录执行：

```powershell
docker compose up -d
```

默认端口：

- Redis: `6379`
- Elasticsearch: `9200`
- Kibana: `5601`

### 5. 启动后端

在项目根目录执行：

```powershell
$env:APP_ENV="dev"
go run .\cmd\server\main.go
```

如果你没有显式设置 `APP_ENV`，程序也会默认走 `dev`。

默认访问地址通常为：

- `http://localhost:8080`

如果你在配置里修改了端口，就按配置中的端口访问。

### 6. 启动前端

进入前端目录：

```powershell
cd vue
npm install
npm run dev
```

Vite 默认地址通常为：

- `http://localhost:5173`

## 主要接口

公共接口：

- `POST /api/register`
- `POST /api/login`
- `GET /api/problems`
- `GET /api/problems/:id`
- `GET /api/tags`
- `GET /api/leaderboard`
- `POST /api/problems/search`

登录后接口：

- `GET /api/profile`
- `POST /api/submit`
- `GET /api/submissions/:id`
- `GET /api/my-submissions`
- `GET /api/submissions/:id/ai-help`
- `GET /api/ws`

管理员接口：

- `POST /api/admin/problems`
- `PUT /api/admin/problems/:id`
- `DELETE /api/admin/problems/:id`
- `POST /api/admin/tags`
- `DELETE /api/admin/tags/:id`

## 常见问题

### 1. 后端启动失败，提示找不到配置文件

请检查：

- 是否已经创建 `config/config.dev.yaml`
- 是否设置了正确的 `APP_ENV`
- 对应环境的配置文件是否真的存在

例如，如果设置了：

```powershell
$env:APP_ENV="prod"
```

那程序就会去读：

```text
config/config.prod.yaml
```

### 2. 后端启动失败，提示连不上 MySQL

请检查：

- 本地 MySQL 是否已启动
- `gin_demo` 数据库是否已创建
- `config/config.dev.yaml` 中的 `sql.dsn` 是否正确

### 3. Redis / Elasticsearch 连接失败

请确认已经执行：

```powershell
docker compose up -d
```

并检查对应端口是否被占用。

### 4. AI 功能不可用

请检查：

- `ai.api_key` 是否填写正确
- `ai.base_url` 是否正确
- `ai.model` 是否是当前可用模型

## 当前完成度与后续计划

当前这个项目已经完成了后端核心主链路，并做了第一版工程化整理，但还可以继续完善：

- 补充自动化测试
- 进一步完善配置分环境方案
- 优化异步判题失败补偿逻辑
- 增加 rejudge 功能
- 增加 Swagger / OpenAPI 文档
- 补充部署说明和 CI

## 说明

- `config/config.example.yaml` 会提交到仓库
- `config/config.dev.yaml`、`config/config.test.yaml`、`config/config.prod.yaml` 不会提交
- 仓库中的配置模板不会包含真实密钥

如果你准备将本仓库链接放到简历里，建议你在提交前再确认：

- `README` 与当前代码一致
- 没有把真实配置文件提交到 Git
- 后端能按文档步骤启动
