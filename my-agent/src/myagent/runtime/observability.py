"""可观测接入层：全项目**只有这里**知道「数据发到哪个平台」。

分层纪律（与 config.py / llm.py 一致）：
    config.py        只有这里读配置文件
    llm.py           只有这里知道用哪家模型
    observability.py 只有这里知道发哪个观测平台   ← 本文件

业务代码的用法只有一行：

    from myagent.runtime.observability import setup
    setup()                       # ⚠️ 必须在 llm.configure() 之后调用

    agent = Agent(...)            # 之后全部自动上报，业务代码零侵入：
    log.info("开始处理")           #   · 日志 → 自动带 trace_id
    await Runner.run(agent, ...)  #   · span 树 → Agent / turn / generation
    httpx.get(...)                #   · HTTP 请求 → method / url / status

配置来源：config.env（同 config.py），支持 OPENOBSERVE_* / OTEL_*，
环境变量优先于文件。缺少凭据就跳过上报，不会报错。
"""

from __future__ import annotations

import base64
import logging
import os
from dataclasses import dataclass
from pathlib import Path

from .config import CONFIG_PATH

_CONFIGURED = False


# ────────────────────────────────────────────────────────────
# 配置读取
# ────────────────────────────────────────────────────────────
@dataclass(frozen=True)
class ObservabilitySettings:
    service_name: str
    oo_traces_endpoint: str
    oo_logs_endpoint: str
    oo_auth: str  # 已 base64 的 Basic 凭据，空串 = 未配置

    @property
    def has_openobserve(self) -> bool:
        return bool(self.oo_auth)


def _read_config_file(path: Path) -> dict[str, str]:
    cfg: dict[str, str] = {}
    if not path.exists():
        return cfg
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        cfg[key.strip()] = value.strip().strip('"').strip("'")
    return cfg


def load_observability_settings(path: Path | None = None) -> ObservabilitySettings:
    cfg = _read_config_file(path or CONFIG_PATH)
    # 环境变量优先（临时覆盖方便调试）
    for key in list(cfg) + ["OPENOBSERVE_USER"]:
        if key.startswith(("OPENOBSERVE_", "OTEL_")) and os.environ.get(key):
            cfg[key] = os.environ[key].strip()

    traces_ep = cfg.get(
        "OPENOBSERVE_ENDPOINT", "http://localhost:5080/api/default/v1/traces"
    )
    user = cfg.get("OPENOBSERVE_USER", "")
    password = cfg.get("OPENOBSERVE_PASSWORD", "")
    auth = (
        base64.b64encode(f"{user}:{password}".encode()).decode() if user and password else ""
    )

    return ObservabilitySettings(
        service_name=cfg.get("OTEL_SERVICE_NAME", "my-agent"),
        oo_traces_endpoint=traces_ep,
        oo_logs_endpoint=traces_ep.replace("/v1/traces", "/v1/logs"),
        oo_auth=auth,
    )


# ────────────────────────────────────────────────────────────
# 各步骤
# ────────────────────────────────────────────────────────────
def _setup_traces(s: ObservabilitySettings):
    """建 TracerProvider 并挂 OpenObserve 的 span exporter。

    ⚠️ OTel 默认给的是 ProxyTracerProvider（占位对象），必须自己建真的，
    否则后面 add_span_processor 会 AttributeError。
    """
    from opentelemetry import trace
    from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
    from opentelemetry.sdk.resources import SERVICE_NAME, Resource
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.sdk.trace.export import BatchSpanProcessor

    provider = trace.get_tracer_provider()
    if not isinstance(provider, TracerProvider):
        # 用 isinstance 而不是 hasattr：既语义准确，也能让类型检查器收窄类型
        provider = TracerProvider(
            resource=Resource.create({SERVICE_NAME: s.service_name})
        )
        trace.set_tracer_provider(provider)

    if s.has_openobserve:
        provider.add_span_processor(
            BatchSpanProcessor(
                OTLPSpanExporter(
                    endpoint=s.oo_traces_endpoint,
                    headers={"Authorization": f"Basic {s.oo_auth}"},
                )
            )
        )
    return provider


def _setup_logs(s: ObservabilitySettings):
    """把标准 logging 桥接到 OTLP。

    收益：日志自动携带当前 span 的 trace_id，平台上「日志 ↔ trace」可互跳。
    """
    from opentelemetry.exporter.otlp.proto.http._log_exporter import OTLPLogExporter
    from opentelemetry.sdk._logs import LoggerProvider, LoggingHandler
    from opentelemetry.sdk._logs.export import BatchLogRecordProcessor
    from opentelemetry.sdk.resources import SERVICE_NAME, Resource

    provider = LoggerProvider(resource=Resource.create({SERVICE_NAME: s.service_name}))
    if s.has_openobserve:
        provider.add_log_record_processor(
            BatchLogRecordProcessor(
                OTLPLogExporter(
                    endpoint=s.oo_logs_endpoint,
                    headers={"Authorization": f"Basic {s.oo_auth}"},
                )
            )
        )
    root = logging.getLogger()
    root.addHandler(LoggingHandler(level=logging.INFO, logger_provider=provider))
    root.setLevel(logging.INFO)
    return provider


def _setup_http() -> None:
    """给 httpx 和 httpx2 都装上 HTTP 埋点。

    ⚠️ 两个库都要，因为它们不是同一个东西：
        httpx     —— 我们其它依赖在用
        httpx2    —— **openai SDK 在用**（openai/_base_client.py 里 import httpx2）
    只 patch httpx 的话，模型请求不产生 HTTP span，平台上就看不到请求地址
    （URL）和状态码 —— 这是实测踩到的坑。

    时机要求：只要在**发起请求之前**完成 patch 即可（实测：先创建 client
    再 patch，请求照样有 span）。
    """
    from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor

    HTTPXClientInstrumentor().instrument()
    _instrument_httpx2()


# 请求体最多记这么多字符：tools 的 JSON Schema 可能很长，别把 span 属性撑爆
_MAX_BODY_CHARS = 16_384


def _instrument_httpx2() -> None:
    """给 httpx2 手写一层埋点（官方没有对应的 instrumentation）。

    做法：把 AsyncClient.send 包一层，开一个 HTTP span 并挂四个属性：
        http.request.method        请求方法
        url.full                   完整请求地址（含 /chat/completions）
        http.response.status_code  状态码
        http.request.body          请求体原文（截断到 _MAX_BODY_CHARS）

    响应体不在这里读 —— 读流会把响应消费掉、影响业务；模型返回的原始内容
    在 OpenInference 的 output.value 属性里已经有了。

    用 start_span 而不是 start_as_current_span：不能把自己的 span 设成
    current，否则会影响日志 trace_id 的归属。

    只包异步客户端（项目走 AsyncOpenAI），同步的 httpx2.Client 用不到。
    """
    import httpx2
    from opentelemetry import trace as otel_trace
    from opentelemetry.trace import Status, StatusCode

    original_send = httpx2.AsyncClient.send
    if getattr(original_send, "_myagent_patched", False):
        return

    async def send_with_span(self, request, *args, **kwargs):
        # tracer 在调用时才取，保证拿到 setup() 之后生效的那个 provider
        tracer = otel_trace.get_tracer("myagent.httpx2")
        span = tracer.start_span(f"HTTP {request.method}")
        span.set_attribute("http.request.method", request.method)
        span.set_attribute("url.full", str(request.url))

        try:
            body = request.content
        except Exception:  # 请求体是流、或已被读过
            body = b""
        if body:
            span.set_attribute(
                "http.request.body", body[:_MAX_BODY_CHARS].decode("utf-8", "replace")
            )

        try:
            response = await original_send(self, request, *args, **kwargs)
        except Exception as exc:
            span.record_exception(exc)
            span.set_status(Status(StatusCode.ERROR, str(exc)))
            span.end()
            raise

        span.set_attribute("http.response.status_code", response.status_code)
        span.end()
        return response

    send_with_span._myagent_patched = True  # type: ignore[attr-defined]
    httpx2.AsyncClient.send = send_with_span


def _detach_sdk_exporter() -> None:
    """摘掉 SDK 默认的 trace exporter。

    它会把数据发往 https://api.openai.com/v1/traces/ingest，
    且 api_key 会 fallback 读环境变量 OPENAI_API_KEY —— 用第三方模型时
    （哪怕环境里放的是别家的 sk- key）会把 prompt 内容发出去。

    ⚠️ 用 set_trace_processors([]) 而不是 set_tracing_disabled(True)：
    OpenInference 的埋点是「挂在 SDK tracing 上的一个 processor」，
    直接 disable 会把上游一起关掉，整条链路静默失效。
    """
    from agents import set_trace_processors

    set_trace_processors([])


def _setup_agent_instrumentation() -> None:
    from openinference.instrumentation.openai_agents import OpenAIAgentsInstrumentor

    OpenAIAgentsInstrumentor().instrument()


# ────────────────────────────────────────────────────────────
# 对外入口
# ────────────────────────────────────────────────────────────
def setup(*, verbose: bool = True) -> None:
    """一次性接入可观测。幂等，重复调用无副作用。

    ⚠️ 必须在 llm.configure() **之后**调用：configure() 会重置 SDK 的
    trace processor，若 setup() 先执行，它装上的 OpenInference processor
    会被覆盖掉，导致整条链路静默失效（平台无数据、也不报错）。

    典型用法：
        configure(settings, console_trace=False)   # 先配模型
        setup()                                    # 再接可观测
    """
    global _CONFIGURED
    if _CONFIGURED:
        return

    s = load_observability_settings()
    provider = _setup_traces(s)
    log_provider = _setup_logs(s)
    _setup_http()
    _detach_sdk_exporter()
    _setup_agent_instrumentation()

    # 存起来，供 flush() 使用
    setup._providers = (provider, log_provider)  # type: ignore[attr-defined]
    _CONFIGURED = True

    if verbose:
        state = (
            "OpenObserve(traces + logs)"
            if s.has_openobserve
            else "未配置平台（数据不会上报）"
        )
        print(f"[observability] 已接入 → {state}")


def flush() -> None:
    """把缓冲区里的数据发出去。

    ⚠️ 短生命周期脚本（跑完就退出的）必须调它，否则批量缓冲的数据会丢。
    长驻服务（CLI/Web）靠后台线程自动发送，不必显式调用。
    """
    for p in getattr(setup, "_providers", ()):  # type: ignore[arg-type]
        if p is not None and hasattr(p, "force_flush"):
            p.force_flush()
        elif p is not None and hasattr(p, "flush"):
            p.flush()


__all__ = ["setup", "flush", "load_observability_settings", "ObservabilitySettings"]
