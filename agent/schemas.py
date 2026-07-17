from pydantic import BaseModel, Field

class ChatMessage(BaseModel):
    role: str
    content: str = ""

class ChatRequest(BaseModel):
    user_id: int
    goal: str = ""
    session_summary: str = ""
    messages: list[ChatMessage] = Field(default_factory=list)

class SessionSummaryRequest(BaseModel):
    existing_summary: str = ""
    messages: list[ChatMessage] = Field(default_factory=list)

class RecommendedProblem(BaseModel):
    problem_id: int
    title: str
    reason: str

class ChatResult(BaseModel):
    answer: str
    weak_tags: list[str] = Field(default_factory=list)
    recommended_problems: list[RecommendedProblem] = Field(default_factory=list)
    response_type: str = "qa"

class ChatResponse(BaseModel):
    message: str
    result: ChatResult

class SessionSummaryResponse(BaseModel):
    message: str
    summary: str

class ProblemIndexSyncRequest(BaseModel):
    problem_id: int

class MessageResponse(BaseModel):
    message: str
