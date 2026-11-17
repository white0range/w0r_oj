import os
import uuid
import unittest
from unittest.mock import Mock, patch

from fastapi.testclient import TestClient
from langchain_core.language_models.chat_models import BaseChatModel
from langchain_core.messages import AIMessage
from langchain_core.outputs import ChatGeneration, ChatResult as ModelResult
from qdrant_client import QdrantClient

import app as agent_app
import client
import langchain_runner
from rag import memory_service
from schemas import ChatResult


class ToolCallingModel(BaseChatModel):
    calls: int = 0

    @property
    def _llm_type(self):
        return "local-fencing-test"

    def bind_tools(self, tools, **kwargs):
        return self

    def _generate(self, messages, stop=None, run_manager=None, **kwargs):
        self.calls += 1
        message = AIMessage(content="", tool_calls=[{"name": "user_ac_history", "args": {}, "id": "call-1"}])
        return ModelResult(generations=[ChatGeneration(message=message)])


class ExecutionFencingTests(unittest.TestCase):
    def setUp(self):
        env = patch.dict(os.environ, {"AGENT_SERVICE_TOKEN": "test-key", "AGENT_DEBUG": "false"})
        env.start()
        self.addCleanup(env.stop)
        self.auth = client.AgentToolAuth(87, "a" * 64)

    def test_terminal_backend_error_is_distinct_from_submission_forbidden(self):
        response = Mock(status_code=403)
        response.json.return_value = {"code": 40302}
        with patch.object(client.requests, "get", return_value=response):
            with self.assertRaises(client.AgentExecutionExpired):
                client.require_active_execution(self.auth)
        response.raise_for_status.assert_not_called()
        response.json.return_value = {"code": 40301}
        response.raise_for_status.side_effect = client.requests.HTTPError("submission forbidden")
        with patch.object(client.requests, "get", return_value=response):
            with self.assertRaises(client.requests.HTTPError):
                client.get_failed_submission_detail(2, self.auth)

    def test_stale_tool_stops_real_graph_without_another_model_call(self):
        model = ToolCallingModel()
        with patch.object(langchain_runner, "_build_model", return_value=model), \
             patch.object(langchain_runner, "_load_memory_context", return_value=""), \
             patch.object(langchain_runner, "require_active_execution", return_value={"user_id": 42}), \
             patch("tool_executor.require_active_execution", side_effect=client.AgentExecutionExpired("stale")), \
             patch("tool_executor.get_user_ac_history") as history:
            with self.assertRaises(client.AgentExecutionExpired):
                langchain_runner.run_chat_with_langchain(42, "test", self.auth)
        self.assertEqual(model.calls, 1)
        history.assert_not_called()

    def test_stale_execution_stops_before_memory_lookup_or_model_build(self):
        with patch.object(langchain_runner, "require_active_execution", side_effect=client.AgentExecutionExpired("stale")), \
             patch.object(langchain_runner, "_load_memory_context") as memory, \
             patch.object(langchain_runner, "_build_model") as model:
            with self.assertRaises(client.AgentExecutionExpired):
                langchain_runner.run_chat_with_langchain(42, "test", self.auth)
        memory.assert_not_called()
        model.assert_not_called()

    def test_replaced_execution_stops_after_model_before_tool(self):
        model = ToolCallingModel()
        with patch.object(langchain_runner, "_build_model", return_value=model), \
             patch.object(langchain_runner, "_load_memory_context", return_value=""), \
             patch.object(langchain_runner, "require_active_execution", side_effect=[{"user_id": 42}, {"user_id": 42}, client.AgentExecutionExpired("replaced while model was running")]), \
             patch("tool_executor.require_active_execution") as tool_check:
            with self.assertRaises(client.AgentExecutionExpired):
                langchain_runner.run_chat_with_langchain(42, "test", self.auth)
        self.assertEqual(model.calls, 1)
        tool_check.assert_not_called()

    def test_successful_chat_does_not_save_unconfirmed_memory(self):
        graph = Mock()
        graph.invoke.return_value = {"messages": [AIMessage(content="ANSWER:\nok")]}
        with patch.object(langchain_runner, "require_active_execution", return_value={"user_id": 42}), \
             patch.object(langchain_runner, "_load_memory_context", return_value=""), \
             patch.object(langchain_runner, "_build_model"), \
             patch.object(langchain_runner, "create_agent", return_value=graph), \
             patch.object(memory_service, "save_chat_memory") as save:
            result = langchain_runner.run_chat_with_langchain(42, "test", self.auth)
        self.assertEqual(result.answer, "ok")
        save.assert_not_called()

    def test_expiration_has_sanitized_http_response(self):
        with patch.object(agent_app, "run_chat_agent", side_effect=client.AgentExecutionExpired("stale")):
            response = TestClient(agent_app.app).post(
                "/chat/run", headers={"X-Agent-Service-Token": "test-key"},
                json={"turn_id": 87, "user_id": 42, "processing_token": "a" * 64},
            )
        self.assertEqual(response.status_code, 409)
        self.assertEqual(response.json()["code"], "agent_execution_expired")
        self.assertNotIn("a" * 64, response.text)
        self.assertNotIn("a" * 64, repr(self.auth))

    def test_memory_sync_requires_service_key(self):
        payload = {"turn_id": 87, "user_id": 42, "query_text": "sample", "result": {"answer": "ok"}, "finished_at": "2026-11-17T00:00:00Z"}
        with patch.object(agent_app, "save_chat_memory") as save:
            http = TestClient(agent_app.app)
            self.assertEqual(http.post("/memory/sync", json=payload).status_code, 401)
            save.assert_not_called()
            self.assertEqual(http.post("/memory/sync", json=payload, headers={"X-Agent-Service-Token": "test-key"}).status_code, 200)
            self.assertEqual(save.call_args.kwargs["turn_id"], 87)

    def test_memory_retries_upsert_one_point_with_stable_payload(self):
        points = {}
        qdrant = Mock()

        def upsert(**kwargs):
            self.assertTrue(kwargs["wait"])
            for point in kwargs["points"]:
                points[point.id] = point.payload

        qdrant.upsert.side_effect = upsert
        args = dict(user_id=42, turn_id=87, query_text="sample", result=ChatResult(answer="ok"), created_at="2026-11-17T00:00:00+00:00")
        with patch.object(memory_service, "qdrant_client", return_value=qdrant), \
             patch.object(memory_service, "embed_documents", return_value=[[0.1, 0.2]]), \
             patch.object(memory_service, "ensure_collection"):
            first = memory_service.save_chat_memory(**args)
            first_payload = points[first].copy()
            second = memory_service.save_chat_memory(**args)
            self.assertEqual(first, second)
            self.assertEqual(len(points), 1)
            self.assertEqual(points[first], first_payload)
            args["turn_id"] = 88
            self.assertNotEqual(first, memory_service.save_chat_memory(**args))
            self.assertEqual(len(points), 2)

    @unittest.skipUnless(os.getenv("GOJO_TEST_QDRANT_URL"), "set GOJO_TEST_QDRANT_URL for isolated collection integration test")
    def test_integration_repeated_memory_sync_leaves_one_qdrant_point(self):
        qdrant = QdrantClient(url=os.environ["GOJO_TEST_QDRANT_URL"])
        collection = "gojo_memory_test_" + uuid.uuid4().hex
        args = dict(user_id=42, turn_id=87, query_text="sample", result=ChatResult(answer="ok"), created_at="2026-11-17T00:00:00+00:00", collection_name=collection)
        try:
            with patch.object(memory_service, "qdrant_client", return_value=qdrant), \
                 patch.object(memory_service, "embed_documents", return_value=[[0.1, 0.2]]):
                first = memory_service.save_chat_memory(**args)
                second = memory_service.save_chat_memory(**args)
                self.assertEqual(first, second)
                self.assertEqual(qdrant.count(collection, exact=True).count, 1)
                point = qdrant.retrieve(collection, ids=[first])[0]
                self.assertEqual(point.payload["turn_id"], 87)
                self.assertEqual(point.payload["chat_result"]["answer"], "ok")
        finally:
            if qdrant.collection_exists(collection):
                qdrant.delete_collection(collection)
            qdrant.close()


if __name__ == "__main__":
    unittest.main()
