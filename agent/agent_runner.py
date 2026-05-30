from deepseek_runner import run_study_plan_with_deepseek


def run_study_plan_agent(user_id: int, goal: str, token: str):
    return run_study_plan_with_deepseek(
        user_id=user_id,
        goal=goal,
        token=token,
    )
