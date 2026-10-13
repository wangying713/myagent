"""离线契约测试：假模型/内存 exporter，不花 token、不向 OpenObserve 写数据。"""

import io
import json
import unittest
from contextlib import redirect_stderr
from unittest.mock import patch

import httpx
from langchain_core.tools import tool
from opentelemetry.sdk.trace.export import SpanExportResult
from opentelemetry.sdk.trace.export.in_memory_span_exporter import (
    InMemorySpanExporter,
)
from opentelemetry.trace import StatusCode

from agent_toolkit.agent_loop import BudgetExceededError, run_agent
from agent_toolkit.llm import DeepSeekClient, ModelError
from agent_toolkit.settings import Settings
from agent_toolkit.telemetry import (
    OpenObserveExporter,
    Telemetry,
    parse_headers,
)
from agent_toolkit.tools import ToolSet
from demos.advanced.advanced_02_evaluation import evaluate
from demos.intermediate.intermediate_01_json import parse_order
from toolbox import multiply


def reply(content="完成", *, calls=None, reason=None):
    message = {"role": "assistant", "content": None if calls else content}
    if calls:
        message["tool_calls"] = calls
    return {
        "id": "test-response",
        "model": "test",
        "choices": [
            {
                "message": message,
                "finish_reason": reason or ("tool_calls" if calls else "stop"),
            }
        ],
        "usage": {
            "prompt_tokens": 12,
            "completion_tokens": 8,
            "total_tokens": 20,
        },
    }


def call(args='{"a":17,"b":23}', name="multiply", ident="call_1"):
    return {
        "id": ident,
        "type": "function",
        "function": {"name": name, "arguments": args},
    }


class LearningTests(unittest.TestCase):
    def setUp(self):
        self.tool_set = ToolSet([multiply])

    def run_agent(self, model, question, **options):
        return run_agent(
            model,
            question,
            system_prompt="你是计算助手。乘法必须用工具。",
            tools=[multiply],
            **options,
        )

    def client(self, handler, **kwargs):
        settings = Settings(
            api_key="test-private-key", secrets=("test-private-key",), **kwargs
        )
        exporter = InMemorySpanExporter()
        telemetry = Telemetry(settings, exporter=exporter)
        return DeepSeekClient(
            "test",
            settings=settings,
            telemetry=telemetry,
            transport=httpx.MockTransport(handler),
        ), exporter

    def test_success_is_one_call_and_one_root(self):
        requests = []
        request_sizes = []

        def handler(request):
            requests.append(json.loads(request.content))
            request_sizes.append(len(request.content))
            return httpx.Response(
                200, json=reply(), headers={"X-Request-ID": "provider-req-42"}
            )

        client, exporter = self.client(handler)
        with client as model:
            model.chat([{"role": "user", "content": "test-private-key"}])
        spans = exporter.get_finished_spans()
        self.assertEqual(len(spans), 2)
        child, root = spans
        self.assertEqual(child.parent.span_id, root.context.span_id)
        self.assertEqual(child.context.trace_id, root.context.trace_id)
        self.assertEqual(child.attributes["gen_ai.usage.input_tokens"], 12)
        self.assertEqual(child.attributes["http.request.method"], "POST")
        self.assertEqual(
            child.attributes["url.full"],
            "https://api.deepseek.com/v1/chat/completions",
        )
        self.assertEqual(
            child.attributes["http.request.body.size"], request_sizes[0]
        )
        self.assertEqual(
            child.attributes["http.response.header.x-request-id"],
            ("provider-req-42",),
        )
        self.assertGreater(child.attributes["http.response.body.size"], 0)
        self.assertNotIn("authorization", child.attributes)
        self.assertEqual(root.attributes["app.total_tokens"], 20)
        self.assertNotIn("test-private-key", child.attributes["app.request"])
        raw_request = json.loads(
            json.loads(child.attributes["gen_ai.input.messages"])[0]["parts"][
                0
            ]["content"]
        )
        self.assertEqual(raw_request["model"], requests[0]["model"])
        self.assertEqual(raw_request["messages"][0]["content"], "[REDACTED]")
        self.assertEqual(raw_request["max_tokens"], requests[0]["max_tokens"])
        self.assertNotIn("test-private-key", json.dumps(raw_request))
        raw_response = json.loads(
            json.loads(child.attributes["gen_ai.output.messages"])[0]["parts"][
                0
            ]["content"]
        )
        self.assertEqual(
            raw_response["choices"][0]["message"]["content"], "完成"
        )
        self.assertEqual(
            requests[0]["messages"][0]["content"], "test-private-key"
        )
        self.assertEqual(requests[0]["thinking"], {"type": "disabled"})

    def test_failures_have_request_and_error_status(self):
        for response in [
            httpx.Response(
                401, json={"error": {"message": "Bearer hidden-key"}}
            ),
            httpx.Response(200, text="not json"),
            httpx.Response(200, json={}),
            httpx.Response(200, json=reply(reason="length")),
        ]:
            with self.subTest(status=response.status_code):
                client, exporter = self.client(lambda request: response)
                with self.assertRaises(ModelError):
                    with client as model:
                        model.chat([{"role": "user", "content": "hi"}])
                spans = exporter.get_finished_spans()
                self.assertEqual(len(spans), 2)
                self.assertTrue(
                    all(s.status.status_code == StatusCode.ERROR for s in spans)
                )
                self.assertIn("app.request", spans[0].attributes)
                self.assertNotIn("hidden-key", str(spans[0].attributes))
                self.assertIn("app.error.message", spans[0].attributes)
                self.assertEqual(
                    spans[0].attributes["app.error.message"],
                    spans[0].status.description,
                )

    def test_timeout_is_traced(self):
        for error_type in (httpx.ConnectTimeout, httpx.ReadTimeout):
            for stream in (False, True):
                with self.subTest(error=error_type.__name__, stream=stream):

                    def timeout(request):
                        raise error_type(
                            "secret should not leak", request=request
                        )

                    client, exporter = self.client(
                        timeout, capture_content=False
                    )
                    with self.assertRaises(ModelError):
                        with client as model:
                            if stream:
                                model.chat_stream([], on_text=lambda text: None)
                            else:
                                model.chat([])
                    for span in exporter.get_finished_spans():
                        self.assertEqual(
                            span.status.status_code, StatusCode.ERROR
                        )
                        self.assertEqual(
                            span.attributes["error.type"], "ModelError"
                        )
                        self.assertIn(
                            error_type.__name__, span.status.description
                        )
                        self.assertEqual(
                            span.attributes["app.error.message"],
                            span.status.description,
                        )
                        self.assertNotIn(
                            "secret should not leak", str(span.attributes)
                        )
                        self.assertNotIn("app.request", span.attributes)

    def test_evaluation_reports_model_failure_reason(self):
        def timeout(request):
            raise httpx.ConnectTimeout(
                "private network detail", request=request
            )

        client, _ = self.client(timeout)
        with patch(
            "demos.advanced.advanced_02_evaluation.DeepSeekClient",
            return_value=client,
        ):
            result = evaluate(
                cases=[{"id": "small", "a": 12, "b": 8, "expected": 96}]
            )[0]
        self.assertFalse(result["passed"])
        self.assertEqual(result["error"], "ModelError")
        self.assertEqual(
            result["error_message"], "DeepSeek 网络请求失败：ConnectTimeout"
        )
        self.assertEqual(result["llm_calls"], 1)
        self.assertEqual(result["tool_calls"], 0)

    def test_untrusted_exception_message_is_not_traced(self):
        client, exporter = self.client(
            lambda request: httpx.Response(200, json=reply())
        )
        with self.assertRaises(RuntimeError):
            with client:
                raise RuntimeError("private tool detail")
        span = exporter.get_finished_spans()[0]
        self.assertEqual(span.status.description, "RuntimeError")
        self.assertNotIn("app.error.message", span.attributes)
        self.assertFalse(span.events)

    def test_tool_loop_links_messages_and_spans(self):
        requests = []

        def handler(request):
            body = json.loads(request.content)
            requests.append(body)
            if len(requests) == 1:
                return httpx.Response(200, json=reply(calls=[call()]))
            self.assertEqual(
                body["messages"][-2]["tool_calls"][0]["id"], "call_1"
            )
            self.assertEqual(body["messages"][-1]["tool_call_id"], "call_1")
            self.assertEqual(
                json.loads(body["messages"][-1]["content"])["result"], 391
            )
            return httpx.Response(200, json=reply("391"))

        client, exporter = self.client(handler)
        with client as model:
            self.assertEqual(self.run_agent(model, "17*23"), "391")
        self.assertEqual(
            client.tool_results,
            [{"request": call(), "response": {"ok": True, "result": 391}}],
        )
        spans = exporter.get_finished_spans()
        self.assertEqual(len(spans), 4)
        root = spans[-1]
        self.assertTrue(
            all(s.parent.span_id == root.context.span_id for s in spans[:-1])
        )
        tool = next(s for s in spans if s.name == "Function: multiply")
        self.assertEqual(
            tool.attributes["gen_ai.operation.name"], "execute_tool"
        )
        self.assertEqual(tool.attributes["app.tool.name"], "multiply")
        self.assertEqual(tool.attributes["app.tool.call.id"], "call_1")
        self.assertNotIn("gen_ai.tool.name", tool.attributes)
        self.assertNotIn("gen_ai.tool.call.id", tool.attributes)
        for field, expected in (
            ("gen_ai.tool.call.arguments", {"a": 17, "b": 23}),
            ("gen_ai.tool.call.result", {"ok": True, "result": 391}),
        ):
            self.assertEqual(json.loads(tool.attributes[field]), expected)
        self.assertEqual(
            json.loads(tool.attributes["app.tool.request"]), call()
        )

    def test_agent_accepts_custom_prompt_and_tools(self):
        prompt = "你是翻译助手，使用 translate 工具把中文翻译成英文。"
        tool_call = call('{"text":"你好"}', name="translate")
        executions = []
        requests = []

        @tool
        def translate(text: str) -> str:
            """把中文翻译成英文。"""
            executions.append(text)
            return "Hello"

        def handler(request):
            body = json.loads(request.content)
            requests.append(body)
            self.assertEqual(
                body["messages"][0], {"role": "system", "content": prompt}
            )
            self.assertEqual(
                [item["function"]["name"] for item in body["tools"]],
                ["translate"],
            )
            if len(requests) == 1:
                self.assertEqual(body["messages"][-1]["content"], "翻译：你好")
                return httpx.Response(200, json=reply(calls=[tool_call]))
            self.assertEqual(
                json.loads(body["messages"][-1]["content"]),
                {"ok": True, "result": "Hello"},
            )
            return httpx.Response(200, json=reply("Hello"))

        client, exporter = self.client(handler)
        with client:
            answer = run_agent(
                client,
                "翻译：你好",
                system_prompt=prompt,
                tools=[translate],
            )
        self.assertEqual(answer, "Hello")
        self.assertEqual(executions, ["你好"])
        self.assertEqual(len(exporter.get_finished_spans()), 4)

    def test_tool_validation_and_budgets(self):
        for tool in [
            call(name="shell"),
            call("bad"),
            call("[]"),
            call('{"a":true,"b":2}'),
            call('{"a":1e999,"b":2}'),
            call('{"a":2,"b":3,"extra":4}'),
            call(json.dumps({"a": 10**400, "b": 2})),
            {"function": "invalid"},
            {"type": "function", "function": {"name": []}},
        ]:
            self.assertFalse(self.tool_set.dispatch(tool)["ok"])
        client, exporter = self.client(
            lambda req: httpx.Response(200, json=reply(calls=[call()]))
        )
        with self.assertRaises(BudgetExceededError):
            with client as model:
                self.run_agent(model, "loop", max_rounds=2)
        self.assertEqual(client.calls, 2)
        self.assertEqual(len(exporter.get_finished_spans()), 5)
        client, exporter = self.client(
            lambda req: httpx.Response(200, json=reply(calls=[call()]))
        )
        with self.assertRaises(BudgetExceededError):
            with client as model:
                self.run_agent(model, "loop", max_tool_calls=0)
        self.assertEqual(len(exporter.get_finished_spans()), 2)

    def test_multiple_tools_and_tool_error(self):
        n = 0

        def handler(req):
            nonlocal n
            n += 1
            if n == 1:
                return httpx.Response(
                    200, json=reply(calls=[call(), call("bad", ident="call_2")])
                )
            messages = json.loads(req.content)["messages"]
            self.assertEqual(
                [x["tool_call_id"] for x in messages[-2:]], ["call_1", "call_2"]
            )
            self.assertFalse(json.loads(messages[-1]["content"])["ok"])
            return httpx.Response(200, json=reply())

        client, exporter = self.client(handler)
        with client as model:
            self.run_agent(model, "two calls")
        tools = [
            s
            for s in exporter.get_finished_spans()
            if s.attributes.get("gen_ai.operation.name") == "execute_tool"
        ]
        self.assertEqual(len(tools), 2)
        self.assertEqual(tools[1].status.status_code, StatusCode.ERROR)
        preview = json.loads(tools[1].attributes["gen_ai.tool.call.result"])
        self.assertEqual(
            preview,
            {"ok": False, "error": "invalid_json"},
        )
        self.assertEqual(
            json.loads(tools[1].attributes["gen_ai.tool.call.arguments"]),
            "bad",
        )
        self.assertEqual(
            [r["response"]["ok"] for r in client.tool_results], [True, False]
        )

    def test_tool_preview_respects_content_controls(self):
        tool_call = call(
            json.dumps({"api_key": "nested-private", "a": 2, "b": 3})
        )
        result = {
            "ok": True,
            "password": "hidden-password",
            "result": "test-private-key",
        }
        for enabled in (True, False):
            with self.subTest(capture_content=enabled):
                client, exporter = self.client(
                    lambda req: self.fail("工具预览不应调用模型"),
                    capture_content=enabled,
                )
                with client:
                    self.assertEqual(
                        client.execute_tool(tool_call, lambda _: result), result
                    )
                attrs = exporter.get_finished_spans()[0].attributes
                for field in (
                    "gen_ai.tool.call.arguments",
                    "gen_ai.tool.call.result",
                ):
                    if enabled:
                        self.assertIn("[REDACTED]", attrs[field])
                        for secret in (
                            "nested-private",
                            "hidden-password",
                            "test-private-key",
                        ):
                            self.assertNotIn(secret, attrs[field])
                    else:
                        self.assertNotIn(field, attrs)
        client, exporter = self.client(
            lambda req: self.fail("工具预览不应调用模型"),
            content_max_chars=256,
        )
        with client:
            client.execute_tool(
                call("x" * 500),
                lambda _: {"ok": False, "error": "x" * 500},
            )
        attrs = exporter.get_finished_spans()[0].attributes
        for field in ("gen_ai.tool.call.arguments", "gen_ai.tool.call.result"):
            self.assertTrue(attrs[field + ".truncated"])
            self.assertTrue(attrs[field].endswith("…[TRUNCATED]"))

    def test_stream_aggregates_chunks_and_final_usage(self):
        chunks = [
            {"choices": [{"delta": {"content": "你"}}]},
            {"choices": [{"delta": {"content": "好"}}]},
            {
                "choices": [{"delta": {}, "finish_reason": "stop"}],
                "usage": {"total_tokens": 8},
            },
        ]
        raw = (
            ": keep-alive\n\n"
            + "".join("data: " + json.dumps(c) + "\n\n" for c in chunks)
            + "data: [DONE]\n\n"
        )
        client, exporter = self.client(
            lambda req: httpx.Response(200, text=raw)
        )
        text = []
        with client as model:
            data = model.chat_stream([], on_text=text.append)
        self.assertEqual(text, ["你", "好"])
        self.assertEqual(data["choices"][0]["message"]["content"], "你好")
        spans = exporter.get_finished_spans()
        self.assertEqual(len(spans), 2)
        self.assertEqual(spans[0].attributes["app.usage.total_tokens"], 8)
        self.assertIn("data: [DONE]", spans[0].attributes["app.response.sse"])

    def test_stream_tools_are_reassembled(self):
        chunks = [
            {
                "choices": [
                    {
                        "delta": {
                            "tool_calls": [
                                {
                                    "index": 0,
                                    "id": "c1",
                                    "function": {
                                        "name": "multiply",
                                        "arguments": '{"a":',
                                    },
                                }
                            ]
                        }
                    }
                ]
            },
            {
                "choices": [
                    {
                        "delta": {
                            "tool_calls": [
                                {
                                    "index": 0,
                                    "function": {"arguments": '2,"b":3}'},
                                }
                            ]
                        },
                        "finish_reason": "tool_calls",
                    }
                ]
            },
            {"choices": [], "usage": {"total_tokens": 10}},
        ]
        raw = (
            "".join("data: " + json.dumps(c) + "\n\n" for c in chunks)
            + "data: [DONE]\n"
        )
        client, exporter = self.client(
            lambda req: httpx.Response(200, text=raw)
        )
        with client as model:
            data = model.chat_stream([], on_text=lambda t: None)
        self.assertEqual(
            self.tool_set.dispatch(
                data["choices"][0]["message"]["tool_calls"][0]
            )["result"],
            6,
        )
        output = json.loads(
            json.loads(
                exporter.get_finished_spans()[0].attributes[
                    "gen_ai.output.messages"
                ]
            )[0]["parts"][0]["content"]
        )
        self.assertEqual(
            output["choices"][0]["message"]["tool_calls"][0]["function"][
                "arguments"
            ],
            '{"a":2,"b":3}',
        )
        self.assertEqual(len(exporter.get_finished_spans()), 2)

    def test_interrupted_stream_is_error_with_partial_response(self):
        client, exporter = self.client(
            lambda req: httpx.Response(
                200, text='data: {"choices":[{"delta":{"content":"半句"}}]}\n'
            )
        )
        with self.assertRaises(ModelError):
            with client as model:
                model.chat_stream([], on_text=lambda t: None)
        span = exporter.get_finished_spans()[0]
        self.assertFalse(span.attributes["app.stream.complete"])
        self.assertIn("半句", span.attributes["app.response"])
        self.assertEqual(span.status.status_code, StatusCode.ERROR)

    def test_redaction_truncation_and_content_off(self):
        settings = Settings(
            api_key="secret", secrets=("known-secret",), content_max_chars=256
        )
        exp = InMemorySpanExporter()
        telemetry = Telemetry(settings, exporter=exp)
        with telemetry.span("test") as span:
            telemetry.payload(
                span,
                "app.request",
                {
                    "password": "hidden",
                    "arguments": '{"api_key":"nested"}',
                    "x": "known-secret",
                },
            )
            telemetry.payload(span, "app.response", "文" * 500)
        telemetry.close()
        attrs = exp.get_finished_spans()[0].attributes
        self.assertNotIn("hidden", attrs["app.request"])
        self.assertNotIn("nested", attrs["app.request"])
        self.assertNotIn("known-secret", attrs["app.request"])
        self.assertTrue(attrs["app.response.truncated"])
        client, exp = self.client(
            lambda req: httpx.Response(200, json=reply()), capture_content=False
        )
        with client as model:
            model.chat([])
        self.assertNotIn("app.request", exp.get_finished_spans()[0].attributes)
        self.assertNotIn(
            "gen_ai.input.messages", exp.get_finished_spans()[0].attributes
        )
        self.assertNotIn(
            "gen_ai.output.messages", exp.get_finished_spans()[0].attributes
        )
        self.assertIn(
            "gen_ai.usage.input_tokens", exp.get_finished_spans()[0].attributes
        )

    def test_export_failure_warns_once_and_does_not_raise(self):
        exporter = OpenObserveExporter("http://localhost/v1/traces", {})
        exporter.client.close()
        exporter.client = httpx.Client(
            transport=httpx.MockTransport(lambda req: httpx.Response(503))
        )
        output = io.StringIO()
        with redirect_stderr(output):
            self.assertEqual(exporter.export([]), SpanExportResult.FAILURE)
            exporter.export([])
        self.assertEqual(output.getvalue().count("[观测]"), 1)
        exporter.shutdown()

    def test_partial_rejection_is_not_success(self):
        from opentelemetry.proto.collector.trace.v1.trace_service_pb2 import (
            ExportTraceServiceResponse,
        )

        ack = ExportTraceServiceResponse()
        ack.partial_success.rejected_spans = 1
        exporter = OpenObserveExporter("http://localhost/v1/traces", {})
        exporter.client.close()
        exporter.client = httpx.Client(
            transport=httpx.MockTransport(
                lambda req: httpx.Response(200, content=ack.SerializeToString())
            )
        )
        with redirect_stderr(io.StringIO()):
            self.assertEqual(exporter.export([]), SpanExportResult.FAILURE)
        exporter.shutdown()

    def test_json_business_validation_and_headers(self):
        self.assertEqual(
            parse_order('{"item":"笔","quantity":3}')["quantity"], 3
        )
        for text in [
            "{}",
            '{"item":"笔","quantity":true}',
            '{"item":"笔","quantity":101}',
        ]:
            with self.assertRaises(ValueError):
                parse_order(text)
        self.assertEqual(
            parse_headers("Authorization=Basic%20abc%3D")["Authorization"],
            "Basic abc=",
        )


if __name__ == "__main__":
    unittest.main()
