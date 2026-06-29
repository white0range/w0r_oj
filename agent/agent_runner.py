from langchain_runner import run_study_plan_with_langchain, summarize_study_plan_session


def run_study_plan_agent(
    user_id: int,
    goal: str,
    token: str,
    session_summary: str = "",
    messages: list[dict] | None = None,
):
    return run_study_plan_with_langchain(
        user_id=user_id,
        goal=goal,
        token=token,
        session_summary=session_summary,
        messages=messages or [],
    )


def summarize_study_plan_session_agent(existing_summary: str, messages: list[dict] | None = None) -> str:
    return summarize_study_plan_session(
        existing_summary=existing_summary,
        messages=messages or [],
    )
