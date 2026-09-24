import asyncio

from agents import (
    Agent,
    Runner,
    ToolOutputImage,
    ToolOutputImageDict,
)
from agents.decorators import tool

return_typed_dict = True

URL = "https://images.unsplash.com/photo-1505761671935-60b3a7427bad?auto=format&fit=crop&w=400&q=80"


@tool
def fetch_random_image() -> ToolOutputImage | ToolOutputImageDict:
    """获取一张随机图片。"""

    print("图片工具被调用")
    if return_typed_dict:
        return {"type": "image", "image_url": URL, "detail": "auto"}

    return ToolOutputImage(image_url=URL, detail="auto")


async def main():
    agent = Agent(
        name="Assistant",
        instructions="你是一个乐于助人的助手。",
        tools=[fetch_random_image],
    )

    result = await Runner.run(
        agent,
        input="用 random_image 工具取一张图片，然后描述它",
    )
    print(result.final_output)
    """这张图是著名的钟楼，通常被称为大本钟，……"""


if __name__ == "__main__":
    asyncio.run(main())
