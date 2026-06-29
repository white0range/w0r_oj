from pydantic import BaseModel, Field


class ChatMessage(BaseModel):
    role: str
    content: str = ""


class StudyPlanRequest(BaseModel):
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


class StudyPlanResult(BaseModel):
    weak_tags: list[str]
    recommended_problems: list[RecommendedProblem]
    study_plan_summary: str


class StudyPlanResponse(BaseModel):
    message: str
    result: StudyPlanResult


class SessionSummaryResponse(BaseModel):
    message: str
    summary: str


class ProblemIndexSyncRequest(BaseModel):
    problem_id: int


class MessageResponse(BaseModel):
    message: str
