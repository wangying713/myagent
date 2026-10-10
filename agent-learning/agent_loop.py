"""公用 Agent 循环：调用方提供提示词、工具描述和分发函数。"""

import json
from typing import Callable

from llm import DeepSeekClient, ModelError, final_text, message_of


class BudgetExceeded(RuntimeError):
    """表示 Agent 达到模型调用、工具调用或 token 预算上限。"""

    pass


def run_agent(
    model: DeepSeekClient,
    question: str,
    *,
    system_prompt: str,
    tools: list[dict],
    dispatch: Callable[[dict], dict],
    max_rounds: int = 5,
    max_tool_calls: int = 8,
    token_budget: int = 8000,
) -> str:
    """运行有限 Agent 循环，业务规则和工具由调用方提供。

    Args:
        model: 已进入 with 块的 DeepSeekClient。
        question: 要交给 Agent 处理的用户问题。
        system_prompt: 由调用方定义的角色、任务和回答规则。
        tools: 发给模型的工具描述，与 dispatch 的白名单保持一致。
        dispatch: 校验工具名称和参数、执行工具并返回结果的函数。
        max_rounds: 最多向模型发起的请求轮数。
        max_tool_calls: 最多执行的工具次数。
        token_budget: 两次模型调用之间检查的累计 token 软预算。

    Returns:
        模型最终生成的文本回答。

    Raises:
        BudgetExceeded: 达到轮数、工具次数或 token 预算上限时。
        ModelError: 模型返回不完整回答或无效工具调用时。
    """
    messages = [
        {"role": "system", "content": system_prompt},
        {"role": "user", "content": question},
    ]
    tool_count = 0
    initial_tokens = model.total_tokens
    for _ in range(max_rounds):
        # token 预算是调用间的软预算；单次仍可能超出，max_tokens 限制输出。
        if model.total_tokens - initial_tokens >= token_budget:
            raise BudgetExceeded("达到累计 token 预算")
        data = model.chat(messages, tools=tools, tool_choice="auto")
        message = message_of(data)
        messages.append(message)  # 保留完整 assistant 消息，包括 tool_calls。
        calls = message.get("tool_calls") or []
        if not calls:
            return final_text(data)
        if not isinstance(calls, list) or not all(
            isinstance(call, dict) for call in calls
        ):
            raise ModelError("tool_calls 必须是对象列表")
        if tool_count + len(calls) > max_tool_calls:
            raise BudgetExceeded("达到工具执行次数上限")
        ids = [call.get("id") for call in calls]
        if not all(isinstance(i, str) and i for i in ids) or len(
            set(ids)
        ) != len(ids):
            raise ModelError("工具调用 id 缺失或重复")
        for call in calls:  # 一轮可能提出多个工具请求，逐个回填。
            result = model.execute_tool(call, dispatch)
            tool_count += 1
            messages.append(
                {
                    "role": "tool",
                    "tool_call_id": call["id"],
                    "content": json.dumps(result, ensure_ascii=False),
                }
            )
    raise BudgetExceeded("达到最大模型调用轮数，任务未完成")
