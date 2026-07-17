# Gojo OJ

Gojo OJ 是一个 Go + Vue + FastAPI 构建的在线判题与 AI 学习助手项目。

## 功能

- Docker 沙箱编译、运行和判题
- 题目、标签、测试用例、提交记录和排行榜
- Redis 旁路缓存与 Redis ZSet 排行榜
- Elasticsearch 全文搜索与 Qdrant RAG 题目检索
- Redis 异步同步队列，支持重试、死信和定期全量校准
- AI Chat 会话、上下文压缩和长期记忆

## Chat 架构

学习助手只保留 Chat 模式：

1. 用户创建 `/api/chat/sessions` 并发送消息。
2. 后端创建 `chat_turns` 记录，将回合推入 Redis `chat_turn_queue`。
3. Chat Worker 调用 FastAPI 的 `/chat/run`，并把结构化结果保存到 `ChatTurn.Result`。
4. 展示用消息保存在 `chat_messages`，原始结构化 JSON 保存在 `structured_payload`。
5. 用户可对完成的回合提交 `ChatPlanFeedback`：`POST /api/chat/turns/:turn_id/feedback`。

独立的 `StudyPlanTask`、任务反馈、任务队列和对应 API 已删除。旧空表清理由 [20260718_remove_legacy_study_plan_tables.sql](migrations/20260718_remove_legacy_study_plan_tables.sql) 提供，需人工确认无数据后执行。

## 同步机制

数据库提交后将 ES、RAG 和排行榜同步任务写入 Redis。消费者使用租约、指数退避重试和死信队列处理失败任务，并定期从 MySQL 进行全量校准。

## 配置

复制 `config/config.example.yaml` 为对应环境配置文件，并填写 MySQL、Redis、JWT、AI、Elasticsearch 与 Chat Agent 配置。

```yaml
chat:
  worker_count: 3
  agent_base_url: "http://localhost:8000"
  agent_timeout_seconds: 60
```

## 验证

```bash
go test ./...
```