"""课程示例使用的本地计算工具。"""

from langchain_core.tools import tool


@tool
def multiply(a: float, b: float) -> float:
    """计算两个数的乘积。涉及乘法时使用。"""
    return a * b
