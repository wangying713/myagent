"""公用 Agent 循环：调用方提供提示词和工具列表。"""

import json
from collections.abc import Sequence

from langchain_core.tools import BaseTool

from agent_toolkit.llm import DeepSeekClient, ModelError, final_text, message_of
from agent_toolkit.tools import ToolSet


class BudgetExceededError(RuntimeError):
    """表示 Agent 达到模型调用、工具调用或 token 预算上限。"""

    pass


def run_agent(
    llm_client: DeepSeekClient,
    question: str,
    *,
    system_prompt: str,
    tools: Sequence[BaseTool],
    max_rounds: int = 5,
    max_tool_calls: int = 8,
    token_budget: int = 8000,
) -> str:
    """运行有限 Agent 循环，业务规则和工具由调用方提供。

    Args:
        llm_client: 已进入 with 块的 DeepSeekClient。
        question: 要交给 Agent 处理的用户问题。
        system_prompt: 由调用方定义的角色、任务和回答规则。
        tools: 本次运行可用的工具列表，由 @tool 装饰的函数组成。
        max_rounds: 最多向模型发起的请求轮数。
        max_tool_calls: 最多执行的工具次数。
        token_budget: 两次模型调用之间检查的累计 token 软预算。

    Returns:
        模型最终生成的文本回答。

    Raises:
        BudgetExceededError: 达到轮数、工具次数或 token 预算上限时。
        ModelError: 模型返回不完整回答或无效工具调用时。
    """
    tool_set = ToolSet(tools)
    messages = [
        {"role": "system", "content": system_prompt},
        {"role": "user", "content": question},
    ]
    tool_call_count = 0
    initial_token_count = llm_client.total_tokens
    for _ in range(max_rounds):
        # token 预算是调用间的软预算；单次仍可能超出，max_tokens 限制输出。
        if llm_client.total_tokens - initial_token_count >= token_budget:
            raise BudgetExceededError("达到累计 token 预算")
        response_data = llm_client.chat(
            messages, tools=tool_set.schemas, tool_choice="auto"
        )
        message = message_of(response_data)
        messages.append(message)  # 保留完整 assistant 消息，包括 tool_calls。
        tool_calls = message.get("tool_calls") or []
        if not tool_calls:
            return final_text(response_data)
        if not isinstance(tool_calls, list) or not all(
            isinstance(call, dict) for call in tool_calls
        ):
            raise ModelError("tool_calls 必须是对象列表")
        if tool_call_count + len(tool_calls) > max_tool_calls:
            raise BudgetExceededError("达到工具执行次数上限")
        tool_call_ids = [call.get("id") for call in tool_calls]
        if not all(
            isinstance(tool_call_id, str) and tool_call_id
            for tool_call_id in tool_call_ids
        ) or len(set(tool_call_ids)) != len(tool_call_ids):
            raise ModelError("工具调用 id 缺失或重复")
        for call in tool_calls:  # 一轮可能提出多个工具请求，逐个回填。
            result = llm_client.execute_tool(call, tool_set.dispatch)
            tool_call_count += 1
            messages.append(
                {
                    "role": "tool",
                    "tool_call_id": call["id"],
                    "content": json.dumps(result, ensure_ascii=False),
                }
            )
    raise BudgetExceededError("达到最大模型调用轮数，任务未完成")
