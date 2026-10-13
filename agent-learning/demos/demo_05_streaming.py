"""第五课：流式显示，观测仍是一条完整模型调用。"""

from agent_toolkit.llm import DeepSeekClient, final_text


def main():
    """调用流式接口，边接收边打印回答，并显示 token 用量。"""
    with DeepSeekClient("05_stream") as llm_client:
        print("trace_id:", llm_client.trace_id)
        response_data = llm_client.chat_stream(
            [
                {
                    "role": "user",
                    "content": "用不超过三句话解释 Agent 中的工具调用。",
                }
            ],
            on_text=lambda text: print(text, end="", flush=True),
        )
        final_text(response_data)  # 检查是否完整结束。
        print("\nusage:", response_data.get("usage"))


if __name__ == "__main__":
    main()
