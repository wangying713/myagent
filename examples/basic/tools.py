import asyncio
from typing import Annotated

from pydantic import BaseModel, Field

from agents import (
    Agent,
    Runner,
)
from agents.decorators import tool


class Weather(BaseModel):
    city: str = Field(description="城市名")
    temperature_range: str = Field(description="气温区间，单位摄氏度")
    conditions: str = Field(description="天气状况")


@tool
def get_weather(city: Annotated[str, "要查询天气的城市"]) -> Weather:
    """查询指定城市的当前天气信息。"""
    print("[debug] get_weather called")
    return Weather(city=city, temperature_range="14-20C", conditions="Sunny with wind.")


agent = Agent(
    name="Hello world",
    instructions="你是一个乐于助人的助手。",
    tools=[get_weather],
)


async def main():
    result = await Runner.run(agent, input="东京的天气怎么样？")
    print(result.final_output)
    # 东京的天气是晴间多云。


if __name__ == "__main__":
    asyncio.run(main())
