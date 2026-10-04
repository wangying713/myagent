"""可观测接入层：全项目**只有这里**知道「数据发到哪个平台」。

分层纪律（与 config.py / llm.py 一致）：
    config.py        只有这里读配置文件
    llm.py           只有这里知道用哪家模型
    observability.py 只有这里知道发哪个观测平台   ← 本文件

业务代码的用法只有一行：

    from myagent.runtime.observability import setup
    setup()                       # 通常由 runtime.lab 统一调用

    agent = Agent(...)            # 之后全部自动上报，业务代码零侵入：
    log.info("开始处理")           #   · 日志 → 自动带 trace_id
    await Runner.run(agent, ...)  #   · span 树 → Agent / turn / generation
    # HTTP 基础信息及 JSON 正文默认采集；debug_http=True 额外增加响应请求 ID

配置来源：config.env（同 config.py），支持 OPENOBSERVE_* / OTEL_*，
环境变量优先于文件。缺少凭据就跳过上报，不会报错。
"""

from __future__ import annotations

import base64
import logging
import os
import sys
from dataclasses import dataclass
from pathlib import Path

from .config import CONFIG_PATH, _read_env_file

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


def load_observability_settings(path: Path | None = None) -> ObservabilitySettings:
    cfg = _read_env_file(path or CONFIG_PATH)
    # 环境变量优先（临时覆盖方便调试）
    for key in set(cfg) | {"OPENOBSERVE_USER", "OPENOBSERVE_PASSWORD", "OPENOBSERVE_ENDPOINT", "OPENOBSERVE_LOGS_ENDPOINT", "OTEL_SERVICE_NAME"}:
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
        oo_logs_endpoint=cfg.get("OPENOBSERVE_LOGS_ENDPOINT", traces_ep.replace("/v1/traces", "/v1/logs")),
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

    from .span_export import DisplaySpanExporter
    from .telemetry import DemoSpanProcessor
    provider.add_span_processor(DemoSpanProcessor())

    if s.has_openobserve:
        provider.add_span_processor(
            BatchSpanProcessor(
                DisplaySpanExporter(OTLPSpanExporter(
                    endpoint=s.oo_traces_endpoint,
                    timeout=5,
                    headers={"Authorization": f"Basic {s.oo_auth}"},
                ))
            )
        )
    return provider


def _setup_logs(s: ObservabilitySettings):
    """把标准 logging 桥接到 OTLP。

    收益：日志自动携带当前 span 的 trace_id，平台上「日志 ↔ trace」可互跳。
    """
    from opentelemetry.exporter.otlp.proto.http._log_exporter import OTLPLogExporter
    from opentelemetry.sdk._logs import LoggerProvider
    from opentelemetry.instrumentation.logging.handler import LoggingHandler
    from opentelemetry.sdk._logs.export import BatchLogRecordProcessor
    from opentelemetry.sdk.resources import SERVICE_NAME, Resource

    provider = LoggerProvider(resource=Resource.create({SERVICE_NAME: s.service_name}))
    if s.has_openobserve:
        provider.add_log_record_processor(
            BatchLogRecordProcessor(
                OTLPLogExporter(
                    endpoint=s.oo_logs_endpoint,
                    timeout=5,
                    headers={"Authorization": f"Basic {s.oo_auth}"},
                )
            )
        )
    root = logging.getLogger()
    handler = LoggingHandler(level=logging.INFO, logger_provider=provider)
    # 上报失败产生的内部日志不能再次上报，否则服务离线时会不断重试。
    from .telemetry import DemoLogFilter
    handler.addFilter(DemoLogFilter())
    root.addHandler(handler)
    root.setLevel(logging.INFO)
    return provider


def _setup_http(*, debug_http: bool = False) -> None:
    from .http_telemetry import instrument_http

    instrument_http(debug_http=debug_http)


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
    from .agent_io import instrument_agent_io
    instrument_agent_io()
    from .request_causality import instrument_tool_context
    instrument_tool_context()
    from openinference.instrumentation.openai_agents import OpenAIAgentsInstrumentor

    OpenAIAgentsInstrumentor().instrument()
    from agents import set_trace_processors
    from openinference.instrumentation import OITracer, TraceConfig
    from opentelemetry import trace
    from .compact_tracing import CompactTracingProcessor

    set_trace_processors([CompactTracingProcessor(
        OITracer(trace.get_tracer("myagent.agents"), config=TraceConfig())
    )])


# ────────────────────────────────────────────────────────────
# 对外入口
# ────────────────────────────────────────────────────────────
def setup(*, verbose: bool = True, debug_http: bool = False) -> None:
    """一次性接入可观测。幂等，重复调用无副作用。

    默认 configure() 会重置 SDK 的
    trace processor，若 setup() 先执行，它装上的 OpenInference processor
    会被覆盖掉。lab 使用 reset_tracing=False 保留处理器。

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
    _setup_http(debug_http=debug_http)
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
            if p.force_flush(timeout_millis=6000) is False:
                print("[observability] 刷新超时，部分数据可能尚未送达", file=sys.stderr)
        elif p is not None and hasattr(p, "flush"):
            p.flush()


__all__ = ["setup", "flush", "load_observability_settings", "ObservabilitySettings"]
