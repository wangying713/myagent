"""工具调用 + 可观测接入：结构和 hello_world_traced.py 一样，只多了一个 @tool。

演示两件事：
  1. 用 @tool 把普通函数登记成模型可调用的工具
  2. 模型自己决定调不调这个工具

"""

from __future__ import annotations

import asyncio
from agents import Agent, Runner
from agents.decorators import tool


URL = "https://images.unsplash.com/photo-1505761671935-60b3a7427bad?auto=format&fit=crop&w=400&q=80"


# @tool 就是「返回一个包装好的工具对象，再赋回这个名字」的简写：
#     fetch_random_image = tool(fetch_random_image)
# 包装时会把函数名、文档字符串、参数类型登记成工具的名字、说明、参数表。
@tool
def fetch_random_image(subject: str) -> dict[str, str]:
    """按主题获取一张图片。"""
    # ↑ 上面这行不是注释，是「文档字符串」，会被当作工具说明发给模型。
    #   模型靠它判断该不该调这个工具，所以要写清楚。

    print(f"  [工具被调用] fetch_random_image(subject={subject!r})")

    # 返回结构化结果，工具 span 的 output.value 会呈现为 JSON，便于排查。
    return {"requested_subject": subject, "image_url": URL}


async def main() -> None:
    agent = Agent(
        name="Assistant",
        instructions="你是一个乐于助人的助手。",
        tools=[fetch_random_image],   # 传函数本身（不加括号），交给模型自己决定调不调
    )

    question = "用 fetch_random_image 工具找一张关于伦敦的图片，然后告诉我你传了什么参数、工具返回了什么"
    result = await Runner.run(agent, question)

    print(result.final_output)


if __name__ == "__main__":
    asyncio.run(main())
