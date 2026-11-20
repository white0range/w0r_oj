from datetime import datetime
from pydantic import BaseModel, Field

class ChatMessage(BaseModel):
    role: str
    content: str = ""

class ChatRequest(BaseModel):
    turn_id: int = Field(gt=0)
    processing_token: str = Field(min_length=64, max_length=64, repr=False)
    user_id: int = Field(gt=0)
    goal: str = ""
    session_summary: str = ""
    messages: list[ChatMessage] = Field(default_factory=list)

class SessionSummaryRequest(BaseModel):
    turn_id: int = Field(gt=0)
    existing_summary: str = ""
    messages: list[ChatMessage] = Field(default_factory=list)

class RecommendedProblem(BaseModel):
    problem_id: int
    title: str
    reason: str

class TrialRecord(BaseModel):
    status: str
    output: str = ""
    duration_ms: int | None = None
    truncated: bool = False


class ChatResult(BaseModel):
    answer: str
    weak_tags: list[str] = Field(default_factory=list)
    recommended_problems: list[RecommendedProblem] = Field(default_factory=list)
    response_type: str = "qa"
    trial: TrialRecord | None = None

class ChatResponse(BaseModel):
    message: str
    result: ChatResult

class ChatMemorySyncRequest(BaseModel):
    turn_id: int = Field(gt=0)
    user_id: int = Field(gt=0)
    query_text: str
    result: ChatResult
    finished_at: datetime

class SessionSummaryResponse(BaseModel):
    message: str
    summary: str

class ProblemIndexSyncRequest(BaseModel):
    problem_id: int

class MessageResponse(BaseModel):
    message: str
