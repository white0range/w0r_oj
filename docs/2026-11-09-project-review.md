# w0r_oj 项目审查与改进清单

审查日期为 2026-11-09，代码基线为 `main` 的 `0b00ebf`。范围包括提交判题、Agent 权限、题库体验、索引同步和服务启动。

## 优先处理判题与权限

### 1. 隔离隐藏用例的反馈

判题器把失败用例的实际输出写入 `actual_output`，提交详情再把它返回给提交者。如果程序回显标准输入，隐藏用例的内容就可能出现在提交详情里。AI 错题分析也读取这个字段。 [判题结果生成](../internal/judge/service/judge_service.go#L76) · [提交详情接口](../internal/submission/handler/submission_handler.go#L88) · [错题详情](../internal/chat/service/chat_service.go#L153)

- 改进建议：区分公开样例和隐藏用例。公开样例保留可读的输出对比，隐藏用例只向用户和 Agent 返回判定、耗时和内存等安全字段。对历史提交中的输出也应用相同的展示边界。
- 验收条件：用回显程序提交后，隐藏输入无法通过提交详情或 AI 错题分析读到，公开样例仍可用于调试。

### 2. 统一输出采集、存储和展示上限

沙箱允许采集约 1 MiB 输出，`actual_output` 使用的 MySQL `TEXT` 最多存储 65,535 字节。程序产生较长的错误输出时，判题可能已经完成，但结果更新失败。 [沙箱输出上限](../internal/judge/sandbox/sandbox.go#L57) · [提交模型](../internal/submission/model/submission.go#L24) · [MySQL 类型限制](https://dev.mysql.com/doc/refman/8.4/en/string-type-syntax.html)

- 改进建议：判题比较沿用沙箱的输出边界，在持久化前单独限制诊断文本的 UTF-8 字节数，并给截断内容加上明确标记。系统错误对用户只返回简短说明和可追踪编号。
- 验收条件：超过 `TEXT` 上限的错误输出和多字节文本均能写入终态，正常答案比较不受诊断截断影响。

### 3. 衔接提交回执和队列重试

编译容器创建、沙箱启动或执行异常会被判题服务直接写成 `SE`。写入成功后，Worker 会确认任务，现有重试逻辑无法处理这些暂时性故障。另一条路径上，MySQL 已保存提交、Redis 入队失败时，客户端收到提交失败，但后台仍可能补判。 [判题服务](../internal/judge/service/judge_service.go#L38) · [Worker 重试](../internal/judge/worker/judge_worker.go#L162) · [提交入队](../internal/submission/service/submission_service.go#L53)

- 改进建议：分类处理用户代码错误、题目配置错误和可重试的基础设施错误。暂时性故障交给 Worker 重试，重试耗尽后写入 `SE`。已经落库的提交返回原提交 ID 和待调度状态。
- 验收条件：模拟一次 Docker 调用失败后，任务重试并产生唯一终态。模拟 Redis 入队失败后，用户可以用返回的 ID 查询到最终判定。

### 4. 为 Agent 使用专用凭据

Chat Worker 会选取一个活跃管理员，并把该管理员的普通访问令牌交给 Agent。该令牌也适用于题目管理和用户封禁等管理接口。 [服务账号选择](../internal/chat/worker/chat_worker.go#L105) · [令牌签发](../internal/chat/worker/chat_worker.go#L162) · [管理路由](../internal/app/routes.go#L57)

- 改进建议：为内部工具签发短期服务凭据，限定可调用的只读接口，并在 Go 端校验凭据绑定的用户和回合。
- 验收条件：Agent 能读取授权用户的错题数据，使用同一凭据访问其他用户数据、修改题目或封禁用户时会被拒绝。

## 改善刷题体验和运行成本

### 5. 修复题库进度和实时判题状态

题库列表路由缺少可选鉴权，服务层拿不到登录用户的 ID，导致「本页已通过」无法正确计算。提交页和提交详情页在 SSE 连上后会停止轮询，若结果早于连接建立，页面可能停在 `Pending`。 [题库路由](../internal/app/routes.go#L48) · [通过状态计算](../internal/problem/service/problem_service.go#L150) · [提交页 SSE](https://github.com/white0range/w0r_oj/blob/0b00ebf/vue/src/views/ProblemDetail.vue#L247) · [详情页 SSE](https://github.com/white0range/w0r_oj/blob/0b00ebf/vue/src/views/SubmissionDetail.vue#L145)

- 改进建议：接入现有的可选鉴权，保留公共题库缓存，在返回前合并用户自己的通过状态。SSE 建立时主动查询一次提交，等待期间保留低频轮询作为兜底。
- 验收条件：已通过和未通过的两个用户看到各自的题库状态。判题先于 SSE 连接完成，或通知丢失时，页面仍会显示终态。

### 6. 把题库搜索扩展到全库

首页搜索只过滤当前页的数据。已有 Elasticsearch 搜索接口支持关键词和标签，但单次最多返回 50 条且没有分页参数。 [首页搜索](https://github.com/white0range/w0r_oj/blob/0b00ebf/vue/src/views/Home.vue#L101) · [搜索查询](../internal/problem/repository/search_repo.go#L28)

- 改进建议：让首页复用服务端搜索，补充分页、总数和题号精确匹配，保留标签筛选。
- 验收条件：可以从首页找到其他分页中的题目，结果总数、分页和标签筛选一致。

### 7. 减少重复生成题目向量

每次判题完成都会安排题目索引同步，后台还每 30 分钟重新安排全量同步。提交数和通过数进入向量文本，题面未变时也会调用向量生成服务。 [判题后同步](../internal/judge/repository/judge_repo.go#L93) · [全量校准](../internal/syncer/manager.go#L236) · [向量文档](../agent/rag/problem_doc_service.py#L48)

- 改进建议：将计数作为独立元数据，按题目内容哈希和向量模型版本判断是否需要重新生成向量。周期校准优先补缺失或版本不一致的记录。
- 验收条件：连续提交和内容无变化的校准不会产生新的向量请求。修改题面、标签或向量模型版本后，相关索引可以更新。

### 8. 让搜索故障可降级，判题积压可观测

Elasticsearch 连接或索引初始化失败会中止 Go API 启动。`/ping` 只能确认 HTTP 进程能响应，无法反映判题任务积压。 [Elasticsearch 初始化](../infrastructure/search/elasticsearch.go#L15) · [探活路由](../internal/app/routes.go#L36)

- 改进建议：允许 API 在搜索不可用时启动，并对搜索接口返回明确的降级状态。监控最老待判任务的等待时间、死信数量和 Worker 心跳。
- 验收条件：Elasticsearch 停止时仍能浏览题库和提交判题。暂停 Worker 后，监控能识别积压并触发告警。

## 验证记录

`go test -mod=readonly ./...` 已通过。本机 Docker daemon 未运行，隐藏用例反馈、沙箱故障重试和完整判题链路仍需在 Linux Docker 环境做端到端验证。

## 后续可迭代目标

### 判题 Worker 阻塞领取任务

记录于 2026-11-19。正式判题 Worker 空闲时，每 200 毫秒查询一次 Redis。有任务时，通过 Lua 将任务从待领取队列移到处理中队列，并在同一次操作中登记租约。 [Worker 轮询](../internal/judge/worker/judge_worker.go#L132) · [Redis 领取逻辑](../internal/judge/queue/queue.go#L64)

- **改进建议**。评估 Redis 阻塞领取，让 Worker 在队列为空时等待，有任务时由 Redis 返回领取结果。空闲查询量或领取延迟成为负担时，可考虑采用 `BLMOVE`。 [Redis BLMOVE 文档](https://redis.io/docs/latest/commands/blmove/)
- **验收条件**。空队列时停止固定间隔查询，新任务入队后空闲 Worker 可以及时领取。结果落库后再确认任务，Worker 在任务移入处理中队列后、租约登记前退出时，任务仍能被恢复。停止进程时，等待中的 Worker 可以及时退出。
