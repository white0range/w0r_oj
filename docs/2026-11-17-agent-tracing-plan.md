# Agent 执行日志

日期：2026-11-17

已实现：复用框架回调输出执行日志，生产交给 Docker 收集。不需要追踪平台账号、额外 Key、新服务或数据库变更。本地不写日志文件。

## 记录内容

- Go：任务创建、入队、领取、请求 Python、保存结果、失败、重试和记忆同步。
- Python：任务开始/结束，模型和工具的开始/结束、耗时、错误类型，以及摘要、记忆召回和回答解析。
- 模型返回 Token 用量时记录数值，不估算缺失的数据。
- 每条记录带 `turn_id`；同一任务重试时通过 ProcessingToken 摘要形成的 `attempt_id` 区分执行，不输出原始令牌。

示例：

```text
[chat] turn_id=87 stage=agent_start
[agent] turn_id=87 stage=model_start name=ChatDeepSeek
[agent] turn_id=87 stage=tool_done name=user_ac_history duration_ms=30
[agent] turn_id=87 stage=model_done duration_ms=2000 input_tokens=100 output_tokens=50
[chat] turn_id=87 stage=completed
```

Python 回调不记录提示词、源码、完整对话、工具返回正文或异常原文。异常只记录类型。日志输出失败不会重跑模型或改变业务异常。

## 实现位置

- `agent/tracing.py`：任务上下文、标准输出和模型/工具回调。
- `agent/langchain_runner.py`：将回调传给现有 Agent、摘要模型；补记忆和解析步骤。
- `agent/app.py`：给对话、摘要和异步记忆任务绑定回合编号。
- Go Chat Service/Worker 和记忆同步 Handler：沿用已有的任务编号日志。

已移除云端追踪接入、直接 SDK 依赖和相关环境配置。框架自身可能仍间接依赖相关 SDK，应用主动关闭自动导出，不使用外部追踪服务。

## 使用

无需新增配置，更新代码后按原方式部署。生产日志仍由现有 Docker 日志轮转限制容量。查询某个任务：

```bash
docker compose --env-file .env.local-production \
  -f docker-compose.prod.yml -f docker-compose.4g.yml \
  logs --no-color api agent 2>&1 | rg 'turn_id=87([^0-9]|$)'
```

只能查到 Docker 尚未淘汰的日志。本地直接运行时在终端查看输出。

## 验证

使用合成模型和内存内的 API 测试，验证回合编号、模型/工具事件、耗时、数值用量、敏感内容过滤和并发任务隔离。不调用外部模型或追踪服务，不启动后端服务进行连通性验证。
