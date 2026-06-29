from fastapi import FastAPI, Header, HTTPException

from agent_runner import run_study_plan_agent, summarize_study_plan_session_agent
from rag.index_service import delete_problem_doc, upsert_problem_doc
from rag.problem_doc_service import fetch_problem_doc_record
from schemas import (
    MessageResponse,
    ProblemIndexSyncRequest,
    SessionSummaryRequest,
    SessionSummaryResponse,
    StudyPlanRequest,
    StudyPlanResponse,
)

app = FastAPI()


def _require_bearer_token(authorization: str) -> str:
    if not authorization.startswith("Bearer "):
        raise HTTPException(status_code=401, detail="missing bearer token")

    return authorization.removeprefix("Bearer ").strip()


@app.get("/ping")
def ping():
    return {"message": "agent service is running"}


@app.post("/study-plan/run", response_model=StudyPlanResponse)
def run_study_plan(req: StudyPlanRequest, authorization: str = Header(default="")):
    token = _require_bearer_token(authorization)

    try:
        result = run_study_plan_agent(
            user_id=req.user_id,
            goal=req.goal,
            token=token,
            session_summary=req.session_summary,
            messages=[item.model_dump() for item in req.messages],
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"run study plan agent failed: {e}")

    return StudyPlanResponse(
        message="study plan generated successfully",
        result=result,
    )


@app.post("/study-plan/summarize-session", response_model=SessionSummaryResponse)
def summarize_session(req: SessionSummaryRequest, authorization: str = Header(default="")):
    _require_bearer_token(authorization)

    try:
        summary = summarize_study_plan_session_agent(
            existing_summary=req.existing_summary,
            messages=[item.model_dump() for item in req.messages],
        )
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"summarize study plan session failed: {e}")

    return SessionSummaryResponse(
        message="session summary generated successfully",
        summary=summary,
    )


@app.post("/rag/problems/sync", response_model=MessageResponse)
def sync_problem_doc(req: ProblemIndexSyncRequest, authorization: str = Header(default="")):
    token = _require_bearer_token(authorization)

    try:
        doc = fetch_problem_doc_record(problem_id=req.problem_id, token=token)
        upsert_problem_doc(doc)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"sync problem doc failed: {e}")

    return MessageResponse(message=f"problem {req.problem_id} synced to qdrant")


@app.post("/rag/problems/delete", response_model=MessageResponse)
def remove_problem_doc(req: ProblemIndexSyncRequest, authorization: str = Header(default="")):
    _require_bearer_token(authorization)

    try:
        delete_problem_doc(req.problem_id)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"delete problem doc failed: {e}")

    return MessageResponse(message=f"problem {req.problem_id} deleted from qdrant")
