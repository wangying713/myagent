"""探针：在带 ctx 的工具函数里，到底能拿到什么？

验证三件事：
  1. RunContextWrapper（ctx）里有没有 trace 信息？
  2. 用 OTel API 能不能拿到当前 trace_id / span_id？
  3. 写普通日志会不会自动关联 trace？
"""
from __future__ import annotations

import asyncio
import logging
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from agents import Agent, RunContextWrapper, Runner, function_tool, set_default_openai_api, set_default_openai_client
from openai import AsyncOpenAI
from opentelemetry import trace

from myagent.runtime.config import load_settings

logging.basicConfig(level=logging.INFO, format="%(levelname)s %(name)s: %(message)s")
log = logging.getLogger("probe")


def setup() -> str:
    s = load_settings()
    set_default_openai_client(
        AsyncOpenAI(base_url=s.base_url, api_key=s.api_key), use_for_tracing=False
    )
    set_default_openai_api("chat_completions")

    from opentelemetry.sdk.resources import SERVICE_NAME, Resource
    from opentelemetry.sdk.trace import TracerProvider

    provider = trace.get_tracer_provider()
    if not hasattr(provider, "add_span_processor"):
        provider = TracerProvider(resource=Resource.create({SERVICE_NAME: "probe"}))
        trace.set_tracer_provider(provider)

    # 额外挂一个控制台 exporter，用来观察「到底导出了哪些 span」
    from opentelemetry.sdk.trace.export import ConsoleSpanExporter, SimpleSpanProcessor

    provider.add_span_processor(SimpleSpanProcessor(ConsoleSpanExporter()))

    from openinference.instrumentation.openai_agents import OpenAIAgentsInstrumentor

    OpenAIAgentsInstrumentor().instrument()
    return s.model


@function_tool
def probe(ctx: RunContextWrapper, keyword: str) -> str:
    """探针工具：打印能从 ctx 和 OTel 拿到的东西。"""
    print("\n" + "=" * 60)
    print("【1】ctx（RunContextWrapper）的公开属性")
    print("   ", [a for a in dir(ctx) if not a.startswith("_")])
    print(f"    ctx.context = {ctx.context!r}")
    print(f"    ctx.usage   = {ctx.usage!r}")
    print(
        "    → 里面有没有 trace_id / span_id ? ",
        "有" if any("trace" in a.lower() or "span" in a.lower() for a in dir(ctx)) else "没有",
    )

    print("\n【2】OTel 的当前 span（与 ctx 无关，来自 OTel context）")
    span = trace.get_current_span()
    sc = span.get_span_context() if span else None
    if sc and sc.trace_id:
        print(f"     当前 span 名字 : {span.name if hasattr(span, 'name') else '?'}")
        print(f"     trace_id      : {format(sc.trace_id, '032x')}")
        print(f"     span_id       : {format(sc.span_id, '016x')}")
        print("     → 这个 trace_id 就是你在平台上看的那条 trace")
    else:
        print("     拿不到（当前 OTel context 里没有活跃 span）")

    print("\n【3】写一条普通日志，看它是否自带 trace 信息")
    log.info("这是一条普通 logging 日志，keyword=%s", keyword)
    print("     ↑ 注意上面日志行里【没有】trace_id —— 默认不会关联")

    # 手动关联的做法：把 trace_id 拼进日志
    if sc and sc.trace_id:
        log.info(
            "手动关联版日志 trace_id=%s span_id=%s",
            format(sc.trace_id, "032x"),
            format(sc.span_id, "016x"),
        )

    print("\n【4】往当前 span 里写自定义属性（推荐做法之一）")
    span.set_attribute("probe.keyword", keyword)
    span.set_attribute("probe.note", "这行会在平台 trace 详情里看到")
    print("     已 set_attribute: probe.keyword / probe.note")
    print("=" * 60 + "\n")
    return "探针执行完毕"


async def main() -> None:
    model = setup()
    agent = Agent(
        name="ProbeAgent",
        instructions="你必须调用 probe 工具，然后原样复述它的返回值。",
        tools=[probe],
        model=model,
    )
    result = await Runner.run(agent, "请调用 probe 工具，keyword 传 'hello'。")
    print("最终输出:", result.final_output)


if __name__ == "__main__":
    asyncio.run(main())
