"""高级综合项目：模型决定搜索，程序执行只读工具，最终校验来源 ID。"""

import argparse
import json

from demos.beginner.beginner_03_agent_loop import BudgetExceeded
from llm import DeepSeekClient, ModelError, final_text, message_of
from retrieval import (
    ANSWER_RULE,
    SEARCH_TOOLS,
    dispatch_search,
    validate_answer,
)


def run_knowledge_agent(
    model, question, *, max_rounds=4, max_tools=4, token_budget=8000
):
    """运行可自主搜索本地资料的 Agent，并校验答案引用。

    Args:
        model: 已进入 with 块的 DeepSeekClient。
        question: 用户的问题。
        max_rounds: 最多向模型请求的轮数。
        max_tools: 最多执行的搜索工具次数。
        token_budget: 调用间检查的累计 token 软预算。

    Returns:
        通过来源校验的答案字典。

    Raises:
        BudgetExceeded: 达到轮数、工具数或 token 预算时。
        ModelError: 模型未搜索资料、工具调用无效或答案校验失败时。
    """
    messages = [
        {
            "role": "system",
            "content": "你是星河笔记资料助手。先搜索资料再回答，可调整关键词再搜索。"
            + ANSWER_RULE,
        },
        {"role": "user", "content": question},
    ]
    sources = {}
    tool_count = 0
    initial_tokens = model.total_tokens
    for round_index in range(max_rounds):
        if model.total_tokens - initial_tokens >= token_budget:
            raise BudgetExceeded("资料助手达到累计 token 软预算")
        data = model.chat(
            messages,
            tools=SEARCH_TOOLS,
            tool_choice="required" if round_index == 0 else "auto",
            max_tokens=768,
        )
        message = message_of(data)
        messages.append(message)
        calls = message.get("tool_calls") or []
        if not calls:
            if tool_count == 0:
                raise ModelError("尚未搜索资料，不能直接回答")
            return validate_answer(final_text(data), list(sources.values()))
        if tool_count + len(calls) > max_tools:
            raise BudgetExceeded("资料助手达到工具执行次数上限")
        ids = [c.get("id") for c in calls]
        if not all(isinstance(i, str) and i for i in ids) or len(
            set(ids)
        ) != len(ids):
            raise ModelError("工具调用 ID 无效或重复")
        for call in calls:
            result = model.execute_tool(call, dispatch_search)
            tool_count += 1
            if result["ok"]:
                sources.update({p["id"]: p for p in result["passages"]})
            messages.append(
                {
                    "role": "tool",
                    "tool_call_id": call["id"],
                    "content": json.dumps(result, ensure_ascii=False),
                }
            )
    raise BudgetExceeded("资料助手达到模型调用轮次上限，未完成")


def main():
    """读取命令行问题，运行只读资料 Agent 并打印带引用的结果。"""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "question",
        nargs="?",
        default="星河笔记误删的笔记能恢复多久？请给出资料依据。",
    )
    args = parser.parse_args()
    with DeepSeekClient("advanced_knowledge_agent") as model:
        print("trace_id:", model.trace_id)
        print(
            json.dumps(
                run_knowledge_agent(model, args.question),
                ensure_ascii=False,
                indent=2,
            )
        )


if __name__ == "__main__":
    main()
