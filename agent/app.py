import hmac
import os

from fastapi import FastAPI, Header, HTTPException

from agent_runner import run_chat_agent, summarize_chat_session
from rag.index_service import delete_problem_doc, upsert_problem_doc
from rag.problem_doc_service import fetch_problem_doc_record
from schemas import ChatRequest, ChatResponse, MessageResponse, ProblemIndexSyncRequest, SessionSummaryRequest, SessionSummaryResponse

app = FastAPI()


def _require_service_token(token: str) -> None:
    expected = os.getenv("AGENT_SERVICE_TOKEN", "")
    if not expected or not hmac.compare_digest(token, expected):
        raise HTTPException(status_code=401, detail="invalid agent service token")


def _require_bearer_token(authorization: str) -> str:
    if not authorization.startswith("Bearer "):
        raise HTTPException(status_code=401, detail="missing bearer token")
    return authorization.removeprefix("Bearer ").strip()


@app.get("/ping")
def ping():
    return {"message": "agent service is running"}


@app.post("/chat/run", response_model=ChatResponse)
def run_chat(req: ChatRequest, authorization: str = Header(default=""), x_agent_service_token: str = Header(default="")):
    _require_service_token(x_agent_service_token)
    result = run_chat_agent(req.user_id, req.goal, _require_bearer_token(authorization), req.session_summary, [item.model_dump() for item in req.messages])
    return ChatResponse(message="chat response generated successfully", result=result)


@app.post("/chat/summarize-session", response_model=SessionSummaryResponse)
def summarize_session(req: SessionSummaryRequest, authorization: str = Header(default=""), x_agent_service_token: str = Header(default="")):
    _require_service_token(x_agent_service_token)
    summary = summarize_chat_session(req.existing_summary, [item.model_dump() for item in req.messages])
    _require_bearer_token(authorization)
    return SessionSummaryResponse(message="session summary generated successfully", summary=summary)


@app.post("/rag/problems/sync", response_model=MessageResponse)
def sync_problem_doc(req: ProblemIndexSyncRequest, x_agent_service_token: str = Header(default="")):
    _require_service_token(x_agent_service_token)
    upsert_problem_doc(fetch_problem_doc_record(problem_id=req.problem_id))
    return MessageResponse(message=f"problem {req.problem_id} synced to qdrant")


@app.post("/rag/problems/delete", response_model=MessageResponse)
def remove_problem_doc(req: ProblemIndexSyncRequest, x_agent_service_token: str = Header(default="")):
    _require_service_token(x_agent_service_token)
    delete_problem_doc(req.problem_id)
    return MessageResponse(message=f"problem {req.problem_id} deleted from qdrant")
