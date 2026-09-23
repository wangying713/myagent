"""hello_world + 双平台可观测（Langfuse + OpenObserve）

────────────────────────────────────────────────────────────
核心思路：一次埋点，两个平台
────────────────────────────────────────────────────────────
Langfuse SDK 本身就是基于 OpenTelemetry 实现的，所以链路是：

    OpenAI Agents SDK
        │
        │ ① OpenInference instrumentor 把 Agent 的每一步转成 OTel Span
        ▼
    OTel TracerProvider（同一个）
        ├──► ② Langfuse 注册的 OTLP Exporter → localhost:3000
        └──► ③ 我们额外挂的 OTLP Exporter  → localhost:5080 (OpenObserve)

即：**业务代码里没有任何 Langfuse / OpenObserve 的调用**，
两个平台都是靠"挂在 OTel 上的导出器"拿到数据的。
这就是可观测性里说的「埋点与后端解耦」——
以后要加第三个平台，只需再加一个 Exporter，业务代码不动。

────────────────────────────────────────────────────────────
运行
────────────────────────────────────────────────────────────
    # 需要 Langfuse 的 key（在 Langfuse UI → Settings → API Keys 创建）
    export LANGFUSE_PUBLIC_KEY="pk-lf-..."
    export LANGFUSE_SECRET_KEY="sk-lf-..."
    export LANGFUSE_BASE_URL="http://localhost:3000"

    # OpenObserve（本地默认账号，密码在 docker-stack/openobserve 的 compose 注释里）
    export OPENOBSERVE_USER="..."
    export OPENOBSERVE_PASSWORD="..."

    uv run python experiments/hello_world_traced.py

没配 Langfuse key 也能跑：会自动跳过 Langfuse，只发 OpenObserve + 控制台。
"""

from __future__ import annotations

import asyncio
import base64
import logging
import os
import sys
from pathlib import Path

# 让脚本能 import 项目内的 myagent 包
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from agents import (
    Agent,
    Runner,
    custom_span,
    set_default_openai_api,
    set_default_openai_client,
    set_trace_processors,
    trace,
)
from openai import AsyncOpenAI

from myagent.runtime.config import Settings, load_settings

CONFIG_ENV = Path(__file__).resolve().parents[1] / "config.env"


def load_observability_env() -> None:
    """把 config.env 里的可观测配置灌进环境变量。

    用 setdefault：命令行显式传的（临时覆盖）优先级更高。
    """
    if not CONFIG_ENV.exists():
        return
    for raw in CONFIG_ENV.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        key, value = key.strip(), value.strip().strip('"').strip("'")
        if key.startswith(("LANGFUSE_", "OPENOBSERVE_")) and value:
            os.environ.setdefault(key, value)


# ────────────────────────────────────────────────────────────
# 1) 模型接入（与项目其它地方完全一致：只认 config.env）
# ────────────────────────────────────────────────────────────
def setup_model() -> Settings:
    settings = load_settings()
    client = AsyncOpenAI(base_url=settings.base_url, api_key=settings.api_key)
    set_default_openai_client(client, use_for_tracing=False)
    # SDK 默认走 Responses API，DeepSeek 只支持 Chat Completions
    set_default_openai_api("chat_completions")
    # ⚠️ SDK 没有「全局默认模型」这类接口，模型名必须在 Agent 上显式传，
    #    否则会退回 SDK 自带默认（gpt-*），DeepSeek 直接报 400。
    return settings


def detach_default_trace_exporter() -> None:
    """摘掉 SDK 默认的 trace exporter（它会把数据发往 api.openai.com），
    但**保留 SDK 的 tracing 机制本身**。

    为什么要保留机制：OpenInference 的接入原理就是「往 SDK 的 tracing 上
    挂一个 TracingProcessor」，它消费的是 SDK 自己产生的 span。
    两者不是两套并行埋点，而是**上下游关系**：

        SDK Tracing（产生 span）
            ├── 默认 BatchTraceProcessor  → api.openai.com   ← 要摘掉的就是它
            └── OpenInference 的 processor → OTel → 你的平台  ← 要留下的

    ⚠️ 千万不要图省事写 set_tracing_disabled(True)：
       那会把上游一起关掉，OpenInference 再也收不到 span，
       整条 OTel 链路静默断掉——平台里不再有新数据，且**不会报任何错**。
    """
    set_trace_processors([])


# ────────────────────────────────────────────────────────────
# 2) 埋点：把 Agent 的每一步变成 OTel Span
# ────────────────────────────────────────────────────────────
def setup_instrumentation():
    from openinference.instrumentation.openai_agents import OpenAIAgentsInstrumentor

    OpenAIAgentsInstrumentor().instrument()


# ────────────────────────────────────────────────────────────
# 3) 导出器 A：Langfuse
#    Langfuse SDK 初始化时会自己往 OTel provider 上挂一个 Exporter，
#    我们不需要手动传 endpoint，只要给 key 和 host。
# ────────────────────────────────────────────────────────────
def setup_langfuse():
    pk = os.environ.get("LANGFUSE_PUBLIC_KEY", "").strip()
    sk = os.environ.get("LANGFUSE_SECRET_KEY", "").strip()
    host = os.environ.get("LANGFUSE_BASE_URL", "http://localhost:3000").strip()

    if not pk or not sk:
        print("⚠️  未配置 LANGFUSE_PUBLIC_KEY / SECRET_KEY → 跳过 Langfuse")
        print("    Langfuse UI → Settings → API Keys → Create new API key\n")
        return None

    from langfuse import get_client

    langfuse = get_client()
    ok = langfuse.auth_check()
    print(f"{'✓' if ok else '✗'} Langfuse 认证{'通过' if ok else '失败（检查 key 与 host）'}  host={host}")
    return langfuse if ok else None


# ────────────────────────────────────────────────────────────
# 4) 导出器 B：OpenObserve
#    手动往同一个 TracerProvider 上再挂一个 OTLP Exporter。
# ────────────────────────────────────────────────────────────
def setup_openobserve():
    endpoint = os.environ.get(
        "OPENOBSERVE_ENDPOINT", "http://localhost:5080/api/default/v1/traces"
    )
    user = os.environ.get("OPENOBSERVE_USER", "").strip()
    password = os.environ.get("OPENOBSERVE_PASSWORD", "").strip()

    if not user or not password:
        print("⚠️  未配置 OPENOBSERVE_USER / PASSWORD → 跳过 OpenObserve")
        return None

    from opentelemetry import trace
    from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
    from opentelemetry.sdk.resources import SERVICE_NAME, Resource
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.sdk.trace.export import BatchSpanProcessor

    # ⚠️ 关键：OTel 默认给的是 ProxyTracerProvider（占位，没有 add_span_processor）。
    # 必须自己建一个真正的 TracerProvider 并设为全局，否则挂不上 exporter。
    provider = trace.get_tracer_provider()
    if not hasattr(provider, "add_span_processor"):
        # Resource 描述「这些 span 是哪个服务产生的」。
        # 不设的话平台里会显示成 unknown_service，多个服务的数据混在一起没法区分。
        provider = TracerProvider(
            resource=Resource.create({SERVICE_NAME: os.environ.get("OTEL_SERVICE_NAME", "my-agent")})
        )
        trace.set_tracer_provider(provider)

    token = base64.b64encode(f"{user}:{password}".encode()).decode()
    exporter = OTLPSpanExporter(
        endpoint=endpoint,
        headers={"Authorization": f"Basic {token}"},
    )
    provider.add_span_processor(BatchSpanProcessor(exporter))
    print(f"✓ OpenObserve exporter 已挂载  endpoint={endpoint}")
    return provider


# ────────────────────────────────────────────────────────────
# 5) HTTP 追踪：SDK 调模型走的是 httpx，装上就有网络请求的 span
# ────────────────────────────────────────────────────────────
def setup_http_tracing() -> None:
    """给 httpx 装上自动埋点。

    效果：每次模型调用会多出一个 HTTP span，带 url / method / status_code /
    请求体大小 / 耗时。注意它**不含正文**——正文在 generation span 里，
    这是正确的分层，避免把 prompt 存两遍。
    """
    from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor

    HTTPXClientInstrumentor().instrument()
    print("✓ HTTP 追踪已开启（httpx → 全局）")


# ────────────────────────────────────────────────────────────
# 6) 日志：把 Python logging 桥接到 OTLP
#    关键收益：日志会自动带上当前 span 的 trace_id，无需手动拼
# ────────────────────────────────────────────────────────────
def setup_logs():
    """桥接 logging → OTLP logs（只发 OpenObserve，Langfuse 不收日志）。"""
    import logging

    user = os.environ.get("OPENOBSERVE_USER", "").strip()
    password = os.environ.get("OPENOBSERVE_PASSWORD", "").strip()
    if not user or not password:
        print("⚠️  未配置 OpenObserve 凭据 → 跳过日志上报")
        return None

    from opentelemetry.exporter.otlp.proto.http._log_exporter import OTLPLogExporter
    from opentelemetry.sdk._logs import LoggerProvider, LoggingHandler
    from opentelemetry.sdk._logs.export import BatchLogRecordProcessor
    from opentelemetry.sdk.resources import SERVICE_NAME, Resource

    # OpenObserve 三种信号同一套 API，把 /v1/traces 换成 /v1/logs 即可
    traces_ep = os.environ.get(
        "OPENOBSERVE_ENDPOINT", "http://localhost:5080/api/default/v1/traces"
    )
    logs_ep = traces_ep.replace("/v1/traces", "/v1/logs")

    token = base64.b64encode(f"{user}:{password}".encode()).decode()
    log_provider = LoggerProvider(
        resource=Resource.create(
            {SERVICE_NAME: os.environ.get("OTEL_SERVICE_NAME", "my-agent")}
        )
    )
    log_provider.add_log_record_processor(
        BatchLogRecordProcessor(
            OTLPLogExporter(endpoint=logs_ep, headers={"Authorization": f"Basic {token}"})
        )
    )

    # 挂到 root logger：之后任何模块的 logging 都会自动进 OTLP。
    # LoggingHandler 会自动把当前 span 的 trace_id / span_id 写进日志记录，
    # 所以在平台上「日志 ↔ trace」能互相跳转，不用你手动拼 trace_id。
    root = logging.getLogger()
    root.addHandler(LoggingHandler(level=logging.INFO, logger_provider=log_provider))
    root.setLevel(logging.INFO)

    print(f"✓ 日志已桥接到 OTLP  endpoint={logs_ep}")
    return log_provider


async def main() -> None:
    load_observability_env()

    # ── 顺序很关键，别随手调 ──
    # 1) 先建好 TracerProvider 并挂 exporter（后面所有埋点都依赖它）
    provider = setup_openobserve()
    # 2) 日志桥接（让 logging 进 OTLP）
    log_provider = setup_logs()
    # 3) patch httpx —— 必须在创建 AsyncOpenAI 之前！
    #    HTTPXClientInstrumentor 是 patch「类」，只影响之后创建的实例；
    #    AsyncOpenAI 内部会实例化 httpx.AsyncClient，所以晚一步就漏掉网络请求 span。
    setup_http_tracing()
    # 4) 摘掉 SDK 默认发往 OpenAI 的 exporter（保留 tracing 机制本身）
    detach_default_trace_exporter()
    # 5) 创建模型 client（此时 httpx 已被 patch）
    settings = setup_model()
    # 6) Langfuse 挂它自己的 exporter
    langfuse = setup_langfuse()
    # 7) Agent 语义埋点
    setup_instrumentation()

    log = logging.getLogger("hello")

    # ── 你要看的 case（就是改过中文俳句的那个 hello_world）──
    question = "讲讲编程里的递归。"
    agent = Agent(
        name="Assistant",
        instructions="你只用中文俳句回答。",
        model=settings.model,
    )

    # 想让日志带上 trace_id，必须写在「真实的 OTel span」里。
    #   · SDK 的 trace() 只是个逻辑容器，不是 OTel span —— 直接写在它里面的日志没有 trace_id
    #   · custom_span() 才会产生真正的 OTel span —— 写在它里面的日志自动带 trace_id
    #   · Runner.run 期间（Agent/工具内部）本来就有 span，那里的日志天然带 trace_id
    with trace("hello_world 单轮问答"):
        with custom_span("启动"):
            log.info("开始执行：%s", question)
        result = await Runner.run(agent, question)
        with custom_span("收尾"):
            log.info("执行完成，输出 %d 字", len(result.final_output))

    print("\n───── 模型输出 ─────")
    print(result.final_output)

    # ────────────────────────────────────────────────────────
    # 5) 必须 flush！
    #    OTLP 是批量异步导出的，脚本跑完就退出的话，
    #    缓冲区里的数据会直接丢掉——这是新手最常踩的坑。
    #    长驻服务可以靠后台线程慢慢发，短脚本必须显式 flush。
    # ────────────────────────────────────────────────────────
    if langfuse:
        langfuse.flush()
    if provider and hasattr(provider, "force_flush"):
        provider.force_flush()
    if log_provider and hasattr(log_provider, "force_flush"):
        log_provider.force_flush()

    print("\n───── 去哪里看 ─────")
    if langfuse:
        print(
            f"  Langfuse     : {os.environ.get('LANGFUSE_BASE_URL', 'http://localhost:3000')} → Traces"
        )
    if provider:
        print("  OpenObserve  : http://localhost:5080 → Traces（Agent 链路 + HTTP 请求）")
    if log_provider:
        print("  OpenObserve  : http://localhost:5080 → Logs（带 trace_id，可与 trace 互跳）")
    print("  提示：OpenObserve 有索引延迟，若没看到等 10-20 秒再刷新")


if __name__ == "__main__":
    asyncio.run(main())
