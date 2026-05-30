from pydantic import BaseModel


class StudyPlanRequest(BaseModel):
    user_id: int
    goal: str = ""

# 表示单道推荐题的结构
class RecommendedProblem(BaseModel):
    problem_id: int
    title: str
    reason: str


# 表示训练计划的核心结果
class StudyPlanResult(BaseModel):
    weak_tags: list[str]
    recommended_problems: list[RecommendedProblem]
    study_plan_summary: str


class StudyPlanResponse(BaseModel):
    message: str
    result: StudyPlanResult


class ProblemIndexSyncRequest(BaseModel):
    problem_id: int


class MessageResponse(BaseModel):
    message: str
