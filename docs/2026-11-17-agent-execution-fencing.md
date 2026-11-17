# Agent 执行隔离与记忆幂等

日期：2026-11-17

## 执行隔离

Go Worker 领取回合时原本就会生成 ProcessingToken。现在将它随 `turn_id`、`user_id` 传给 Python；工具请求在固定 `X-Agent-Service-Token` 之外携带 `X-Agent-Turn-ID`、`X-Agent-Processing-Token`。Go 校验回合状态、租约、会话归属和当前 ProcessingToken，重领后的旧执行会收到 HTTP 403，业务码 `40302`。

Python 将其识别为 `AgentExecutionExpired`，直接结束整轮执行，对 Go 返回 HTTP 409。这个错误不会转成模型可重试的工具结果。Graph 在模型调用前后检查执行状态，所有工具（含缓存命中、语义检索）执行前也检查；读取用户记忆前额外校验身份。执行编号保存在闭包中，不进入模型工具参数、提示词或普通日志。Go 也不再将可能回显请求字段的 Python 错误响应正文写入日志。

最终回答继续通过原有 `CompleteClaimedTurn` 的 ProcessingToken 条件更新，旧执行不能覆盖当前结果。已经发出的模型或检索请求可能无法立即取消，但在下一处校验时会停止旧执行。

## 记忆同步

1. Python 只返回回答，移除 Graph 执行结束时直接保存记忆的逻辑。
2. Go 在同一数据库事务中确认成功结果、保存助手消息，并将 `chat_turns.memory_sync_pending` 设为 true。
3. 事务成功后投递 `chat_memory/chat_memory_sync` 任务，复用 `sync:pending` 等现有同步队列，任务只携带回合 ID。
4. Go 在启动时及每 30 秒扫描待同步回合，补偿提交后退出、Redis 入队失败或 Redis 数据丢失的情况。
5. Redis 使用 `sync:chat_memory:dispatch:<turn_id>` 原子去重；分发标记覆盖排队、处理和重试，补偿扫描不会绕过退避。
6. 消费者从 MySQL 读取已经确认成功的结果、原始用户消息和完成时间，调用 Python `/memory/sync`。已归档会话跳过同步。
7. Python 用用户 ID、回合 ID、记忆类型生成稳定 UUID，执行 Qdrant upsert 并等待写入确认；重试使用同一 ID 和原完成时间。
8. Python 成功后才清除数据库待同步标记，再确认 Redis 任务。Qdrant 已写入但响应或数据库确认失败时可安全重试，不会产生另一份记忆。

## 数据库与发布

没有新增表。`chat_turns` 增加一个 `memory_sync_pending` 布尔字段（默认 false、建立索引），由现有 AutoMigrate 在 Go API 启动时创建。历史回合保持 false，不触发历史向量回填。Go、Python 契约同时改变，发布时应同步更新两个服务。固定服务密钥继续使用，不新增 JWT。

## 死信处理

记忆任务沿用同步队列的退避、租约恢复及 8 次重试上限。耗尽后保留死信及分发标记，避免扫描无限发起新一轮重试。

修复故障后，显式重放应原子地将选中的原始任务从 `sync:dead_letter` 移回 `sync:pending`，重置 `retry_count` 和 `last_error`，保留其回合分发标记。消费者始终从 MySQL 加载同一份已确认数据。不要清空整个死信队列，也不要仅删除分发标记而留下旧死信。

## 验证

- Go 全量测试：路由、鉴权、Worker 请求及现有业务回归。
- Worker 测试：只有确认成功的执行投递记忆；丢失执行不投递；入队失败不把成功回答改成失败。
- Python 测试：真实 Graph 使用测试模型，验证旧执行会停止且不再调用模型；错误响应不包含执行编号；未确认回答不保存记忆。
- MySQL 集成测试：临时独立数据库中模拟租约过期、重新领取，旧执行完成被拒绝，重复完成只生成一条助手消息，待同步标记与成功结果一起提交。
- Redis 集成测试：独立临时 Redis 验证重复分发去重、重试退避保留、确认清理标记和死信不自动重新入队。
- Qdrant 集成测试：独立临时集合、固定测试向量，重复保存同一回合只保留一个点，测试后删除该集合。

Qdrant 集成测试使用固定测试向量，不调用外部模型服务；它验证重复写入的存储幂等性，不验证外部向量服务的额度或可用性。
