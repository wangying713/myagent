import asyncio                             # 用来跑下面的 main()，Python 的异步入口
from typing import Annotated               # 给类型标注挂一段说明，SDK 会读出来当参数描述

from pydantic import BaseModel, Field      # 用类来描述数据结构，自带类型校验

from agents import (
    Agent,                                 # 一个 Agent = 提示词 + 模型 + 工具
    Runner,                                # 负责真正把 Agent 跑起来
)
from agents.decorators import tool         # 就是 function_tool 的别名


# 工具要返回什么，先声明成一个类：写代码有补全，构造时自动校验，字段说明还会被送给模型
class Weather(BaseModel):
    city: str = Field(description="城市名")
    temperature_range: str = Field(description="气温区间，单位摄氏度")
    conditions: str = Field(description="天气状况")


# @tool 把这个普通函数改造成 FunctionTool 对象，模型才能看到并调用它。
# 函数名 → 工具名，docstring → 工具说明，Annotated 里的那句话 → 参数说明。
@tool
def get_weather(city: Annotated[str, "要查询天气的城市"]) -> Weather:
    """查询指定城市的当前天气信息。"""     # 这句是给模型看的，模型靠它判断该不该调这个工具
    print("[debug] get_weather called")    # 只为确认模型真的调了；真实项目换成实际逻辑
    # 三个字段名必须和上面 Weather 里写的完全一致，写错会在运行时抛 ValidationError
    return Weather(city=city, temperature_range="14-20C", conditions="Sunny with wind.")


# 把工具挂到 Agent 上。少了这一步，前面写好的工具模型根本看不见。
agent = Agent(
    name="Hello world",
    instructions="你是一个乐于助人的助手。",
    tools=[get_weather],
)


# 入口：把问题交给 Runner，它会自动循环「问模型 → 调工具 → 再问模型」直到拿到答案
async def main():
    result = await Runner.run(agent, input="东京的天气怎么样？")
    print(result.final_output)             # final_output 是模型最后那段自然语言回答
    # 上面这行预期打印：东京的天气是晴间多云。


# 直接 python tools.py 时才执行；被别的文件 import 时不会跑
if __name__ == "__main__":
    asyncio.run(main())                    # 启动事件循环，把 main() 跑完
