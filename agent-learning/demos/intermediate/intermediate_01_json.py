"""第四课：JSON 可解析、字段正确、业务正确，是三层不同的检查。"""

import json

from llm import DeepSeekClient, final_text


def parse_order(text):
    """解析订单 JSON，并验证字段、类型和数量范围。

    Args:
        text: 模型生成的 JSON 字符串。

    Returns:
        只包含 item 和 quantity 的已验证字典。

    Raises:
        ValueError: JSON 无效、字段不符或订单数量超出范围时。
    """
    value = json.loads(text)
    if not isinstance(value, dict) or set(value) != {"item", "quantity"}:
        raise ValueError("订单必须且只能包含 item、quantity")
    if not isinstance(value["item"], str) or not value["item"].strip():
        raise ValueError("item 必须是非空字符串")
    if type(value["quantity"]) is not int or not 1 <= value["quantity"] <= 100:
        raise ValueError("quantity 必须是 1 到 100 的整数")
    return value


def main():
    """请求模型提取订单信息，并展示经过验证的结构化结果。"""
    with DeepSeekClient("04_json_output") as model:
        print("trace_id:", model.trace_id)
        data = model.chat(
            [
                {
                    "role": "system",
                    "content": (
                        "将订单提取为 JSON。只包含 item 和 quantity，"
                        '例如 {"item":"铅笔","quantity":2}。'
                    ),
                },
                {"role": "user", "content": "我想买三本笔记本。"},
            ],
            response_format={"type": "json_object"},
        )
        print(parse_order(final_text(data)))


if __name__ == "__main__":
    main()
