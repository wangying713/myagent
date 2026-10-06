import asyncio

from agents import Agent, Runner
from examples.auto_mode import input_with_fallback, is_auto_mode

"""演示 `previous_response_id` 参数的用法：用它把一段对话延续下去。
第二次运行时，把上一次的 response ID 传给模型，模型就能接着聊，
而不必重新发送之前的消息。

注意事项：
1. 该参数只对 OpenAI 的 Responses API 生效，其它模型会忽略它。
2. 官方文档成文时，响应只保留 30 天。所以生产环境应当把
response ID 连同过期时间一起存储；一旦响应失效，
就需要重新发送之前的对话历史。
"""


async def main():
    print("=== 非流式示例 ===")
    agent = Agent(
        name="Assistant",
        instructions="你是一个乐于助人的助手。回答要非常简洁。",
    )

    result = await Runner.run(agent, "南美洲面积最大的国家是哪个？")
    print(result.final_output)
    # 巴西

    result = await Runner.run(
        agent,
        "那个国家的首都是哪里？",
        previous_response_id=result.last_response_id,
    )
    print(result.final_output)
    # 巴西利亚


async def main_stream():
    print("=== 流式示例 ===")
    agent = Agent(
        name="Assistant",
        instructions="你是一个乐于助人的助手。回答要非常简洁。",
    )

    result = Runner.run_streamed(agent, "南美洲面积最大的国家是哪个？")

    async for event in result.stream_events():
        if event.type == "raw_response_event" and event.data.type == "response.output_text.delta":
            print(event.data.delta, end="", flush=True)

    print()

    result = Runner.run_streamed(
        agent,
        "那个国家的首都是哪里？",
        previous_response_id=result.last_response_id,
    )

    async for event in result.stream_events():
        if event.type == "raw_response_event" and event.data.type == "response.output_text.delta":
            print(event.data.delta, end="", flush=True)


if __name__ == "__main__":
    if is_auto_mode():
        asyncio.run(main())
        print()
        asyncio.run(main_stream())
    else:
        is_stream = input_with_fallback("用流式模式运行吗？(y/n): ", "n")
        if is_stream == "y":
            asyncio.run(main_stream())
        else:
            asyncio.run(main())
