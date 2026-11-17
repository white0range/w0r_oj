import os
import unittest
from unittest.mock import Mock, patch

from fastapi.testclient import TestClient

import app as agent_app
import client
from langchain_runner import _build_tools
from schemas import ChatResult
from tool_executor import ToolExecutionContext


class ServiceAuthTests(unittest.TestCase):
    def setUp(self):
        self.env = patch.dict(os.environ, {"APP_ENV": "test", "AGENT_DEBUG": "false", "AGENT_SERVICE_TOKEN": "test-service-key"})
        self.env.start()
        self.addCleanup(self.env.stop)
        self.http = TestClient(agent_app.app)

    def test_chat_rejects_missing_or_wrong_service_key_before_execution(self):
        payload = {"turn_id": 87, "user_id": 42, "goal": "test", "processing_token": "a" * 64}
        with patch.object(agent_app, "run_chat_agent") as run:
            for headers in ({}, {"Authorization": "Bearer user-jwt"}, {"X-Agent-Service-Token": "wrong"}):
                with self.subTest(headers=headers):
                    self.assertEqual(self.http.post("/chat/run", json=payload, headers=headers).status_code, 401)
            run.assert_not_called()

    def test_chat_accepts_service_key_without_user_jwt_and_binds_turn(self):
        with patch.object(agent_app, "run_chat_agent", return_value=ChatResult(answer="ok")) as run:
            response = self.http.post(
                "/chat/run", json={"turn_id": 87, "user_id": 42, "goal": "test", "processing_token": "a" * 64},
                headers={"X-Agent-Service-Token": "test-service-key"},
            )
            self.assertEqual(response.status_code, 200)
            self.assertEqual(run.call_args.args[:3], (42, "test", client.AgentToolAuth(turn_id=87, processing_token="a" * 64)))

    def test_chat_requires_positive_turn_id(self):
        with patch.object(agent_app, "run_chat_agent") as run:
            for turn_id in (None, 0, -1):
                payload = {"user_id": 42, "processing_token": "a" * 64}
                if turn_id is not None:
                    payload["turn_id"] = turn_id
                self.assertEqual(self.http.post("/chat/run", json=payload, headers={"X-Agent-Service-Token": "test-service-key"}).status_code, 422)
            run.assert_not_called()

    def test_summary_requires_only_service_key_and_rejects_before_model_call(self):
        with patch.object(agent_app, "summarize_chat_session", return_value="summary") as summarize:
            self.assertEqual(self.http.post("/chat/summarize-session", json={"turn_id": 87}).status_code, 401)
            summarize.assert_not_called()
            response = self.http.post("/chat/summarize-session", json={"turn_id": 87}, headers={"X-Agent-Service-Token": "test-service-key"})
            self.assertEqual(response.status_code, 200)
            summarize.assert_called_once()

    def test_user_tool_uses_service_header_and_turn_without_user_id(self):
        response = Mock()
        response.json.return_value = {"code": 0, "data": []}
        tools = {tool.name: tool for tool in _build_tools(client.AgentToolAuth(87, "a" * 64), ToolExecutionContext())}
        with patch.object(client.requests, "get", return_value=response) as get:
            tools["user_failed_submissions"].invoke({"limit": 3, "user_id": 99, "turn_id": 999})
        self.assertTrue(get.call_args.args[0].endswith("/api/agent/me/failed-submissions"))
        self.assertEqual(get.call_args.kwargs["headers"], {"X-Agent-Service-Token": "test-service-key", "X-Agent-Turn-ID": "87", "X-Agent-Processing-Token": "a" * 64})
        self.assertEqual(get.call_args.kwargs["params"], {"limit": 3})
        for tool in tools.values():
            fields = tool.args_schema.model_fields
            self.assertFalse({"auth", "token", "user_id", "turn_id", "processing_token"} & fields.keys())


if __name__ == "__main__":
    unittest.main()
