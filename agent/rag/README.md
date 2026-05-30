# Problem Doc Export

这个目录先做 RAG 的第一步：把题库导出成适合 embedding 的 `document + metadata` JSON。

## 为什么先做这个

`study_plan` 后面要做向量检索时，第一件事不是直接上 Qdrant，而是先把题目整理成稳定的语料格式。

当前脚本会：

1. 调用 Go 后端的公开题目接口拿题目列表
2. 再逐题调用详情接口拿完整描述
3. 输出 `problem_docs.json`

## 运行方式

在 `agent` 目录下执行：

```powershell
cd D:\beryl\letsgo\gojo\agent
python .\rag\export_problem_docs.py
```

如果 Go 服务不在默认地址，可以传：

```powershell
python .\rag\export_problem_docs.py --base-url http://localhost:8080
```

如果以后某个导出接口需要鉴权，可以传：

```powershell
python .\rag\export_problem_docs.py --token 你的token
```

## 输出格式

输出文件默认是：

`D:\beryl\letsgo\gojo\agent\rag\problem_docs.json`

每条记录结构如下：

```json
{
  "problem_id": 1,
  "document": "标题：二分查找入门\n标签：二分, 数组\n题目描述：...",
  "metadata": {
    "title": "二分查找入门",
    "tags": ["二分", "数组"],
    "submit_count": 123,
    "accepted_count": 45,
    "time_limit": 1000,
    "memory_limit": 256
  }
}
```

## 下一步

这一步跑通后，后面可以继续做：

1. 给 `document` 算 embedding
2. 写入向量库
3. 做检索 demo
4. 接回 `study_plan`

## 把文档写入 Qdrant

先确保本地 Qdrant 已经启动：

```powershell
cd D:\beryl\letsgo\gojo
docker compose up -d qdrant
```

再准备一个阿里云百炼 DashScope API Key。当前脚本直接走 DashScope SDK，所以需要：

- `DASHSCOPE_API_KEY`
- 可选：`EMBEDDING_MODEL`
- 可选：`EMBEDDING_DIMENSION`

例如：

```powershell
$env:DASHSCOPE_API_KEY="你的key"
$env:EMBEDDING_MODEL="text-embedding-v3"
$env:EMBEDDING_DIMENSION="1024"
python .\rag\index_problem_docs.py --qdrant-url http://localhost:6333
```

脚本会：

1. 读取 `problem_docs.json`
2. 调 DashScope `text-embedding-v3` 把 `document` 变成向量
3. 在 Qdrant 里创建 collection
4. 把 `problem_id + vector + payload(metadata)` 一起写进去

## 查询 demo

如果题目已经入库，可以直接做一次最小语义检索：

```powershell
cd D:\beryl\letsgo\gojo
docker compose run --rm agent python rag/search_demo.py "适合练二分边界处理的入门题"
```

或者：

```powershell
docker compose run --rm agent python rag/search_demo.py "适合 BFS 和 DFS 入门的图论题" --limit 3
```

它会打印：

- 相似度分数
- `problem_id`
- 标题
- 标签
- 文档预览
