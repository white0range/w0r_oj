# Gojo OJ Demo

一个面向后端练习的 OJ 小项目，包含用户注册登录、题目管理、代码提交、异步判题、排行榜、标签筛选、搜索等功能。

## 技术栈

后端：
- Go 1.26
- Gin
- Gorm
- MySQL
- Redis
- Elasticsearch

前端：
- Vue 3
- Vite
- Axios
- Vue Router

## 项目结构

```text
gojo/
├── cmd/server/               # 后端启动入口
├── config/                   # 配置文件
├── infrastructure/           # MySQL / Redis / ES / WebSocket 等基础设施
├── internal/                 # 核心业务模块
│   ├── user/
│   ├── problem/
│   ├── submission/
│   ├── judge/
│   └── leaderboard/
├── middlewares/              # 鉴权、限流、权限控制
├── pkg/                      # JWT、密码、分页等通用组件
├── vue/                      # 前端项目
└── docker-compose.yml        # Redis / Elasticsearch / Kibana
```

## 已实现功能

- 用户注册 / 登录
- JWT 鉴权
- 题目列表 / 题目详情
- 标签管理
- 代码提交
- 异步判题
- 排行榜
- Elasticsearch 搜索
- WebSocket / SSE 实时返回部分结果
- 管理员题目管理

## 运行环境

请先准备以下环境：

- Go 1.26
- Node.js
- MySQL
- Docker Desktop

说明：
- MySQL 使用本地安装，不在 Docker 中
- Redis / Elasticsearch / Kibana 使用 `docker-compose.yml` 启动

## 配置文件

复制示例配置文件：

```bash
copy config\config.example.yaml config\config.yaml
```

然后修改 `config/config.yaml`：

```yaml
sql:
  dsn: "root:123456@tcp(127.0.0.1:3306)/gin_demo?charset=utf8mb4&parseTime=True&loc=Local"

redis:
  addr: "localhost:6379"
  password: ""
  db: 0

jwt:
  secret: "your_jwt_secret"

ai:
  api_key: "your_api_key"
```

## 启动步骤

### 1. 启动本地 MySQL

确保本地 MySQL 已启动，并提前创建数据库：

```sql
CREATE DATABASE gin_demo CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
```

### 2. 启动 Redis / Elasticsearch / Kibana

在项目根目录执行：

```bash
docker compose up -d
```

默认端口：
- Redis: `6379`
- Elasticsearch: `9200`
- Kibana: `5601`

### 3. 启动后端

在项目根目录执行：

```bash
go run .\cmd\server\main.go
```

后端默认监听：
- `http://localhost:8080`

### 4. 启动前端

进入前端目录：

```bash
cd vue
npm install
npm run dev
```

Vite 默认地址一般为：
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

## 开发说明

- 项目采用 `handler / service / repository` 分层
- 判题任务通过 Redis 队列异步处理
- 搜索功能基于 Elasticsearch
- 部分功能依赖本地 Docker 环境
- `config/config.yaml` 已加入 `.gitignore`，不会提交到仓库

## 常见问题

### 1. 后端启动失败，提示连不上 MySQL

请检查：
- 本地 MySQL 是否已启动
- `gin_demo` 数据库是否已创建
- `config/config.yaml` 中的 DSN 是否正确

### 2. Redis / Elasticsearch 连不上

请先执行：

```bash
docker compose up -d
```

再检查对应端口是否被占用。

### 3. 前端打不开接口

请确认：
- 后端已运行在 `8080`
- 前端请求地址配置正确

## 后续计划

- 补充自动化测试
- 完善 README 和部署说明
- 优化判题失败补偿逻辑
- 增加重判功能（rejudge）
- 增加 Swagger / OpenAPI 文档
