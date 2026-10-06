"""中高级课程业务边界与 review 回归，全程离线。"""

import json
import os
import unittest
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest.mock import patch

import httpx

from demos.advanced.advanced_02_recovery import LocalJobs
from demos.advanced.advanced_03_evaluation import evaluate, grade
from demos.advanced.advanced_04_knowledge_agent import run_knowledge_agent
from demos.intermediate.intermediate_03_reliability import retry_chat
from demos.intermediate.intermediate_04_rag import answer_question
from demos.intermediate.intermediate_05_memory import ask
from llm import ModelError
from offline_lab import completion, offline_client
from retrieval import search_documents, validate_answer
from session_store import SessionConflict, SessionStore
from settings import Settings, env_bool


def search_call(query="导出格式"):
    return {
        "id": "search-1",
        "type": "function",
        "function": {
            "name": "search_documents",
            "arguments": json.dumps({"query": query}, ensure_ascii=False),
        },
    }


class LevelTests(unittest.TestCase):
    def test_invalid_boolean_cannot_silently_disable_telemetry(self):
        with patch.dict(os.environ, {"TELEMETRY_ENABLED": "tru"}):
            with self.assertRaises(ValueError):
                env_bool("TELEMETRY_ENABLED")
        with self.assertRaises(ValueError):
            Settings(api_key="test", timeout=float("nan"))

    def test_malformed_response_is_model_error(self):
        responses = [
            completion(content=["not text"]),
            completion(content="   "),
            completion(content="not final", calls={"unexpected": "object"}),
        ]
        bad_usage = completion("hi")
        bad_usage["usage"] = {"total_tokens": "20"}
        responses.append(bad_usage)
        inconsistent = completion("hello", calls=[search_call()])
        inconsistent["choices"][0]["finish_reason"] = "stop"
        responses.append(inconsistent)
        for body in responses:
            with self.subTest(body=body):
                model, memory = offline_client(
                    "bad_protocol", lambda req: httpx.Response(200, json=body)
                )
                with self.assertRaises(ModelError):
                    with model:
                        model.chat([])
                self.assertEqual(len(memory.get_finished_spans()), 2)

    def test_retry_limits_and_permanent_errors(self):
        for status, expected in [(503, 3), (401, 1)]:
            model, memory = offline_client(
                "retry",
                lambda req: httpx.Response(status, json={"error": "simulated"}),
            )
            delays = []
            with self.assertRaises(ModelError):
                with model:
                    retry_chat(model, [], sleep=delays.append)
            self.assertEqual(model.calls, expected)
            self.assertEqual(len(memory.get_finished_spans()), expected + 1)
            self.assertEqual(len(delays), expected - 1)

    def test_keyword_retrieval_and_unknown_topic(self):
        self.assertEqual(
            search_documents("星河笔记如何导出？")[0]["id"], "export"
        )
        self.assertEqual(search_documents("是否支持脑机接口？"), [])
        with self.assertRaises(ValueError):
            search_documents("x" * 201)

    def test_rag_abstains_without_model_call(self):
        def no_http(req):
            self.fail("没有资料时不应该调用模型")

        model, memory = offline_client("rag_unknown", no_http)
        with model:
            result = answer_question(model, "是否支持脑机接口？")
        self.assertTrue(result["insufficient_evidence"])
        self.assertEqual(model.calls, 0)
        self.assertEqual(model.tool_calls, 1)
        self.assertEqual(len(memory.get_finished_spans()), 2)

    def test_rag_context_and_citation_validation(self):
        def handler(req):
            context = json.loads(
                json.loads(req.content)["messages"][-1]["content"]
            )
            self.assertEqual(context["passages"][0]["id"], "export")
            return httpx.Response(
                200,
                json=completion(
                    json.dumps(
                        {
                            "answer": "PDF 与 Markdown",
                            "source_ids": ["export"],
                            "insufficient_evidence": False,
                        }
                    )
                ),
            )

        model, memory = offline_client("rag", handler)
        with model:
            self.assertEqual(
                answer_question(model, "导出格式")["source_ids"], ["export"]
            )
        self.assertEqual(len(memory.get_finished_spans()), 3)
        for ids, insufficient in [
            (["invented"], False),
            ([], False),
            (["export"], True),
        ]:
            with self.assertRaises(ValueError):
                validate_answer(
                    json.dumps(
                        {
                            "answer": "答案",
                            "source_ids": ids,
                            "insufficient_evidence": insufficient,
                        }
                    ),
                    search_documents("导出"),
                )

    def test_session_reopen_isolation_and_conflict(self):
        with TemporaryDirectory() as folder:
            path = Path(folder) / "memory.db"
            store = SessionStore(path)
            _, revision = store.load("alice")
            store.save("alice", [{"role": "user", "content": "海盐"}], revision)
            reopened = SessionStore(path)
            self.assertEqual(reopened.load("alice")[0][0]["content"], "海盐")
            self.assertEqual(reopened.load("bob"), ([], 0))
            with self.assertRaises(SessionConflict):
                reopened.save("alice", [], revision)
            self.assertEqual(reopened.load("alice")[1], 1)

    def test_failed_model_does_not_commit_user_turn(self):
        with TemporaryDirectory() as folder:
            path = Path(folder) / "memory.db"
            model, _ = offline_client(
                "memory_failure", lambda req: httpx.Response(503, json={})
            )
            with self.assertRaises(ModelError):
                with model:
                    ask(model, path, "alice", "不要保存失败轮次")
            self.assertEqual(SessionStore(path).load("alice"), ([], 0))

    def test_knowledge_agent_executes_search_and_passes_result_back(self):
        def handler(req):
            messages = json.loads(req.content)["messages"]
            if messages[-1]["role"] != "tool":
                return httpx.Response(
                    200, json=completion(calls=[search_call()])
                )
            self.assertEqual(messages[-1]["tool_call_id"], "search-1")
            self.assertEqual(
                json.loads(messages[-1]["content"])["passages"][0]["id"],
                "export",
            )
            return httpx.Response(
                200,
                json=completion(
                    json.dumps(
                        {
                            "answer": "PDF 或 Markdown",
                            "source_ids": ["export"],
                            "insufficient_evidence": False,
                        }
                    )
                ),
            )

        model, memory = offline_client("knowledge", handler)
        with model:
            self.assertEqual(
                run_knowledge_agent(model, "导出格式")["source_ids"], ["export"]
            )
        self.assertEqual(len(memory.get_finished_spans()), 4)

    def test_recovery_requires_approval_and_replay_is_idempotent(self):
        with TemporaryDirectory() as folder:
            path = Path(folder) / "jobs.db"
            jobs = LocalJobs(path)
            jobs.plan("job1", "固定内容")
            with self.assertRaises(PermissionError):
                jobs.execute("job1", approved=False)
            self.assertEqual(jobs.ticket_count(), 0)
            first = jobs.execute("job1", approved=True)
            restarted = LocalJobs(path)
            self.assertEqual(restarted.execute("job1", approved=True), first)
            self.assertEqual(restarted.ticket_count(), 1)
            with self.assertRaises(ValueError):
                restarted.plan("job1", "篡改内容")

    def test_crash_before_commit_rolls_back_effect_and_checkpoint(self):
        def crash():
            raise RuntimeError("simulated crash")

        with TemporaryDirectory() as folder:
            jobs = LocalJobs(Path(folder) / "jobs.db")
            jobs.plan("job1", "任务")
            with self.assertRaises(RuntimeError):
                jobs.execute("job1", approved=True, before_commit=crash)
            self.assertEqual(jobs.ticket_count(), 0)
            jobs.execute("job1", approved=True)
            self.assertEqual(jobs.ticket_count(), 1)

    def test_evaluation_does_not_accept_correct_text_without_tool(self):
        score = grade({"a": 17, "b": 23, "expected": 391}, "391", [])
        self.assertTrue(score["answer_correct"])
        self.assertFalse(score["tool_correct"])
        self.assertTrue(all(r["passed"] for r in evaluate()))
        wrong = evaluate(
            cases=[{"id": "wrong_reference", "a": 17, "b": 23, "expected": 999}]
        )
        self.assertFalse(wrong[0]["passed"])


if __name__ == "__main__":
    unittest.main()
