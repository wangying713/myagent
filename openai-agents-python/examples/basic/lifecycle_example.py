import asyncio
import random
from typing import Any, cast

from pydantic import BaseModel

from agents import (
    Agent,
    AgentHookContext,
    AgentHooks,
    RunContextWrapper,
    RunHooks,
    Runner,
    Tool,
    Usage,
)
from agents.decorators import tool
from agents.items import ModelResponse, TResponseInputItem
from agents.tool_context import ToolContext
from examples.auto_mode import input_with_fallback


class LoggingHooks(AgentHooks[Any]):
    async def on_start(
        self,
        context: AgentHookContext[Any],
        agent: Agent[Any],
    ) -> None:
        # 从 context 里取 turn_input，看这个 Agent 收到了什么输入
        print(f"#### {agent.name} 启动，turn_input: {context.turn_input}")

    async def on_end(
        self,
        context: RunContextWrapper[Any],
        agent: Agent[Any],
        output: Any,
    ) -> None:
        print(f"#### {agent.name} 产出输出：{output}。")


class ExampleHooks(RunHooks):
    def __init__(self):
        self.event_counter = 0

    def _usage_to_str(self, usage: Usage) -> str:
        return f"{usage.requests} 次请求，{usage.input_tokens} 输入 token，{usage.output_tokens} 输出 token，{usage.total_tokens} 合计 token"

    async def on_agent_start(self, context: AgentHookContext, agent: Agent) -> None:
        self.event_counter += 1
        # 从 context 里取 turn_input，看这个 Agent 收到了什么输入
        print(
            f"### {self.event_counter}: Agent {agent.name} 启动。turn_input: {context.turn_input}。用量：{self._usage_to_str(context.usage)}"
        )

    async def on_llm_start(
        self,
        context: RunContextWrapper,
        agent: Agent,
        system_prompt: str | None,
        input_items: list[TResponseInputItem],
    ) -> None:
        self.event_counter += 1
        print(f"### {self.event_counter}: LLM 开始。用量：{self._usage_to_str(context.usage)}")

    async def on_llm_end(
        self, context: RunContextWrapper, agent: Agent, response: ModelResponse
    ) -> None:
        self.event_counter += 1
        print(f"### {self.event_counter}: LLM 结束。用量：{self._usage_to_str(context.usage)}")

    async def on_agent_end(self, context: RunContextWrapper, agent: Agent, output: Any) -> None:
        self.event_counter += 1
        print(
            f"### {self.event_counter}: Agent {agent.name} 结束，输出 {output}。用量：{self._usage_to_str(context.usage)}"
        )

    # 注意：on_tool_start / on_tool_end 只对本地工具生效。
    # 不包含在 OpenAI 服务端运行的托管工具，
    # 例如 WebSearchTool、FileSearchTool、CodeInterpreterTool、HostedMCPTool
    # 以及其它内置托管工具。
    async def on_tool_start(self, context: RunContextWrapper, agent: Agent, tool: Tool) -> None:
        self.event_counter += 1
        # 这个类型转换并不理想，
        # 但出于向后兼容，短期内不打算改动 context 参数的类型。
        tool_context = cast(ToolContext[Any], context)
        print(
            f"### {self.event_counter}: 工具 {tool.name} 开始。name={tool_context.tool_name}, call_id={tool_context.tool_call_id}, args={tool_context.tool_arguments}。用量：{self._usage_to_str(tool_context.usage)}"
        )

    async def on_tool_end(
        self, context: RunContextWrapper, agent: Agent, tool: Tool, result: object
    ) -> None:
        self.event_counter += 1
        # 这个类型转换并不理想，
        # 但出于向后兼容，短期内不打算改动 context 参数的类型。
        tool_context = cast(ToolContext[Any], context)
        print(
            f"### {self.event_counter}: 工具 {tool.name} 结束。result={result}, name={tool_context.tool_name}, call_id={tool_context.tool_call_id}, args={tool_context.tool_arguments}。用量：{self._usage_to_str(tool_context.usage)}"
        )

    async def on_handoff(
        self, context: RunContextWrapper, from_agent: Agent, to_agent: Agent
    ) -> None:
        self.event_counter += 1
        print(
            f"### {self.event_counter}: 从 {from_agent.name} 交接给 {to_agent.name}。用量：{self._usage_to_str(context.usage)}"
        )


hooks = ExampleHooks()

###


@tool
def random_number(max: int) -> int:
    """生成一个 0 到 max（含端点）之间的随机数。"""
    return random.randint(0, max)


@tool
def multiply_by_two(x: int) -> int:
    """返回 x 的两倍。"""
    return x * 2


class FinalResult(BaseModel):
    number: int


multiply_agent = Agent(
    name="Multiply Agent",
    instructions="把这个数乘以 2，然后返回最终结果。",
    tools=[multiply_by_two],
    output_type=FinalResult,
    hooks=LoggingHooks(),
)

start_agent = Agent(
    name="Start Agent",
    instructions="生成一个随机数。如果是偶数就停下；如果是奇数，交接给乘法 Agent。",
    tools=[random_number],
    output_type=FinalResult,
    handoffs=[multiply_agent],
    hooks=LoggingHooks(),
)


async def main() -> None:
    user_input = input_with_fallback("请输入最大值：", "50")
    try:
        max_number = int(user_input)
        await Runner.run(
            start_agent,
            hooks=hooks,
            input=f"生成一个 0 到 {max_number} 之间的随机数。",
        )
    except ValueError:
        print("请输入一个合法的整数。")
        return

    print("完成！")


if __name__ == "__main__":
    asyncio.run(main())
"""
$ python examples/basic/lifecycle_example.py

请输入最大值：250
### 1: Agent Start Agent 启动。用量：0 次请求，0 输入 token，0 输出 token，0 合计 token
### 2: LLM 开始。用量：0 次请求，0 输入 token，0 输出 token，0 合计 token
### 3: LLM 结束。用量：1 次请求，143 输入 token，15 输出 token，158 合计 token
### 4: 工具 random_number 开始。name=random_number, call_id=call_IujmDZYiM800H0hy7v17VTS0, args={"max":250}。用量：1 次请求，143 输入 token，15 输出 token，158 合计 token
### 5: 工具 random_number 结束。result=107, name=random_number, call_id=call_IujmDZYiM800H0hy7v17VTS0, args={"max":250}。用量：1 次请求，143 输入 token，15 输出 token，158 合计 token
### 6: LLM 开始。用量：1 次请求，143 输入 token，15 输出 token，158 合计 token
### 7: LLM 结束。用量：2 次请求，310 输入 token，29 输出 token，339 合计 token
### 8: 从 Start Agent 交接给 Multiply Agent。用量：2 次请求，310 输入 token，29 输出 token，339 合计 token
### 9: Agent Multiply Agent 启动。用量：2 次请求，310 输入 token，29 输出 token，339 合计 token
### 10: LLM 开始。用量：2 次请求，310 输入 token，29 输出 token，339 合计 token
### 11: LLM 结束。用量：3 次请求，472 输入 token，45 输出 token，517 合计 token
### 12: 工具 multiply_by_two 开始。name=multiply_by_two, call_id=call_KhHvTfsgaosZsfi741QvzgYw, args={"x":107}。用量：3 次请求，472 输入 token，45 输出 token，517 合计 token
### 13: 工具 multiply_by_two 结束。result=214, name=multiply_by_two, call_id=call_KhHvTfsgaosZsfi741QvzgYw, args={"x":107}。用量：3 次请求，472 输入 token，45 输出 token，517 合计 token
### 14: LLM 开始。用量：3 次请求，472 输入 token，45 输出 token，517 合计 token
### 15: LLM 结束。用量：4 次请求，660 输入 token，56 输出 token，716 合计 token
### 16: Agent Multiply Agent 结束，输出 number=214。用量：4 次请求，660 输入 token，56 输出 token，716 合计 token
完成！

"""
