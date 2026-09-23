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

配置来源：config.env（同 config.py），支持 LANGFUSE_* / OPENOBSERVE_* / OTEL_*，
环境变量优先于文件。缺少哪个平台的凭据就自动跳过哪个，不会报错。
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
    lf_public_key: str
    lf_secret_key: str
    lf_host: str

    @property
    def has_openobserve(self) -> bool:
        return bool(self.oo_auth)

    @property
    def has_langfuse(self) -> bool:
        return bool(self.lf_public_key and self.lf_secret_key)


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
    for key in list(cfg) + ["LANGFUSE_PUBLIC_KEY", "OPENOBSERVE_USER"]:
        if key.startswith(("LANGFUSE_", "OPENOBSERVE_", "OTEL_")) and os.environ.get(key):
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
        lf_public_key=cfg.get("LANGFUSE_PUBLIC_KEY", ""),
        lf_secret_key=cfg.get("LANGFUSE_SECRET_KEY", ""),
        lf_host=cfg.get("LANGFUSE_BASE_URL", "http://localhost:3000"),
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
    """给 httpx 装自动埋点。

    时机要求：只要在**发起请求之前**完成 patch 即可（实测：先创建 client
    再 patch，请求照样有 span）。不需要早于 client 创建。

    注意：openai SDK 3.17 已改用 httpx2，本 patch 对它不生效（httpx2 暂无
    OTel instrumentation），所以 openai 的请求不会生成 HTTP span，
    但能看到 httpx2 自身打的 INFO 日志（含 method / url / status）。
    """
    from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor

    HTTPXClientInstrumentor().instrument()


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


def _setup_langfuse(s: ObservabilitySettings):
    """Langfuse 初始化时自己会往 OTel provider 挂 exporter，只要给 key。"""
    if not s.has_langfuse:
        return None
    from langfuse import get_client

    client = get_client()
    return client if client.auth_check() else None


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
    langfuse = _setup_langfuse(s)
    _setup_agent_instrumentation()

    # 存起来，供 flush() 使用
    setup._providers = (provider, log_provider, langfuse)  # type: ignore[attr-defined]
    _CONFIGURED = True

    if verbose:
        sinks = []
        if s.has_openobserve:
            sinks.append("OpenObserve(traces+logs)")
        if langfuse:
            sinks.append("Langfuse(traces)")
        state = " + ".join(sinks) if sinks else "未配置任何平台（数据不会上报）"
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
