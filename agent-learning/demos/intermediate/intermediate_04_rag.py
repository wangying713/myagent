"""中级：固定流程先检索再生成；关键词未命中时程序直接返回无依据。"""

import argparse
import json

from llm import DeepSeekClient, final_text
from retrieval import ANSWER_RULE, dispatch_search, validate_answer


def answer_question(model, question):
    """先检索本地资料，再让模型回答并验证引用来源。

    Args:
        model: 已进入 with 块的 DeepSeekClient。
        question: 用户提出的问题。

    Returns:
        含回答、来源 ID 和证据状态的字典。

    Raises:
        ValueError: 本地检索工具执行失败时。
        ModelError: 模型响应不完整或引用校验失败时。
    """
    result = model.execute_tool(
        {
            "id": "local-retrieval",
            "type": "function",
            "function": {
                "name": "search_documents",
                "arguments": json.dumps(
                    {"query": question}, ensure_ascii=False
                ),
            },
        },
        dispatch_search,
    )
    if not result["ok"]:
        raise ValueError(result["error"])
    passages = result["passages"]
    if not passages:
        return {
            "answer": "现有资料不足，无法回答。",
            "source_ids": [],
            "insufficient_evidence": True,
        }
    data = model.chat(
        [
            {"role": "system", "content": ANSWER_RULE},
            {
                "role": "user",
                "content": json.dumps(
                    {"question": question, "passages": passages},
                    ensure_ascii=False,
                ),
            },
        ],
        response_format={"type": "json_object"},
    )
    return validate_answer(final_text(data), passages)


def main():
    """读取命令行问题，运行固定流程 RAG 并打印校验后的结果。"""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "question", nargs="?", default="星河笔记支持导出哪些格式？"
    )
    args = parser.parse_args()
    with DeepSeekClient("intermediate_rag") as model:
        print("trace_id:", model.trace_id)
        print(
            json.dumps(
                answer_question(model, args.question),
                ensure_ascii=False,
                indent=2,
            )
        )


if __name__ == "__main__":
    main()
