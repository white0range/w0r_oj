from langchain_runner import run_study_plan_with_langchain


def run_study_plan_agent(user_id: int, goal: str, token: str):
    # 这一层是“稳定入口”而不是“业务实现层”：
    # app.py 只知道“我要跑学习规划 Agent”，并不知道底层具体用的是哪种 runtime。
    # 这样以后切回手写 DeepSeek loop、做 A/B 实验，或者按配置切换实现时，
    # 只需要改这个小文件，而不用动 HTTP 路由层。
    return run_study_plan_with_langchain(
        user_id=user_id,
        goal=goal,
        token=token,
    )
