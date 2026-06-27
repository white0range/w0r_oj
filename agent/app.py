from fastapi import FastAPI, Header, HTTPException

from agent_runner import run_study_plan_agent
from rag.index_service import delete_problem_doc, upsert_problem_doc
from rag.problem_doc_service import fetch_problem_doc_record
from schemas import (
    MessageResponse,
    ProblemIndexSyncRequest,
    StudyPlanRequest,
    StudyPlanResponse,
)

# FastAPI 这一层只负责 HTTP 协议适配：
# 1. 接收请求和做最薄的参数校验
# 2. 调用下层 Agent / RAG 服务
# 3. 把 Python 内部对象包装成 HTTP 响应
# 这里刻意不放业务推理逻辑，避免入口层和具体 Agent 实现耦合。
app = FastAPI()


def _require_bearer_token(authorization: str) -> str:
    # 所有对外接口都要求 Bearer Token。
    # 这里统一抽成一个小函数，避免每个路由都重复写鉴权前置校验。
    if not authorization.startswith("Bearer "):
        raise HTTPException(status_code=401, detail="missing bearer token")

    return authorization.removeprefix("Bearer ").strip()


@app.get("/ping")
def ping():
    # 探活接口：容器、反向代理、Go worker 都可以用它快速确认 Agent 服务是否存活。
    return {"message": "agent service is running"}


@app.post("/study-plan/run", response_model=StudyPlanResponse)
def run_study_plan(req: StudyPlanRequest, authorization: str = Header(default="")):
    # 学习规划主入口：
    # - 这里只负责把 HTTP 请求转成 Python 函数调用
    # - 真正的 Agent 运行时、Tool Calling、RAG、memory 都在下层处理
    token = _require_bearer_token(authorization)

    try:
        # 上层只依赖一个稳定入口 run_study_plan_agent，
        # 而不直接绑定某一种运行时实现（LangChain / 手写 loop）。
        result = run_study_plan_agent(
            user_id=req.user_id,
            goal=req.goal,
            token=token,
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"run study plan agent failed: {e}")

    return StudyPlanResponse(
        message="study plan generated successfully",
        result=result,
    )


@app.post("/rag/problems/sync", response_model=MessageResponse)
def sync_problem_doc(req: ProblemIndexSyncRequest, authorization: str = Header(default="")):
    # 单题增量同步入口：
    # Go 题目新增/更新后，会调用这个接口把最新题目同步到 Qdrant，
    # 避免每次都做全量重建索引。
    token = _require_bearer_token(authorization)

    try:
        doc = fetch_problem_doc_record(problem_id=req.problem_id, token=token)
        upsert_problem_doc(doc)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"sync problem doc failed: {e}")

    return MessageResponse(message=f"problem {req.problem_id} synced to qdrant")


@app.post("/rag/problems/delete", response_model=MessageResponse)
def remove_problem_doc(req: ProblemIndexSyncRequest, authorization: str = Header(default="")):
    # 题目删除同步入口：
    # 当 Go 主系统删除题目时，这里负责把对应向量文档从 Qdrant 一并删掉。
    _require_bearer_token(authorization)

    try:
        delete_problem_doc(req.problem_id)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"delete problem doc failed: {e}")

    return MessageResponse(message=f"problem {req.problem_id} deleted from qdrant")
