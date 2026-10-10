"""直接 HTTP 调 DeepSeek；业务只管 messages/tools，观测和生命周期在这里。"""

import json
from copy import deepcopy
from typing import Callable

import httpx
from opentelemetry.trace import SpanKind, StatusCode

from settings import Settings
from telemetry import Telemetry


class ModelError(RuntimeError):
    """表示模型请求、响应或完成状态不符合预期的错误。"""

    def __init__(self, message, *, status_code=None):
        """保存人类可读的错误信息，以及可选的 HTTP 状态码。"""
        super().__init__(message)
        self.status_code = status_code


def message_of(data: dict) -> dict:
    """从 Chat Completions 响应中取出 assistant 消息；结构无效时抛出 ModelError。"""
    try:
        message = data["choices"][0]["message"]
        if not isinstance(message, dict) or message.get("role") != "assistant":
            raise ValueError
        return message
    except (KeyError, IndexError, TypeError, ValueError):
        raise ModelError("响应缺少有效的 assistant 消息") from None


def final_text(data: dict) -> str:
    """只返回正常结束的纯文本回答；工具调用或不完整回答不算最终答案。"""
    message = message_of(data)
    if (
        data["choices"][0].get("finish_reason") != "stop"
        or not isinstance(message.get("content"), str)
        or not message["content"].strip()
        or message.get("tool_calls")
    ):
        raise ModelError(
            "尚未得到完整文本；检查 finish_reason、tool_calls 和 max_tokens"
        )
    return message["content"]


def serialize_json(value: dict) -> str:
    """将字典编码为紧凑 JSON，供 HTTP 请求和 OpenObserve 原文记录共同使用。"""
    return json.dumps(
        value,
        ensure_ascii=False,
        separators=(",", ":"),
        allow_nan=False,
    )


def json_preview_message(raw_json: str, role: str) -> list[dict]:
    """把完整 JSON 包成普通文本消息，避免 OpenObserve 预览把字段拆开或省略。"""
    return [{"role": role, "parts": [{"type": "text", "content": raw_json}]}]


class DeepSeekClient:
    """直接调用 DeepSeek HTTP API，并统一管理请求和 OpenObserve 观测。

    Attributes:
        settings: DeepSeek 与观测配置。
        telemetry: 负责创建 span 并导出观测数据的对象。
        http: 发送 API 请求的 HTTP 客户端。
        lesson: 标记本次运行来源的课程名称。
        calls: 本次运行的模型请求次数。
        tool_calls: 本次运行的工具执行次数。
        tool_results: 实际执行的工具请求与结果，供业务校验使用。
        total_tokens: 模型响应报告的累计 token 数。
        trace_id: 本次运行的 OpenTelemetry trace ID。
    """

    def __init__(
        self,
        lesson: str,
        *,
        settings=None,
        telemetry=None,
        transport=None,
    ):
        """创建 DeepSeek 客户端。

        Args:
            lesson: 写入 trace 的课程或示例名称。
            settings: 可选配置；省略时从环境变量和 .env 读取。
            telemetry: 可选观测对象；省略时根据 settings 创建。
            transport: 可选 HTTP transport，主要用于离线测试。

        Raises:
            ValueError: 未配置有效的 DeepSeek API key 时。
        """
        self.settings = settings or Settings.from_env()
        missing_api_key = not self.settings.api_key
        placeholder_api_key = self.settings.api_key == "put-your-key-here"
        if missing_api_key or placeholder_api_key:
            raise ValueError("请先在 agent-learning/.env 设置 DEEPSEEK_API_KEY")
        self.telemetry = telemetry or Telemetry(self.settings)
        self.http = httpx.Client(
            timeout=httpx.Timeout(self.settings.timeout, connect=10),
            transport=transport,
            trust_env=False,
        )
        self.lesson = lesson
        self.calls = 0
        self.tool_calls = 0
        self.tool_results = []
        self.total_tokens = 0
        self.trace_id = "disabled"
        self._active = False
        self._used = False

    def __enter__(self):
        """开始一次 Agent 运行并创建根 trace；客户端只能进入 with 块一次。"""
        if self._used:
            raise RuntimeError("每次运行请创建一个新的 DeepSeekClient")
        self._used = True
        self._run = self.telemetry.span(
            "agent.run", attributes={"app.lesson": self.lesson}
        )
        self._root = self._run.__enter__()
        self.trace_id = format(self._root.get_span_context().trace_id, "032x")
        self._active = True
        return self

    def __exit__(self, typ, value, tb):
        """结束根 trace，写入调用统计，并关闭 HTTP 和观测资源。"""
        self._root.set_attribute("app.llm_calls", self.calls)
        self._root.set_attribute("app.tool_calls", self.tool_calls)
        self._root.set_attribute("app.total_tokens", self.total_tokens)
        try:
            self._run.__exit__(typ, value, tb)
        finally:
            self._active = False
            self.http.close()
            self.telemetry.close()

    def _body(self, messages, options):
        """组装请求 JSON；options 中同名参数会覆盖这里的默认值。"""
        if not self._active:
            raise RuntimeError("请在 with DeepSeekClient(...) as model: 中调用")
        return {
            "model": self.settings.model,
            "messages": messages,
            "thinking": {"type": "disabled"},
            "max_tokens": 512,
            **options,
        }

    def _summary(self, span, data):
        """检查模型响应的用量和结束状态，并把可查询的摘要写入模型 span。"""
        if not isinstance(data, dict):
            raise ModelError("模型响应不是 JSON object")
        usage = data.get("usage") or {}
        invalid_usage = not isinstance(usage, dict) or any(
            type(v) is not int or v < 0
            for k, v in usage.items()
            if k
            in {
                "prompt_tokens",
                "completion_tokens",
                "total_tokens",
                "prompt_cache_hit_tokens",
                "prompt_cache_miss_tokens",
            }
        )
        if invalid_usage:
            raise ModelError("usage 的 token 用量必须是非负整数")
        for source, target in [
            ("prompt_tokens", "input_tokens"),
            ("completion_tokens", "output_tokens"),
        ]:
            if isinstance(usage.get(source), int):
                span.set_attribute("gen_ai.usage." + target, usage[source])
        for key in (
            "total_tokens",
            "prompt_cache_hit_tokens",
            "prompt_cache_miss_tokens",
        ):
            if isinstance(usage.get(key), int):
                span.set_attribute("app.usage." + key, usage[key])
        self.total_tokens += usage.get("total_tokens", 0) or 0
        if data.get("id"):
            response_id = str(self.telemetry.clean(data["id"]))[:256]
            span.set_attribute(
                "gen_ai.response.id",
                response_id,
            )
        if data.get("model"):
            response_model = str(self.telemetry.clean(data["model"]))[:128]
            span.set_attribute(
                "gen_ai.response.model",
                response_model,
            )
        message = message_of(data)
        reason = data["choices"][0].get("finish_reason")
        span.set_attribute(
            "gen_ai.response.finish_reasons",
            [str(self.telemetry.clean(reason or "unknown"))[:128]],
        )
        if not isinstance(reason, str):
            raise ModelError("finish_reason 必须是字符串")
        if reason not in {"stop", "tool_calls"}:
            raise ModelError(f"模型未正常完成，finish_reason={reason}")
        if reason == "tool_calls":
            calls = message.get("tool_calls")
            if (
                not isinstance(calls, list)
                or not calls
                or not all(isinstance(c, dict) for c in calls)
            ):
                raise ModelError("tool_calls 必须是非空对象列表")
        if reason == "stop":
            final_text(data)

    def _request_span(self, body):
        """为一次模型 HTTP 调用建立子 span，并记录模型、课程和调用序号。"""
        self.calls += 1
        return self.telemetry.span(
            "chat " + body["model"],
            kind=SpanKind.CLIENT,
            attributes={
                "gen_ai.operation.name": "chat",
                "gen_ai.provider.name": "deepseek",
                "gen_ai.request.model": body["model"],
                "app.llm_call": self.calls,
                "app.lesson": self.lesson,
                "app.stream": body["stream"],
            },
        )

    def _request_metadata(self, span, raw_request):
        """把 HTTP 方法、目标地址和请求体大小写入 span 属性。"""
        url = self.settings.base_url + "/chat/completions"
        span.set_attribute("http.request.method", "POST")
        span.set_attribute("url.full", url)
        span.set_attribute("server.address", httpx.URL(url).host)
        span.set_attribute(
            "http.request.header.content-type",
            ["application/json"],
        )
        span.set_attribute(
            "http.request.body.size",
            len(raw_request.encode("utf-8")),
        )

    def _response_metadata(self, span, response, body_size):
        """记录响应状态、大小、内容类型和少量安全的服务端限流信息。"""
        span.set_attribute("http.response.status_code", response.status_code)
        span.set_attribute("http.response.body.size", body_size)
        content_type = response.headers.get("content-type")
        if content_type:
            span.set_attribute(
                "http.response.header.content-type",
                [self.telemetry.clean(content_type)],
            )
        # 只记录请求关联和限流信息，不记录任意响应头或认证信息。
        for name in (
            "x-request-id",
            "request-id",
            "retry-after",
            "x-ratelimit-limit-requests",
            "x-ratelimit-remaining-requests",
            "x-ratelimit-reset-requests",
            "x-ratelimit-limit-tokens",
            "x-ratelimit-remaining-tokens",
            "x-ratelimit-reset-tokens",
        ):
            value = response.headers.get(name)
            if value:
                safe_value = self.telemetry.clean(value)
                span.set_attribute(
                    "http.response.header." + name,
                    [safe_value],
                )

    def chat(self, messages: list[dict], **options) -> dict:
        """发送一次非流式对话请求，并返回模型响应字典。

        请求和响应会交给观测层记录；敏感内容可能被脱敏，长内容可能被截断。

        Args:
            messages: 按 OpenAI Chat Completions 格式组织的角色消息列表。
            **options: 要附加到 API 请求中的选项，例如 thinking、max_tokens。

        Returns:
            DeepSeek 返回的 JSON 字典。

        Raises:
            ValueError: 传入 stream=True 时；流式请求应使用 chat_stream。
            ModelError: 网络请求失败，或响应格式/结束状态不符合预期时。
        """
        if options.get("stream"):
            raise ValueError("流式调用请使用 chat_stream()")
        body = self._body(messages, {**options, "stream": False})
        raw_request = serialize_json(body)
        with self._request_span(body) as span:
            self._request_metadata(span, raw_request)
            self.telemetry.raw_payload(span, "app.request", raw_request)
            safe_request = self.telemetry.clean_json_text(raw_request)
            self.telemetry.raw_payload(
                span,
                "gen_ai.input.messages",
                json_preview_message(safe_request, "user"),
            )
            try:
                response = self.http.post(
                    self.settings.base_url + "/chat/completions",
                    content=raw_request.encode("utf-8"),
                    headers={
                        "Authorization": "Bearer " + self.settings.api_key,
                        "Content-Type": "application/json",
                    },
                )
                self._response_metadata(span, response, len(response.content))
                raw_response = response.text
                try:
                    data = json.loads(raw_response)
                except ValueError:
                    self.telemetry.raw_payload(
                        span,
                        "app.response",
                        raw_response,
                    )
                    response.raise_for_status()
                    raise ModelError("模型返回的不是 JSON") from None
                self.telemetry.raw_payload(span, "app.response", raw_response)
                response.raise_for_status()
                self._summary(span, data)
                safe_response = self.telemetry.clean_json_text(raw_response)
                self.telemetry.raw_payload(
                    span,
                    "gen_ai.output.messages",
                    json_preview_message(safe_response, "assistant"),
                )
                return data
            except httpx.HTTPStatusError as exc:
                raise ModelError(
                    f"DeepSeek HTTP {exc.response.status_code}；详情见 trace",
                    status_code=exc.response.status_code,
                ) from None
            except httpx.RequestError as exc:
                raise ModelError(
                    f"DeepSeek 网络请求失败：{type(exc).__name__}"
                ) from None

    def chat_stream(
        self, messages: list[dict], *, on_text: Callable[[str], None], **options
    ) -> dict:
        """读取 SSE 流并重组成模型响应字典，每次请求只上报一个模型 span。

        敏感内容可能被脱敏，长内容可能被截断后再写入观测数据。

        Args:
            messages: 按 OpenAI Chat Completions 格式组织的角色消息列表。
            on_text: 收到一段最终回答文本时调用的函数，常用于实时显示。
            **options: 要附加到 API 请求中的选项，例如 thinking、max_tokens。

        Returns:
            重组后的模型响应 JSON 字典。reasoning_content 和工具参数也会保留。

        Raises:
            ModelError: 网络请求失败，流中断，或响应格式/结束状态不符合预期时。
        """
        stream_options = {
            **options,
            "stream": True,
            "stream_options": {"include_usage": True},
        }
        body = self._body(messages, stream_options)
        raw_request = serialize_json(body)
        data = {
            "choices": [
                {
                    "message": {"role": "assistant", "content": ""},
                    "finish_reason": None,
                }
            ]
        }
        message = data["choices"][0]["message"]
        tool_parts = {}
        done = False
        stream_lines = []
        with self._request_span(body) as span:
            self._request_metadata(span, raw_request)
            self.telemetry.raw_payload(span, "app.request", raw_request)
            safe_request = self.telemetry.clean_json_text(raw_request)
            self.telemetry.raw_payload(
                span,
                "gen_ai.input.messages",
                json_preview_message(safe_request, "user"),
            )
            try:
                with self.http.stream(
                    "POST",
                    self.settings.base_url + "/chat/completions",
                    content=raw_request.encode("utf-8"),
                    headers={
                        "Authorization": "Bearer " + self.settings.api_key,
                        "Content-Type": "application/json",
                    },
                ) as response:
                    span.set_attribute(
                        "http.response.status_code", response.status_code
                    )
                    if response.is_error:
                        response.read()
                        self._response_metadata(
                            span,
                            response,
                            len(response.content),
                        )
                        self.telemetry.raw_payload(
                            span, "app.error_response", response.text
                        )
                    response.raise_for_status()
                    for line in response.iter_lines():
                        stream_lines.append(line)
                        if not line.startswith("data:"):
                            continue  # 空行/注释/保活不是模型回复。
                        raw = line[5:].strip()
                        if raw == "[DONE]":
                            done = True
                            break
                        chunk = json.loads(raw)
                        for key in ("id", "model", "usage"):
                            if chunk.get(key) is not None:
                                data[key] = chunk[key]
                        for choice in chunk.get("choices", []):
                            delta = choice.get("delta") or {}
                            for key in ("content", "reasoning_content"):
                                if delta.get(key):
                                    previous = message.get(key, "")
                                    message[key] = previous + delta[key]
                                    if key == "content":
                                        on_text(delta[key])
                            for call in delta.get("tool_calls") or []:
                                part = tool_parts.setdefault(
                                    call["index"],
                                    {
                                        "id": "",
                                        "type": "function",
                                        "function": {
                                            "name": "",
                                            "arguments": "",
                                        },
                                    },
                                )
                                if call.get("id"):
                                    part["id"] = call["id"]
                                for key in ("name", "arguments"):
                                    part["function"][key] += (
                                        call.get("function") or {}
                                    ).get(key) or ""
                            if choice.get("finish_reason"):
                                data["choices"][0]["finish_reason"] = choice[
                                    "finish_reason"
                                ]
                    self._response_metadata(
                        span, response, response.num_bytes_downloaded
                    )
                if not done:
                    raise ModelError("流式响应在 [DONE] 前中断")
                if tool_parts:
                    message["tool_calls"] = [
                        tool_parts[index] for index in sorted(tool_parts)
                    ]
                self._summary(span, data)
                raw_response = serialize_json(data)
                safe_response = self.telemetry.clean_json_text(raw_response)
                self.telemetry.raw_payload(
                    span,
                    "gen_ai.output.messages",
                    json_preview_message(safe_response, "assistant"),
                )
                return data
            except httpx.HTTPStatusError as exc:
                raise ModelError(
                    f"DeepSeek HTTP {exc.response.status_code}；详情见 trace",
                    status_code=exc.response.status_code,
                ) from None
            except httpx.RequestError as exc:
                raise ModelError(
                    f"DeepSeek 流式网络错误：{type(exc).__name__}"
                ) from None
            finally:
                if tool_parts:
                    message["tool_calls"] = [
                        tool_parts[index] for index in sorted(tool_parts)
                    ]
                span.set_attribute("app.stream.complete", done)
                self.telemetry.raw_payload(
                    span,
                    "app.response",
                    serialize_json(data),
                )
                self.telemetry.raw_payload(
                    span, "app.response.sse", "\n".join(stream_lines)
                )

    def execute_tool(
        self,
        call: dict,
        dispatch: Callable[[dict], dict],
    ) -> dict:
        """观测一次工具分发，并返回 dispatch 的结果。

        Args:
            call: 模型提出的工具调用对象。
            dispatch: 负责白名单检查、参数校验和执行的应用函数。

        Returns:
            dispatch 返回的工具结果。

        Raises:
            RuntimeError: 未在 DeepSeekClient 的 with 块中调用时。
        """
        if not self._active:
            raise RuntimeError("工具也必须在 DeepSeekClient 的 with 块内执行")
        self.tool_calls += 1
        function = call.get("function")
        if isinstance(function, dict):
            name = function.get("name", "unknown")
        else:
            name = "unknown"
        with self.telemetry.span(
            "execute_tool",
            attributes={
                "gen_ai.operation.name": "execute_tool",
                "gen_ai.tool.name": str(self.telemetry.clean(name))[:128],
                "gen_ai.tool.call.id": str(
                    self.telemetry.clean(call.get("id", "unknown"))
                )[:256],
                "app.llm_call": self.calls,
            },
        ) as span:
            self.telemetry.payload(span, "app.tool.request", call)
            result = dispatch(call)
            self.tool_results.append(
                {"request": deepcopy(call), "response": deepcopy(result)}
            )
            self.telemetry.payload(span, "app.tool.response", result)
            if not result.get("ok", False):
                span.set_status(StatusCode.ERROR, "ToolError")
                span.set_attribute("error.type", "ToolError")
            return result
