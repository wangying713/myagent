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
import os
import sys
from pathlib import Path

# 让脚本能 import 项目内的 myagent 包
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from agents import (
    Agent,
    Runner,
    set_default_openai_api,
    set_default_openai_client,
    set_tracing_disabled,
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


def silence_sdk_tracing() -> None:
    """关掉 SDK 自带的 tracing（它和我们挂的 OTel 链路是两套）。

    ⚠️ 不关会有一个真实的数据外发风险：
       SDK 默认装了一个 BatchTraceProcessor → BackendSpanExporter，
       目标地址是 https://api.openai.com/v1/traces/ingest。
       而它的 api_key 会 fallback 读环境变量 OPENAI_API_KEY ——
       我们用的是 DeepSeek，但如果环境里恰好有 OPENAI_API_KEY
       （哪怕就是 DeepSeek 那把 sk- 开头的 key），
       SDK 依然会把 prompt / 响应内容打包发往 api.openai.com。
       认证大概率失败，但**数据已经离开本机了**。

    这里用 set_tracing_disabled(True) 彻底关掉。
    想看 SDK 原生 span 时，可以换成：
        set_trace_processors([BatchTraceProcessor(ConsoleSpanExporter())])
    """
    set_tracing_disabled(True)


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


async def main() -> None:
    load_observability_env()
    settings = setup_model()

    # 先关掉 SDK 自带的那套 tracing，只留下面挂的 OTel 链路
    silence_sdk_tracing()

    # 顺序很重要：先建好 TracerProvider 并挂上 OpenObserve，
    # 再让 Langfuse 挂它的，最后才 instrument——
    # 否则 instrumentor 拿到的是占位 provider，span 会全部丢掉。
    provider = setup_openobserve()
    langfuse = setup_langfuse()
    setup_instrumentation()

    # ── 你要看的 case（就是改过中文俳句的那个 hello_world）──
    agent = Agent(
        name="Assistant",
        instructions="你只用中文俳句回答。",
        model=settings.model,
    )

    result = await Runner.run(agent, "讲讲编程里的递归。")
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

    print("\n───── 去哪里看 ─────")
    if langfuse:
        print(f"  Langfuse     : {os.environ.get('LANGFUSE_BASE_URL', 'http://localhost:3000')} → Traces")
    if provider:
        print("  OpenObserve  : http://localhost:5080 → Traces（组织选 default）")
    print("  提示：OpenObserve 有索引延迟，若没看到等 10-20 秒再刷新")


if __name__ == "__main__":
    asyncio.run(main())
