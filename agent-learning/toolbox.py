"""初级课程的本地乘法工具：模型选择，Python 校验并执行。"""

import json
import math

TOOLS = [
    {
        "type": "function",
        "function": {
            "name": "multiply",
            "description": "精确计算两个数的乘积。涉及乘法时使用。",
            "parameters": {
                "type": "object",
                "properties": {
                    "a": {
                        "type": "number",
                        "minimum": -1_000_000,
                        "maximum": 1_000_000,
                    },
                    "b": {
                        "type": "number",
                        "minimum": -1_000_000,
                        "maximum": 1_000_000,
                    },
                },
                "required": ["a", "b"],
                "additionalProperties": False,
            },
        },
    }
]


def multiply(a, b):
    """返回两个数的乘积。"""
    return a * b


AVAILABLE_TOOLS = {"multiply": multiply}


def dispatch(call: dict) -> dict:
    """校验模型工具调用，只分发给已登记的 Python 函数。

    Args:
        call: 模型返回的工具调用对象。

    Returns:
        含 ok 状态及计算结果，或稳定错误码的字典。
    """
    if not isinstance(call, dict) or not isinstance(call.get("function"), dict):
        return {"ok": False, "error": "invalid_tool_call"}
    function = call.get("function") or {}
    name = function.get("name")
    if (
        call.get("type") != "function"
        or not isinstance(name, str)
        or name not in AVAILABLE_TOOLS
    ):
        return {"ok": False, "error": "unknown_tool"}
    try:
        args = json.loads(function.get("arguments", ""))
    except (ValueError, TypeError):
        return {"ok": False, "error": "invalid_json"}
    if not isinstance(args, dict) or set(args) != {"a", "b"}:
        return {"ok": False, "error": "expected_exactly_a_and_b"}
    for value in args.values():
        if (
            type(value) not in (int, float)
            or abs(value) > 1_000_000
            or not math.isfinite(value)
        ):
            return {
                "ok": False,
                "error": "expected_finite_numbers_within_one_million",
            }
    return {"ok": True, "result": AVAILABLE_TOOLS[name](**args)}
