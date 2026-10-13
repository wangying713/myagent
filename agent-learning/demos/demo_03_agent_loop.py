"""第三课：配置乘法助手，使用公用手写 Agent 循环。"""

from agent_toolkit.agent_loop import run_agent
from agent_toolkit.llm import DeepSeekClient
from demos.tools.calculator import multiply

SYSTEM_PROMPT = (
    "你是计算助手。涉及乘法必须用工具；工具出错时不能捏造结果。用中文简短回答。"
)


def main():
    """运行一次乘法工具调用示例，并打印 trace ID 和最终答案。"""
    with DeepSeekClient("03_agent_loop") as llm_client:
        print("trace_id:", llm_client.trace_id)
        print(
            run_agent(
                llm_client,
                "每盒有 17 支笔，买 23 盒，一共多少支？",
                system_prompt=SYSTEM_PROMPT,
                tools=[multiply],
            )
        )


if __name__ == "__main__":
    main()
