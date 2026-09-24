import asyncio

from pydantic import BaseModel

from agents import (
    Agent,
    Runner,
    Usage,
)
from agents.decorators import tool


class Weather(BaseModel):
    city: str
    temperature_range: str
    conditions: str


@tool
def get_weather(city: str) -> Weather:
    """查询指定城市的当前天气信息。"""
    return Weather(city=city, temperature_range="14-20C", conditions="Sunny with wind.")


def print_usage(usage: Usage) -> None:
    print("\n=== 用量 ===")
    print(f"输入 token: {usage.input_tokens}")
    print(f"输出 token: {usage.output_tokens}")
    print(f"合计 token: {usage.total_tokens}")
    print(f"请求次数: {usage.requests}")
    for i, request in enumerate(usage.request_usage_entries):
        print(f"  {i + 1}: 输入 {request.input_tokens}，输出 {request.output_tokens}")


async def main() -> None:
    agent = Agent(
        name="Usage Demo",
        instructions="你是一个简洁的助手。需要时使用工具。",
        tools=[get_weather],
    )

    result = await Runner.run(agent, "东京的天气怎么样？")

    print("\n最终输出：")
    print(result.final_output)

    # 从运行上下文里取用量
    print_usage(result.context_wrapper.usage)


if __name__ == "__main__":
    asyncio.run(main())
