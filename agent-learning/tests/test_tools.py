"""工具声明、严格参数校验与通用分发的离线回归。"""

import json
import unittest

from langchain_core.tools import tool

from agent_toolkit.tools import ToolSet
from toolbox import tool_set


def tool_call(name, arguments):
    """构造与模型响应相同格式的工具调用。"""
    return {
        "type": "function",
        "function": {
            "name": name,
            "arguments": json.dumps(arguments),
        },
    }


class ToolTests(unittest.TestCase):
    def test_new_tool_uses_its_own_parameters_and_defaults(self):
        @tool(parse_docstring=True)
        def greet(text: str, suffix: str = "!") -> str:
            """为文本添加后缀。

            Args:
                text: 问候文本。
                suffix: 问候后缀。
            """
            return text + suffix

        tools = ToolSet([greet])
        function = tools.schemas[0]["function"]
        self.assertEqual(function["name"], "greet")
        self.assertEqual(function["description"], "为文本添加后缀。")
        self.assertEqual(function["parameters"]["required"], ["text"])
        self.assertFalse(function["parameters"]["additionalProperties"])
        self.assertEqual(
            function["parameters"]["properties"]["text"]["description"],
            "问候文本。",
        )
        self.assertEqual(
            tools.dispatch(tool_call("greet", {"text": "你好"})),
            {"ok": True, "result": "你好!"},
        )
        self.assertEqual(
            tools.dispatch(
                tool_call("greet", {"text": "你好", "suffix": "。"})
            ),
            {"ok": True, "result": "你好。"},
        )

    def test_invalid_arguments_do_not_execute_tool(self):
        executions = []

        @tool
        def record(text: str) -> str:
            """记录收到的文本。"""
            executions.append(text)
            return text

        tools = ToolSet([record])
        for arguments in [{}, {"text": 1}, {"text": "hi", "extra": 1}]:
            with self.subTest(arguments=arguments):
                self.assertEqual(
                    tools.dispatch(tool_call("record", arguments)),
                    {"ok": False, "error": "invalid_arguments"},
                )
        self.assertEqual(executions, [])
        self.assertEqual(
            tools.dispatch(tool_call("unregistered", {"text": "hi"})),
            {"ok": False, "error": "unknown_tool"},
        )

    def test_duplicate_names_are_rejected(self):
        @tool
        def echo(text: str) -> str:
            """返回文本。"""
            return text

        with self.assertRaises(ValueError):
            ToolSet([echo, echo])

    def test_tool_execution_error_has_stable_result(self):
        @tool
        def fail() -> str:
            """模拟执行失败。"""
            raise RuntimeError("private implementation detail")

        self.assertEqual(
            ToolSet([fail]).dispatch(tool_call("fail", {})),
            {"ok": False, "error": "tool_execution_failed"},
        )

    def test_multiply_accepts_integer_and_float_inputs(self):
        self.assertEqual(tool_set.schemas[0]["function"]["name"], "multiply")
        for arguments, expected in [
            ({"a": 17, "b": 23}, 391),
            ({"a": -1_000_001, "b": 2}, -2_000_002),
            ({"a": 1.5, "b": 2}, 3.0),
        ]:
            with self.subTest(arguments=arguments):
                response = tool_set.dispatch(tool_call("multiply", arguments))
                self.assertEqual(response, {"ok": True, "result": expected})

    def test_multiply_rejects_invalid_numbers_and_fields(self):
        for arguments in [
            {"a": True, "b": 2},
            {"a": "2", "b": 2},
            {"a": None, "b": 2},
            {"a": float("nan"), "b": 2},
            {"a": float("inf"), "b": 2},
            {"a": -float("inf"), "b": 2},
            {"a": 10**400, "b": 2},
            {"a": 1},
            {"a": 1, "b": 2, "extra": 3},
            [],
        ]:
            with self.subTest(arguments=arguments):
                self.assertEqual(
                    tool_set.dispatch(tool_call("multiply", arguments)),
                    {"ok": False, "error": "invalid_arguments"},
                )


if __name__ == "__main__":
    unittest.main()
