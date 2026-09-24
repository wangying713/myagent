import asyncio
import random
from typing import Any

from pydantic import BaseModel

from agents import (
    Agent,
    AgentHookContext,
    AgentHooks,
    RunContextWrapper,
    Runner,
    Tool,
)
from agents.decorators import tool
from examples.auto_mode import input_with_fallback, is_auto_mode


class CustomAgentHooks(AgentHooks):
    def __init__(self, display_name: str):
        self.event_counter = 0
        self.display_name = display_name

    async def on_start(self, context: AgentHookContext, agent: Agent) -> None:
        self.event_counter += 1
        # 从 context 里取 turn_input，看这个 Agent 收到了什么输入
        print(
            f"### ({self.display_name}) {self.event_counter}: Agent {agent.name} 启动，turn_input: {context.turn_input}"
        )

    async def on_end(self, context: RunContextWrapper, agent: Agent, output: Any) -> None:
        self.event_counter += 1
        print(
            f"### ({self.display_name}) {self.event_counter}: Agent {agent.name} 结束，输出 {output}"
        )

    async def on_handoff(self, context: RunContextWrapper, agent: Agent, source: Agent) -> None:
        self.event_counter += 1
        print(
            f"### ({self.display_name}) {self.event_counter}: Agent {source.name} 交接给 {agent.name}"
        )

    # 注意：on_tool_start / on_tool_end 只对本地工具生效。
    # 不包含在 OpenAI 服务端运行的托管工具，
    # 例如 WebSearchTool、FileSearchTool、CodeInterpreterTool、HostedMCPTool
    # 以及其它内置托管工具。
    async def on_tool_start(self, context: RunContextWrapper, agent: Agent, tool: Tool) -> None:
        self.event_counter += 1
        print(
            f"### ({self.display_name}) {self.event_counter}: Agent {agent.name} 开始调用工具 {tool.name}"
        )

    async def on_tool_end(
        self, context: RunContextWrapper, agent: Agent, tool: Tool, result: object
    ) -> None:
        self.event_counter += 1
        print(
            f"### ({self.display_name}) {self.event_counter}: Agent {agent.name} 结束调用工具 {tool.name}，结果 {result}"
        )


###


@tool
def random_number(max: int) -> int:
    """
    生成一个 0 到 max（含端点）之间的随机数。
    """
    if is_auto_mode():
        if max <= 0:
            print("[debug] 自动模式：返回确定值 0")
            return 0
        value = min(max, 37)
        if value % 2 == 0:
            value = value - 1 if value > 1 else 1
        print(f"[debug] 自动模式：返回确定的奇数 {value}")
        return value
    return random.randint(0, max)


@tool
def multiply_by_two(x: int) -> int:
    """简单的乘以二。"""
    return x * 2


class FinalResult(BaseModel):
    number: int


multiply_agent = Agent(
    name="Multiply Agent",
    instructions="把这个数乘以 2，然后返回最终结果。",
    tools=[multiply_by_two],
    output_type=FinalResult,
    hooks=CustomAgentHooks(display_name="Multiply Agent"),
)

start_agent = Agent(
    name="Start Agent",
    instructions="生成一个随机数。如果是偶数就停下；如果是奇数，交接给乘法 Agent。",
    tools=[random_number],
    output_type=FinalResult,
    handoffs=[multiply_agent],
    hooks=CustomAgentHooks(display_name="Start Agent"),
)


async def main() -> None:
    user_input = input_with_fallback("请输入最大值：", "50")
    try:
        max_number = int(user_input)
        await Runner.run(
            start_agent,
            input=f"生成一个 0 到 {max_number} 之间的随机数。",
        )
    except ValueError:
        print("请输入一个合法的整数。")
        return

    print("完成！")


if __name__ == "__main__":
    asyncio.run(main())
"""
$ python examples/basic/agent_lifecycle_example.py

请输入最大值：250
### (Start Agent) 1: Agent Start Agent 启动
### (Start Agent) 2: Agent Start Agent 开始调用工具 random_number
### (Start Agent) 3: Agent Start Agent 结束调用工具 random_number，结果 37
### (Start Agent) 4: Agent Start Agent 交接给 Multiply Agent
### (Multiply Agent) 1: Agent Multiply Agent 启动
### (Multiply Agent) 2: Agent Multiply Agent 开始调用工具 multiply_by_two
### (Multiply Agent) 3: Agent Multiply Agent 结束调用工具 multiply_by_two，结果 74
### (Multiply Agent) 4: Agent Multiply Agent 结束，输出 number=74
完成！
"""
