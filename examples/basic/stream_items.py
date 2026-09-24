import asyncio
import random

from agents import (
    Agent,
    ItemHelpers,
    Runner,
)
from agents.decorators import tool


@tool
def how_many_jokes() -> int:
    """返回 1 到 10（含端点）之间随机一个整数，表示要讲几个笑话。"""
    return random.randint(1, 10)


async def main():
    agent = Agent(
        name="Joker",
        instructions="先调用 `how_many_jokes` 工具，然后讲那么多个笑话。",
        tools=[how_many_jokes],
    )

    result = Runner.run_streamed(
        agent,
        input="你好",
    )
    print("=== 运行开始 ===")
    async for event in result.stream_events():
        # 这里忽略原始响应的事件增量
        if event.type == "raw_response_event":
            continue
        elif event.type == "agent_updated_stream_event":
            print(f"Agent 已更新：{event.new_agent.name}")
            continue
        elif event.type == "run_item_stream_event":
            if event.item.type == "tool_call_item":
                print(f"-- 调用了工具：{getattr(event.item.raw_item, 'name', 'Unknown Tool')}")
            elif event.item.type == "tool_call_output_item":
                print(f"-- 工具输出：{event.item.output}")
            elif event.item.type == "message_output_item":
                print(f"-- 消息输出：\n {ItemHelpers.text_message_output(event.item)}")
            else:
                pass  # 忽略其它事件类型

    print("=== 运行结束 ===")


if __name__ == "__main__":
    asyncio.run(main())

    # === 运行开始 ===
    # Agent 已更新：Joker
    # -- 调用了工具：how_many_jokes
    # -- 工具输出：4
    # -- 消息输出：
    #  好的，给你讲四个笑话：

    # 1. **骷髅为什么不互相打架？**
    #    因为它们没有胆量（guts）！

    # 2. **假的意大利面叫什么？**
    #    冒牌货（an impasta）！

    # 3. **稻草人为什么能获奖？**
    #    因为它在自己那片地里出类拔萃（outstanding in his field）！

    # 4. **自行车为什么倒了？**
    #    因为它太累了（two-tired）！
    # === 运行结束 ===
