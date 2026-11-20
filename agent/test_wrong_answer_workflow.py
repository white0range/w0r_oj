from types import SimpleNamespace
from unittest import TestCase
from unittest.mock import Mock, patch

import requests

from client import AgentExecutionExpired, AgentToolAuth
from langchain_runner import run_chat_with_langchain
from wrong_answer_workflow import Diagnosis, SubmissionChoice, is_wrong_answer_request, run_wrong_answer_workflow


class WrongAnswerWorkflowTests(TestCase):
    def setUp(self):
        self.auth = object()
        self.model = Mock()
        self.check_active = Mock()
        self.diagnose = self.model.with_structured_output.return_value.invoke
        self.diagnose.return_value = Diagnosis(
            answer="初步分析", needs_trial=True, hypothesis="检查空数组", trial_input="0\n", expected_output="0\n",
        )
        self.detail = self.enterContext(patch("wrong_answer_workflow.get_failed_submission_detail"))
        self.detail.return_value = {"data": {
            "submission_id": 42, "problem_title": "求和", "code": "package main", "status": "WA", "language": "go",
            "problem_description": "求数组元素之和", "actual_output": "hidden-case marker",
        }}
        self.trial = self.enterContext(patch("wrong_answer_workflow.trial_submission"))

    def run_workflow(self, query="分析提交 #42"):
        return run_wrong_answer_workflow(self.model, query, [], self.auth, self.check_active)

    def test_enough_evidence_answers_directly(self):
        self.diagnose.return_value.needs_trial = False
        answer = self.run_workflow()
        self.assertEqual(answer.answer, "初步分析")
        self.assertIsNone(answer.trial)
        self.detail.assert_called_once_with(42, self.auth)
        self.trial.assert_not_called()
        self.model.invoke.assert_not_called()
        self.assertNotIn("hidden-case marker", str(self.diagnose.call_args))

    def test_runs_once_and_reviews_actual_output(self):
        self.trial.return_value = {"status": "ok", "output": "-1\n"}
        self.model.invoke.return_value = SimpleNamespace(content="实际输出 -1，预期 0。")
        answer = self.run_workflow()
        self.assertEqual(answer.answer, "实际输出 -1，预期 0。")
        self.assertEqual(answer.response_type, "analysis")
        self.assertEqual(answer.trial.status, "ok")
        self.assertEqual(answer.trial.output, "-1\n")
        self.trial.assert_called_once_with(42, "0\n", self.auth)
        self.model.invoke.assert_called_once()
        self.assertIn('"output": "-1', self.model.invoke.call_args.args[0][1][1])

    def test_trial_timeout_preserves_unverified_analysis(self):
        self.trial.side_effect = requests.Timeout()
        answer = self.run_workflow()
        self.assertIn("还没有经过运行验证", answer.answer)
        self.assertEqual(answer.trial.status, "unavailable")
        self.trial.assert_called_once()
        self.model.invoke.assert_not_called()

    def test_excess_input_bytes_prevent_execution(self):
        self.diagnose.return_value.trial_input = "字" * 1400
        self.assertIn("还没有经过运行验证", self.run_workflow().answer)
        self.trial.assert_not_called()

    def test_stale_execution_does_not_return_result(self):
        self.trial.side_effect = AgentExecutionExpired()
        with self.assertRaises(AgentExecutionExpired):
            self.run_workflow()
        self.model.invoke.assert_not_called()

    def test_model_can_only_choose_from_recent_failed_submissions(self):
        with patch("wrong_answer_workflow.get_user_failed_submissions") as submissions:
            submissions.return_value = {"data": {"items": [{"submission_id": 42, "status": "WA"}]}}
            self.diagnose.return_value = SubmissionChoice(submission_id=99)
            self.assertIn("请提供提交编号", self.run_workflow("分析我最近的错题").answer)
        self.detail.assert_not_called()
        self.trial.assert_not_called()

    def test_only_specific_diagnoses_enter_workflow(self):
        self.assertTrue(is_wrong_answer_request("分析提交 #42"))
        self.assertTrue(is_wrong_answer_request("为什么没过？"))
        self.assertTrue(is_wrong_answer_request("为什么这次提交会WA？"))
        self.assertTrue(is_wrong_answer_request("试跑一下"))
        self.assertTrue(is_wrong_answer_request("提交编号 42"))
        self.assertFalse(is_wrong_answer_request("提交 #42 对应的题目叫什么？"))
        self.assertFalse(is_wrong_answer_request("推荐三道动态规划题"))

    def test_chat_graph_routes_latest_message_to_diagnosis(self):
        auth = AgentToolAuth(87, "a" * 64)
        self.diagnose.return_value.needs_trial = False
        with (
            patch("langchain_runner.require_active_execution", return_value={"user_id": 42}),
            patch("langchain_runner._build_model", return_value=self.model),
            patch("langchain_runner.create_agent") as general_agent,
        ):
            result = run_chat_with_langchain(42, "推荐练习", auth, messages=[
                {"role": "user", "content": "分析提交 #42"},
            ])
        self.assertEqual(result.answer, "初步分析")
        self.detail.assert_called_once_with(42, auth)
        general_agent.assert_not_called()
