import hmac
import os
from contextlib import asynccontextmanager

from fastapi import FastAPI, Header, HTTPException, Request
from fastapi.responses import JSONResponse

from client import AgentToolAuth, AgentExecutionExpired
from agent_runner import run_chat_agent, summarize_chat_session
from rag.index_service import delete_problem_doc, upsert_problem_doc
from rag.problem_doc_service import fetch_problem_doc_record
from rag.memory_service import save_chat_memory
from tracing import trace_task
from schemas import ChatRequest, ChatResponse, ChatMemorySyncRequest, MessageResponse, ProblemIndexSyncRequest, SessionSummaryRequest, SessionSummaryResponse

MIN_PRODUCTION_SECRET_LENGTH = 32


def _is_production() -> bool:
    return os.getenv("APP_ENV", "dev").strip().lower() in {"prod", "production"}


def _validate_startup_config() -> None:
    if not _is_production():
        return

    token = os.getenv("AGENT_SERVICE_TOKEN", "").strip()
    lower = token.lower()
    if len(token) < MIN_PRODUCTION_SECRET_LENGTH or "replace-with" in lower or "change-me" in lower:
        raise RuntimeError("AGENT_SERVICE_TOKEN must be a random secret of at least 32 characters in production")


@asynccontextmanager
async def lifespan(_: FastAPI):
    _validate_startup_config()
    yield


app = FastAPI(lifespan=lifespan)


@app.exception_handler(AgentExecutionExpired)
def expired_execution(_: Request, __: AgentExecutionExpired):
    return JSONResponse(status_code=409, content={"code": "agent_execution_expired", "message": "agent execution is no longer current"})


def _require_service_token(token: str) -> None:
    expected = os.getenv("AGENT_SERVICE_TOKEN", "").strip()
    if not expected or not hmac.compare_digest(token.encode("utf-8"), expected.encode("utf-8")):
        raise HTTPException(status_code=401, detail="invalid agent service token")


@app.get("/ping")
def ping():
    return {"message": "agent service is running"}


@app.post("/chat/run", response_model=ChatResponse)
def run_chat(req: ChatRequest, x_agent_service_token: str = Header(default="")):
    _require_service_token(x_agent_service_token)
    with trace_task(req.turn_id, "agent", req.processing_token):
        result = run_chat_agent(req.user_id, req.goal, AgentToolAuth(turn_id=req.turn_id, processing_token=req.processing_token), req.session_summary, [item.model_dump() for item in req.messages])
    return ChatResponse(message="chat response generated successfully", result=result)


@app.post("/memory/sync", response_model=MessageResponse)
def sync_chat_memory(req: ChatMemorySyncRequest, x_agent_service_token: str = Header(default="")):
    _require_service_token(x_agent_service_token)
    with trace_task(req.turn_id, "memory_sync"):
        save_chat_memory(user_id=req.user_id, turn_id=req.turn_id, query_text=req.query_text,
                         result=req.result, created_at=req.finished_at.isoformat())
    return MessageResponse(message="chat memory synced")


@app.post("/chat/summarize-session", response_model=SessionSummaryResponse)
def summarize_session(req: SessionSummaryRequest, x_agent_service_token: str = Header(default="")):
    _require_service_token(x_agent_service_token)
    with trace_task(req.turn_id, "summary"):
        summary = summarize_chat_session(req.existing_summary, [item.model_dump() for item in req.messages])
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
