"""hello_world + 可观测接入（所有接入逻辑都在 runtime/observability.py）。

本文件想说明一件事：**业务代码里几乎看不到「可观测」的存在**。
只有一行 setup()，之后日志 / span / HTTP 请求全部自动上报。
"""

from __future__ import annotations

import asyncio
import logging
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from agents import Agent, Runner, custom_span, trace

from myagent.runtime.config import load_settings
from myagent.runtime.llm import configure
from myagent.runtime.observability import flush, setup


async def main() -> None:
    settings = load_settings()

    # 顺序说明：configure() 内部会调 set_trace_processors(...)，
    # 那会清掉已挂上的 processor —— 所以负责「装上 OpenInference」的 setup()
    # 必须放在它**之后**。
    # （httpx 埋点只要求「发请求前」完成 patch，不要求早于 client 创建，
    #   因此这个顺序不影响网络请求追踪。）
    configure(settings, console_trace=False)
    setup()

    log = logging.getLogger("hello")
    question = "讲讲编程里的递归。"
    agent = Agent(
        name="Assistant",
        instructions="你只用中文俳句回答。",
        model=settings.model,
    )

    # 日志想带 trace_id，得写在「真实的 OTel span」里：
    # custom_span 可以，Runner.run 内部（Agent/工具）也可以。
    with trace("hello_world 单轮问答"):
        with custom_span("启动"):
            log.info("开始执行：%s", question)
        result = await Runner.run(agent, question)
        with custom_span("收尾"):
            log.info("执行完成，输出 %d 字", len(result.final_output))

    print("\n───── 模型输出 ─────")
    print(result.final_output)

    # 脚本跑完就退出，缓冲区里的数据会丢，必须显式刷出去
    flush()


if __name__ == "__main__":
    asyncio.run(main())
