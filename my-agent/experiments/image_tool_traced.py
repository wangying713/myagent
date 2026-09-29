"""工具调用 + 可观测接入：结构和 hello_world_traced.py 一样，只多了一个 @tool。

演示两件事：
  1. 用 @tool 把普通函数登记成模型可调用的工具
  2. 模型自己决定调不调这个工具

"""

from __future__ import annotations

import asyncio
from agents import Agent, Runner, ToolOutputImageDict
from agents.decorators import tool


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
    agent = Agent(
        name="Assistant",
        instructions="你是一个乐于助人的助手。",
        tools=[fetch_random_image],   # 传函数本身（不加括号），交给模型自己决定调不调
    )

    question = "用 fetch_random_image 工具取一张图片，然后描述它"
    result = await Runner.run(agent, question)

    print(result.final_output)


if __name__ == "__main__":
    asyncio.run(main())
