"""第三课：一个可以完整看懂的 Agent 循环，没有 Agent 框架。"""

import json

from llm import DeepSeekClient, ModelError, final_text, message_of
from toolbox import TOOLS, dispatch


class BudgetExceeded(RuntimeError):
    """表示 Agent 达到模型调用、工具调用或 token 预算上限。"""

    pass


def run_agent(
    model, question, *, max_rounds=5, max_tool_calls=8, token_budget=8000
):
    """运行带有白名单工具的有限 Agent 循环。

    Args:
        model: 已进入 with 块的 DeepSeekClient。
        question: 要交给 Agent 处理的用户问题。
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
        {
            "role": "system",
            "content": "你是计算助手。涉及乘法必须用工具；工具出错时不能捏造结果。用中文简短回答。",
        },
        {"role": "user", "content": question},
    ]
    tool_count = 0
    initial_tokens = model.total_tokens
    for _ in range(max_rounds):
        # token 预算是调用间的软预算；单次仍可能超出，max_tokens 限制输出。
        if model.total_tokens - initial_tokens >= token_budget:
            raise BudgetExceeded("达到累计 token 预算")
        data = model.chat(messages, tools=TOOLS, tool_choice="auto")
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


def main():
    """运行一次乘法工具调用示例，并打印 trace ID 和最终答案。"""
    with DeepSeekClient("03_agent_loop") as model:
        print("trace_id:", model.trace_id)
        print(run_agent(model, "每盒有 17 支笔，买 23 盒，一共多少支？"))


if __name__ == "__main__":
    main()
