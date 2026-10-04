"""第二步：手写一个最小的工具调用 Agent 循环。"""

import json
import os

from dotenv import load_dotenv
from openai import OpenAI


def get_weather(city: str) -> str:
    """返回固定示例天气，帮助观察工具调用流程。"""
    return f"{city}今天晴，气温 22°C。"


TOOLS = [
    {
        "type": "function",
        "function": {
            "name": "get_weather",
            "description": "查询一个城市的示例天气。",
            "parameters": {
                "type": "object",
                "properties": {"city": {"type": "string", "description": "城市名"}},
                "required": ["city"],
                "additionalProperties": False,
            },
        },
    }
]


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
    question = input("你：").strip() or "北京今天天气怎么样？"
    messages = [{"role": "user", "content": question}]
    available_tools = {"get_weather": get_weather}

    # 每轮把当前对话和工具说明发给模型。模型可选择回答，或请求调用工具。
    for turn in range(5):
        print(f"\n--- 模型第 {turn + 1} 轮 ---")
        response = client.chat.completions.create(
            model=model,
            messages=messages,
            tools=TOOLS,
            tool_choice="auto",
        )
        assistant_message = response.choices[0].message

        if not assistant_message.tool_calls:
            print("DeepSeek：", assistant_message.content)
            return

        # 把模型的工具调用请求记进对话，之后还要把工具结果发回模型。
        messages.append(assistant_message.model_dump(exclude_none=True))

        for tool_call in assistant_message.tool_calls:
            tool_name = tool_call.function.name
            print(f"模型请求调用工具：{tool_name}({tool_call.function.arguments})")

            # 只运行本程序明确注册的函数，不执行模型生成的代码。
            tool = available_tools.get(tool_name)
            if tool is None:
                tool_result = f"错误：未注册工具 {tool_name}"
            else:
                try:
                    arguments = json.loads(tool_call.function.arguments)
                    city = arguments.get("city")
                    if not isinstance(city, str) or not city.strip():
                        raise ValueError("city 必须是非空字符串")
                    tool_result = tool(city)
                except (json.JSONDecodeError, ValueError) as error:
                    tool_result = f"工具参数错误：{error}"

            print("本地工具返回：", tool_result)
            messages.append(
                {
                    "role": "tool",
                    "tool_call_id": tool_call.id,
                    "content": tool_result,
                }
            )

    print("达到最多 5 轮，停止运行。")


if __name__ == "__main__":
    main()
