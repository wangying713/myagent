"""第五课：流式显示，观测仍是一条完整模型调用。"""

from llm import DeepSeekClient, final_text


def main():
    """调用流式接口，边接收边打印回答，并显示 token 用量。"""
    with DeepSeekClient("05_stream") as model:
        print("trace_id:", model.trace_id)
        data = model.chat_stream(
            [
                {
                    "role": "user",
                    "content": "用不超过三句话解释 Agent 中的工具调用。",
                }
            ],
            on_text=lambda text: print(text, end="", flush=True),
        )
        final_text(data)  # 检查是否完整结束。
        print("\nusage:", data.get("usage"))


if __name__ == "__main__":
    main()
