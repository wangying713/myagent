"""第一步：直接调用 DeepSeek 完成一次普通对话。"""

import os

from dotenv import load_dotenv
from openai import OpenAI


def main() -> None:
    load_dotenv()
    api_key = os.getenv("DEEPSEEK_API_KEY")
    if not api_key or api_key == "put-your-key-here":
        raise SystemExit("请先把 .env.example 复制为 .env，并填写 DEEPSEEK_API_KEY。")

    client = OpenAI(
        api_key=api_key,
        base_url=os.getenv("DEEPSEEK_BASE_URL", "https://api.deepseek.com/v1"),
    )
    model = os.getenv("DEEPSEEK_MODEL", "deepseek-flash")
    question = input("你想问什么？").strip() or "用简单的话解释编程里的递归。"

    # messages 是发给模型的对话内容。现在只有一条用户消息。
    messages = [{"role": "user", "content": question}]
    print("发送给模型的 messages：", messages)

    response = client.chat.completions.create(model=model, messages=messages)
    print("DeepSeek：", response.choices[0].message.content)


if __name__ == "__main__":
    main()
