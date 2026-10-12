"""将 LangChain 工具接入手写 Agent 的工具调用协议。"""

import json
from collections.abc import Sequence

from langchain_core.tools import BaseTool
from langchain_core.utils.function_calling import convert_to_openai_tool
from pydantic import BaseModel, ConfigDict, ValidationError, create_model


class ToolSet:
    """从调用方提供的工具列表生成模型描述和工具注册表。

    Attributes:
        schemas: 发给模型的 OpenAI 兼容工具描述。
    """

    def __init__(self, tools: Sequence[BaseTool]):
        """登记 Pydantic 2 工具；拒绝重名或旧版参数模型。"""
        self._tools: dict[str, BaseTool] = {}
        self.schemas: list[dict] = []
        for tool in tools:
            if tool.name in self._tools:
                raise ValueError("工具名称不能重复")
            input_model = tool.get_input_schema()
            if not issubclass(input_model, BaseModel):
                raise TypeError("ToolSet 只支持 Pydantic 2 参数模型")
            # 在调用边界统一校验，业务函数只需普通类型注解。
            arguments_model = create_model(
                f"{tool.name}Arguments",
                __base__=input_model,
                __config__=ConfigDict(
                    extra="forbid", strict=True, allow_inf_nan=False
                ),
            )
            checked_tool = tool.model_copy(
                update={"args_schema": arguments_model}
            )
            self._tools[tool.name] = checked_tool
            schema = convert_to_openai_tool(checked_tool)
            # 转换工具时库会重建 schema，需同步声明额外字段的拒绝策略。
            schema["function"]["parameters"]["additionalProperties"] = False
            self.schemas.append(schema)

    def dispatch(self, call: dict) -> dict:
        """解析模型请求，由工具参数模型校验后执行。

        Args:
            call: 含 function.name 和 JSON 字符串 arguments 的调用对象。

        Returns:
            含 ok 和 result 的结果，或含 ok 和稳定 error 码的错误对象。
        """
        if not isinstance(call, dict) or not isinstance(
            call.get("function"), dict
        ):
            return {"ok": False, "error": "invalid_tool_call"}
        function = call["function"]
        name = function.get("name")
        if (
            call.get("type") != "function"
            or not isinstance(name, str)
            or name not in self._tools
        ):
            return {"ok": False, "error": "unknown_tool"}
        try:
            arguments = json.loads(function.get("arguments", ""))
        except (ValueError, TypeError):
            return {"ok": False, "error": "invalid_json"}
        if not isinstance(arguments, dict):
            return {"ok": False, "error": "invalid_arguments"}
        try:
            result = self._tools[name].invoke(arguments)
        except ValidationError:
            return {"ok": False, "error": "invalid_arguments"}
        except Exception:
            return {"ok": False, "error": "tool_execution_failed"}
        return {"ok": True, "result": result}
