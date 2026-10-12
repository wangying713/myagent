"""第四课：JSON 可解析、字段正确、业务正确，是三层不同的检查。"""

import json

from agent_toolkit.llm import DeepSeekClient, final_text


def parse_order(text):
    """解析订单 JSON，并验证字段、类型和数量范围。

    Args:
        text: 模型生成的 JSON 字符串。

    Returns:
        只包含 item 和 quantity 的已验证字典。

    Raises:
        ValueError: JSON 无效、字段不符或订单数量超出范围时。
    """
    order = json.loads(text)
    if not isinstance(order, dict) or set(order) != {"item", "quantity"}:
        raise ValueError("订单必须且只能包含 item、quantity")
    if not isinstance(order["item"], str) or not order["item"].strip():
        raise ValueError("item 必须是非空字符串")
    if type(order["quantity"]) is not int or not 1 <= order["quantity"] <= 100:
        raise ValueError("quantity 必须是 1 到 100 的整数")
    return order


def main():
    """请求模型提取订单信息，并展示经过验证的结构化结果。"""
    with DeepSeekClient("04_json_output") as llm_client:
        print("trace_id:", llm_client.trace_id)
        response_data = llm_client.chat(
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
        print(parse_order(final_text(response_data)))


if __name__ == "__main__":
    main()
