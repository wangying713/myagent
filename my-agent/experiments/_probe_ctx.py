"""探针：在带 ctx 的工具函数里，到底能拿到什么？

验证三件事：
  1. RunContextWrapper（ctx）里有没有 trace 信息？
  2. 用 OTel API 能不能拿到当前 trace_id / span_id？
  3. 写普通日志会不会自动关联 trace？
"""
from __future__ import annotations

import asyncio
import logging
from agents import Agent, RunContextWrapper, Runner, function_tool
from opentelemetry import trace


log = logging.getLogger("probe")


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
    print("     logging 已发送给平台，trace_id / span_id 是结构化字段，无需拼进消息")

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
    agent = Agent(
        name="ProbeAgent",
        instructions="你必须调用 probe 工具，然后原样复述它的返回值。",
        tools=[probe],
    )
    result = await Runner.run(agent, "请调用 probe 工具，keyword 传 'hello'。")
    print("最终输出:", result.final_output)


if __name__ == "__main__":
    asyncio.run(main())
