"""观测底座：一次模型请求/工具一个 span，无全局 HTTP 自动埋点。"""

import json
import re
import sys
import time
from contextlib import contextmanager
from urllib.parse import unquote

import httpx
from opentelemetry.exporter.otlp.proto.common.trace_encoder import encode_spans
from opentelemetry.proto.collector.trace.v1.trace_service_pb2 import (
    ExportTraceServiceResponse,
)
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import (
    BatchSpanProcessor,
    SpanExporter,
    SpanExportResult,
)
from opentelemetry.sdk.trace.sampling import ALWAYS_ON
from opentelemetry.trace import NoOpTracerProvider, SpanKind, StatusCode


def parse_headers(value: str) -> dict[str, str]:
    """解析逗号分隔的 key=value 观测 header。

    Raises:
        ValueError: 某项 header 没有等号分隔符时。
    """
    headers = {}
    for item in value.split(","):
        if item.strip():
            key, sep, val = item.partition("=")
            if not sep:
                raise ValueError(
                    "OTEL_EXPORTER_OTLP_HEADERS 格式应为 key=value"
                )
            headers[key.strip()] = unquote(val.strip())
    return headers


class OpenObserveExporter(SpanExporter):
    """标准 OTLP Protobuf；单次发送、短超时，不产生递归上报或重试噪音。"""

    def __init__(self, endpoint: str, headers: dict, timeout: float = 3):
        """创建 OpenObserve exporter。

        Args:
            endpoint: OTLP traces 接收地址。
            headers: HTTP 鉴权和 stream 等请求 header。
            timeout: 单次导出请求的超时秒数。
        """
        self.client = httpx.Client(timeout=timeout, trust_env=False)
        self.endpoint = endpoint
        self.headers = {**headers, "Content-Type": "application/x-protobuf"}
        self.failed_spans = 0
        self.sent_spans = 0
        self._warned = False

    def export(self, spans):
        """将 span 编码为 OTLP Protobuf 并发送一次，不自动重试。"""
        try:
            response = self.client.post(
                self.endpoint,
                headers=self.headers,
                content=encode_spans(spans).SerializeToString(),
            )
            response.raise_for_status()
            ack = ExportTraceServiceResponse()
            ack.ParseFromString(response.content)
            if (
                ack.partial_success.rejected_spans
                or ack.partial_success.error_message
            ):
                raise RuntimeError("partial_success")
        except Exception as exc:
            self.failed_spans += len(spans)
            if not self._warned:
                code = (
                    exc.response.status_code
                    if isinstance(exc, httpx.HTTPStatusError)
                    else type(exc).__name__
                )
                print(
                    f"[观测] OpenObserve 上报失败（{code}），模型运行继续；"
                    "请运行 verify_observe.py。",
                    file=sys.stderr,
                )
                self._warned = True
            return SpanExportResult.FAILURE
        self.sent_spans += len(spans)
        return SpanExportResult.SUCCESS

    def shutdown(self):
        """关闭 exporter 持有的 HTTP 客户端。"""
        self.client.close()


class Telemetry:
    """管理 trace provider、span 生命周期、正文脱敏和导出。"""

    def __init__(self, settings, exporter=None):
        """按 Settings 创建观测器；支持注入 exporter 供离线测试使用。

        Args:
            settings: 服务名、正文采集和 OpenObserve 连接配置。
            exporter: 可选 span exporter；省略时使用 OpenObserveExporter。
        """
        self.capture_content = settings.capture_content
        self.max_chars = settings.content_max_chars
        self.secrets = sorted(
            set(
                (
                    settings.api_key,
                    *settings.secrets,
                    *settings.otel_headers.values(),
                )
            ),
            key=len,
            reverse=True,
        )
        self.exporter = exporter
        self.provider = None
        if settings.telemetry_enabled:
            self.exporter = exporter or OpenObserveExporter(
                settings.traces_endpoint, settings.otel_headers
            )
            # 私有 provider 可重复创建/关闭；不覆盖进程全局 provider。
            self.provider = TracerProvider(
                resource=Resource.create(
                    {"service.name": settings.service_name}
                ),
                sampler=ALWAYS_ON,
            )
            self.provider.add_span_processor(
                BatchSpanProcessor(
                    self.exporter,
                    max_queue_size=256,
                    max_export_batch_size=32,
                    schedule_delay_millis=1000,
                )
            )
            self.tracer = self.provider.get_tracer("agent-learning")
        else:
            self.tracer = NoOpTracerProvider().get_tracer("agent-learning")

    def clean(self, value):
        """递归清理秘密字段和值中的凭据，并返回清理后的对象。"""
        if isinstance(value, dict):
            return {
                str(k): "[REDACTED]"
                if re.search(
                    r"authorization|api[_-]?key|password|secret|"
                    r"access[_-]?token|refresh[_-]?token",
                    str(k),
                    re.I,
                )
                else self.clean(v)
                for k, v in value.items()
            }
        if isinstance(value, (list, tuple)):
            return [self.clean(v) for v in value]
        if isinstance(value, str):
            for secret in self.secrets:
                if secret:
                    value = value.replace(secret, "[REDACTED]")
            value = re.sub(
                r"\b(?:Bearer|Basic)\s+[A-Za-z0-9._~+/=-]+",
                "[REDACTED]",
                value,
                flags=re.I,
            )
            value = re.sub(r"\bsk-[A-Za-z0-9_-]{8,}", "[REDACTED]", value)
            # 保留 JSON 原始排版，同时清理敏感字段值。
            if value.lstrip().startswith(("{", "[")):
                value = re.sub(
                    r'("(?:authorization|api[_-]?key|password|secret|'
                    r'access[_-]?token|refresh[_-]?token)"\s*:\s*)'
                    r'("(?:\\.|[^"\\])*"|[^,}\]\s]+)',
                    r'\1"[REDACTED]"',
                    value,
                    flags=re.I,
                )
        return value

    def payload(self, span, name: str, value):
        """将值编码为 JSON 属性，并按配置脱敏和截断。"""
        if not self.capture_content:
            return
        text = json.dumps(self.clean(value), ensure_ascii=False, default=str)
        span.set_attribute(name + ".chars", len(text))
        truncated = len(text) > self.max_chars
        span.set_attribute(name + ".truncated", truncated)
        span.set_attribute(
            name, text[: self.max_chars] + ("…[TRUNCATED]" if truncated else "")
        )

    def raw_payload(self, span, name: str, value):
        """将原始 JSON 作为文本记录，同时按配置脱敏和截断。"""
        if not self.capture_content:
            return
        cleaned = (
            self.clean_json_text(value)
            if isinstance(value, str)
            else self.clean(value)
        )
        text = (
            cleaned
            if isinstance(cleaned, str)
            else json.dumps(
                cleaned,
                ensure_ascii=False,
                separators=(",", ":"),
                default=str,
            )
        )
        span.set_attribute(name + ".chars", len(text))
        truncated = len(text) > self.max_chars
        span.set_attribute(name + ".truncated", truncated)
        span.set_attribute(
            name, text[: self.max_chars] + ("…[TRUNCATED]" if truncated else "")
        )

    def clean_json_text(self, value: str) -> str:
        """递归脱敏 JSON 文本并紧凑序列化，保留字符串字段的 JSON 内容。"""
        try:
            decoded = json.loads(value)
        except (ValueError, RecursionError):
            return self.clean(value)
        return json.dumps(
            self.clean(decoded),
            ensure_ascii=False,
            separators=(",", ":"),
            default=str,
        )

    @contextmanager
    def span(self, name: str, *, kind=SpanKind.INTERNAL, attributes=None):
        """创建一个自动计时的 span，并将抛出的异常标记为错误后继续抛出。"""
        # 不自动记录异常正文/堆栈，避免 header 或工具秘密进入 trace。
        with self.tracer.start_as_current_span(
            name,
            kind=kind,
            attributes=attributes,
            record_exception=False,
            set_status_on_exception=False,
        ) as span:
            started = time.perf_counter()
            try:
                yield span
            except BaseException as exc:
                span.set_attribute("error.type", type(exc).__name__)
                span.set_status(StatusCode.ERROR, type(exc).__name__)
                raise
            finally:
                span.set_attribute(
                    "app.elapsed_ms",
                    round((time.perf_counter() - started) * 1000, 2),
                )

    def close(self):
        """刷新并关闭 provider，确保已排队的 span 尽量完成导出。"""
        if self.provider is not None:
            if not self.provider.force_flush(timeout_millis=5000):
                print(
                    "[观测] 刷出队列超时，记录可能不完整；请按 trace_id 查询确认。",
                    file=sys.stderr,
                )
            self.provider.shutdown()
            self.provider = None
