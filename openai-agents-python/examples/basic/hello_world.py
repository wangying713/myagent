import asyncio

from agents import Agent, Runner


async def main():
    agent = Agent(
        name="Assistant",
        instructions="你只用中文俳句回答。",
    )

    result = await Runner.run(agent, "讲讲编程里的递归。")
    print(result.final_output)
    # 函数唤自己，
    # 把难题拆成小块，
    # 出口方得停。


if __name__ == "__main__":
    asyncio.run(main())
