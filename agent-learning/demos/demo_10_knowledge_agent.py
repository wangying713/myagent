"""高级综合项目：模型决定搜索，程序执行只读工具，最终校验来源 ID。"""

import argparse
import json

from agent_toolkit.agent_loop import BudgetExceededError
from agent_toolkit.llm import DeepSeekClient, ModelError, final_text, message_of
from demos.tools.retrieval import (
    ANSWER_RULE,
    SEARCH_TOOLS,
    dispatch_search,
    validate_answer,
)


def run_knowledge_agent(
    llm_client, question, *, max_rounds=4, max_tools=4, token_budget=8000
):
    """运行可自主搜索本地资料的 Agent，并校验答案引用。

    Args:
        llm_client: 已进入 with 块的 DeepSeekClient。
        question: 用户的问题。
        max_rounds: 最多向模型请求的轮数。
        max_tools: 最多执行的搜索工具次数。
        token_budget: 调用间检查的累计 token 软预算。

    Returns:
        通过来源校验的答案字典。

    Raises:
        BudgetExceededError: 达到轮数、工具数或 token 预算时。
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
    source_passages = {}
    tool_call_count = 0
    initial_token_count = llm_client.total_tokens
    for round_index in range(max_rounds):
        if llm_client.total_tokens - initial_token_count >= token_budget:
            raise BudgetExceededError("资料助手达到累计 token 软预算")
        response_data = llm_client.chat(
            messages,
            tools=SEARCH_TOOLS,
            tool_choice="required" if round_index == 0 else "auto",
            max_tokens=768,
        )
        message = message_of(response_data)
        messages.append(message)
        tool_calls = message.get("tool_calls") or []
        if not tool_calls:
            if tool_call_count == 0:
                raise ModelError("尚未搜索资料，不能直接回答")
            return validate_answer(
                final_text(response_data), list(source_passages.values())
            )
        if tool_call_count + len(tool_calls) > max_tools:
            raise BudgetExceededError("资料助手达到工具执行次数上限")
        tool_call_ids = [call.get("id") for call in tool_calls]
        if not all(
            isinstance(tool_call_id, str) and tool_call_id
            for tool_call_id in tool_call_ids
        ) or len(set(tool_call_ids)) != len(tool_call_ids):
            raise ModelError("工具调用 ID 无效或重复")
        for call in tool_calls:
            result = llm_client.execute_tool(call, dispatch_search)
            tool_call_count += 1
            if result["ok"]:
                source_passages.update(
                    {passage["id"]: passage for passage in result["passages"]}
                )
            messages.append(
                {
                    "role": "tool",
                    "tool_call_id": call["id"],
                    "content": json.dumps(result, ensure_ascii=False),
                }
            )
    raise BudgetExceededError("资料助手达到模型调用轮次上限，未完成")


def main():
    """读取命令行问题，运行只读资料 Agent 并打印带引用的结果。"""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "question",
        nargs="?",
        default="星河笔记误删的笔记能恢复多久？请给出资料依据。",
    )
    args = parser.parse_args()
    with DeepSeekClient("advanced_knowledge_agent") as llm_client:
        print("trace_id:", llm_client.trace_id)
        print(
            json.dumps(
                run_knowledge_agent(llm_client, args.question),
                ensure_ascii=False,
                indent=2,
            )
        )


if __name__ == "__main__":
    main()
