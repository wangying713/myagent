import asyncio

from openai.types.responses import ResponseTextDeltaEvent

from agents import Agent, Runner


async def main():
    agent = Agent(
        name="Joker",
        instructions="你是一个乐于助人的助手。",
    )

    result = Runner.run_streamed(agent, input="请给我讲 5 个笑话。")
    async for event in result.stream_events():
        if event.type == "raw_response_event" and isinstance(event.data, ResponseTextDeltaEvent):
            print(event.data.delta, end="", flush=True)


if __name__ == "__main__":
    asyncio.run(main())
