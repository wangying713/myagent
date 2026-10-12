"""初级课程的乘法工具：从函数声明生成描述、校验并执行。"""

from langchain_core.tools import tool

from agent_toolkit.tools import ToolSet


@tool
def multiply(a: float, b: float) -> float:
    """计算两个数的乘积。涉及乘法时使用。"""
    return a * b


tool_set = ToolSet([multiply])
