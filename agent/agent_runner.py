from langchain_runner import run_chat_with_langchain, summarize_chat_session

def run_chat_agent(user_id: int, goal: str, token: str, session_summary: str = "", messages: list[dict] | None = None):
    return run_chat_with_langchain(user_id, goal, token, session_summary, messages or [])
