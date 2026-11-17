import io
import os
from contextlib import redirect_stdout
from concurrent.futures import ThreadPoolExecutor
from uuid import uuid4
import unittest
from unittest.mock import patch

from fastapi.testclient import TestClient
from langchain_core.language_models.chat_models import BaseChatModel
from langchain_core.messages import AIMessage
from langchain_core.outputs import ChatGeneration, ChatResult

import app as agent_app
import langchain_runner
import tracing


class TracingModel(BaseChatModel):
    calls: int = 0

    @property
    def _llm_type(self):
        return "offline-tracing-test"

    def bind_tools(self, tools, **kwargs):
        return self

    def _generate(self, messages, stop=None, run_manager=None, **kwargs):
        self.calls += 1
        if self.calls == 1:
            message = AIMessage(content="", tool_calls=[{"name": "user_ac_history", "args": {}, "id": "test-call"}])
        else:
            message = AIMessage(content="ANSWER:\nprivate-answer")
        return ChatResult(generations=[ChatGeneration(message=message)], llm_output={"token_usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}})


class TracingTests(unittest.TestCase):
    def setUp(self):
        env = patch.dict(os.environ, {"APP_ENV": "test", "AGENT_DEBUG": "false", "AGENT_SERVICE_TOKEN": "service-secret"})
        env.start()
        self.addCleanup(env.stop)

    def test_real_graph_logs_models_tools_and_custom_steps_without_network(self):
        output = io.StringIO()
        model = TracingModel()
        # Old cloud settings must not activate any automatic exporter.
        with patch.dict(os.environ, {"LANGSMITH_TRACING": "true", "LANGCHAIN_TRACING_V2": "true", "LANGSMITH_API_KEY": "private-cloud-key"}), \
             patch("requests.Session.request", side_effect=AssertionError("unexpected network")) as network, \
             patch.object(langchain_runner, "_build_model", return_value=model), \
             patch.object(langchain_runner, "require_active_execution", return_value={"user_id": 42}), \
             patch.object(langchain_runner, "search_user_memories", return_value=[]), \
             patch("tool_executor.require_active_execution", return_value={"user_id": 42}), \
             patch("tool_executor.get_user_ac_history", return_value={"data": "private-tool-result"}), \
             redirect_stdout(output):
            response = TestClient(agent_app.app).post(
                "/chat/run", headers={"X-Agent-Service-Token": "service-secret"},
                json={"turn_id": 87, "user_id": 42, "processing_token": "a" * 64, "goal": "private-user-query"},
            )
            self.assertEqual(os.environ["LANGSMITH_TRACING"], "false")
            self.assertEqual(os.environ["LANGCHAIN_TRACING_V2"], "false")
        self.assertEqual(response.status_code, 200, response.text)
        self.assertIn("private-answer", response.text)
        self.assertEqual(model.calls, 2)
        network.assert_not_called()
        logs = output.getvalue()
        for stage in ("agent_start", "memory.retrieve_start", "model_start", "model_done", "tool_start", "tool_done", "response.parse_done", "agent_done"):
            self.assertIn("stage=" + stage, logs)
        self.assertIn("name=user_ac_history", logs)
        self.assertIn("input_tokens=10 output_tokens=5 total_tokens=15", logs)
        for line in logs.splitlines():
            self.assertIn("turn_id=87", line)
        for secret in ("private-user-query", "private-answer", "private-tool-result", "a" * 64, "service-secret", "private-cloud-key"):
            self.assertNotIn(secret, logs)

    def test_memory_error_keeps_task_id_and_preserves_business_error(self):
        output = io.StringIO()
        payload = {"turn_id": 87, "user_id": 42, "query_text": "private-user-query", "result": {"answer": "private-answer"}, "finished_at": "2026-11-17T00:00:00Z"}
        with patch.object(agent_app, "save_chat_memory", side_effect=ValueError("private-error")), redirect_stdout(output):
            with self.assertRaisesRegex(ValueError, "private-error"):
                TestClient(agent_app.app).post("/memory/sync", json=payload, headers={"X-Agent-Service-Token": "service-secret"})
        logs = output.getvalue()
        self.assertIn("turn_id=87 stage=memory_sync_failed", logs)
        self.assertIn("error_type=ValueError", logs)
        self.assertNotIn("private-error", logs)
        self.assertNotIn("private-user-query", logs)

    def test_parallel_tasks_keep_separate_contexts(self):
        output = io.StringIO()

        def run(turn_id):
            with tracing.trace_task(turn_id, "test"):
                with tracing.trace_step(f"step_{turn_id}"):
                    pass

        with redirect_stdout(output), ThreadPoolExecutor(max_workers=2) as pool:
            list(pool.map(run, (87, 88)))
        for turn_id in (87, 88):
            for line in output.getvalue().splitlines():
                if f"step_{turn_id}" in line:
                    self.assertIn(f"turn_id={turn_id}", line)
        self.assertEqual(tracing.console_callbacks(), [])

    def test_model_and_tool_failures_do_not_log_inputs_or_error_messages(self):
        output = io.StringIO()
        with redirect_stdout(output), tracing.trace_task(87, "test"):
            callback = tracing.console_callbacks()[0]
            model_id, tool_id = uuid4(), uuid4()
            callback.on_chat_model_start({"name": "model"}, [["private-prompt"]], run_id=model_id)
            callback.on_llm_error(RuntimeError("private-error"), run_id=model_id)
            callback.on_tool_start({"name": "tool"}, "private-source", run_id=tool_id)
            callback.on_tool_error(ValueError("private-tool-error"), run_id=tool_id)
        logs = output.getvalue()
        self.assertIn("stage=model_failed", logs)
        self.assertIn("stage=tool_failed", logs)
        self.assertNotIn("private-", logs)

    def test_output_failure_does_not_fail_business(self):
        with patch("builtins.print", side_effect=BrokenPipeError):
            with tracing.trace_task(87, "test"):
                with tracing.trace_step("step"):
                    value = 42
        self.assertEqual(value, 42)
        self.assertEqual(tracing.console_callbacks(), [])


if __name__ == "__main__":
    unittest.main()
