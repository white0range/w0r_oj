from langchain_runner import run_study_plan_with_langchain


def run_study_plan_agent(user_id: int, goal: str, token: str):
    return run_study_plan_with_langchain(
        user_id=user_id,
        goal=goal,
        token=token,
    )
