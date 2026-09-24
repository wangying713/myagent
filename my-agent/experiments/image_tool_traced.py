"""工具调用 + 可观测接入：结构和 hello_world_traced.py 一样，只多了一个 @tool。

演示三件事：
  1. 用 @tool 把普通函数登记成模型可调用的工具
  2. 模型自己决定调不调这个工具
  3. setup() 接上观测平台后，工具调用也自动上报（trace 里能看到工具的 span）

跑法：python3 my-agent/experiments/image_tool_traced.py
"""

from __future__ import annotations

import asyncio
import logging
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from agents import Agent, Runner, ToolOutputImageDict, custom_span, trace
from agents.decorators import tool

from myagent.runtime.config import load_settings
from myagent.runtime.llm import configure
from myagent.runtime.observability import flush, setup

# 开关：True 返回图片（要求模型能看图），False 返回纯文本（任何模型都能跑）。
# 当前 config.env 配的是 deepseek-chat，不支持图片输入，所以默认走 False。
RETURN_IMAGE = False

URL = "https://images.unsplash.com/photo-1505761671935-60b3a7427bad?auto=format&fit=crop&w=400&q=80"


# @tool 就是「返回一个包装好的工具对象，再赋回这个名字」的简写：
#     fetch_random_image = tool(fetch_random_image)
# 包装时会把函数名、文档字符串、参数类型登记成工具的名字、说明、参数表。
@tool
def fetch_random_image() -> str | ToolOutputImageDict:
    # ↑ 返回值标注要盖住「所有 return 分支」：纯文本是 str，
    #   图片那条分支返回的是字典，所以两样都得写上。
    """获取一张随机图片。"""
    # ↑ 上面这行不是注释，是「文档字符串」，会被当作工具说明发给模型。
    #   模型靠它判断该不该调这个工具，所以要写清楚。

    print("  [工具被调用] fetch_random_image")   # 打出来好确认工具真的被调了

    if RETURN_IMAGE:
        # 图片要能被模型"看到"，前提是模型支持图片输入（gpt-4o、qwen-vl 这类）
        return {"type": "image", "image_url": URL, "detail": "auto"}

    # 纯文本：任何模型都能跑，用来观察「工具被调用 → 结果回传 → 模型作答」这条链路
    return f"图片地址：{URL}"


async def main() -> None:
    settings = load_settings()

    # 顺序不能反！
    # configure() 会重置 trace 处理器，必须先执行；
    # setup() 要在它之后把处理器挂上去，否则数据不会上报，而且不报错。
    configure(settings, console_trace=False)
    setup()

    log = logging.getLogger("tool")

    agent = Agent(
        name="Assistant",
        instructions="你是一个乐于助人的助手。",
        tools=[fetch_random_image],   # 传函数本身（不加括号），交给模型自己决定调不调
        model=settings.model,
    )

    question = "用 fetch_random_image 工具取一张图片，然后描述它"
    with trace("工具调用 单轮问答"):
        with custom_span("启动"):
            log.info("开始执行：%s", question)

        result = await Runner.run(agent, question)

        with custom_span("收尾"):
            log.info("执行完成，输出 %d 字", len(result.final_output))

    print("\n───── 模型输出 ─────")
    print(result.final_output)

    # 数据是批量缓冲发送的，脚本马上退出会丢，所以必须刷一次。
    flush()


if __name__ == "__main__":
    asyncio.run(main())
