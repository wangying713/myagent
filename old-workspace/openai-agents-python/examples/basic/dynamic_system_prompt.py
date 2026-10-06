import asyncio
import random
from dataclasses import dataclass
from typing import Literal

from agents import Agent, RunContextWrapper, Runner


@dataclass
class CustomContext:
    style: Literal["haiku", "pirate", "robot"]


def custom_instructions(
    run_context: RunContextWrapper[CustomContext], agent: Agent[CustomContext]
) -> str:
    context = run_context.context
    if context.style == "haiku":
        return "只用俳句回答。"
    elif context.style == "pirate":
        return "用海盗的口吻回答。"
    else:
        return "用机器人的口吻回答，并且不停地说「beep boop」。"


agent = Agent(
    name="Chat agent",
    instructions=custom_instructions,
)


async def main():
    context = CustomContext(style=random.choice(["haiku", "pirate", "robot"]))
    print(f"使用风格：{context.style}\n")

    user_message = "给我讲个笑话。"
    print(f"用户：{user_message}")
    result = await Runner.run(agent, user_message, context=context)

    print(f"助手：{result.final_output}")


if __name__ == "__main__":
    asyncio.run(main())


"""
$ python examples/basic/dynamic_system_prompt.py

使用风格：haiku

用户：给我讲个笑话。
助手：鸡蛋为什么不爱讲笑话？
它们怕把彼此笑裂，
溅一脸蛋黄。

$ python examples/basic/dynamic_system_prompt.py
使用风格：robot

用户：给我讲个笑话。
助手：Beep boop！机器人为什么踢不好足球？Beep boop……因为它老是在调试（debug）时把自己绊倒！Beep boop！

$ python examples/basic/dynamic_system_prompt.py
使用风格：pirate

用户：给我讲个笑话。
助手：海盗为什么要去上学？

为了提高自己的「啊」音发音（arrr-ticulation）！哈哈哈！🏴‍☠️
"""
