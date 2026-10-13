"""第一课：messages → HTTP JSON → assistant。埋点已封装在工具包客户端中。"""

import json

from old.agent_toolkit.llm import DeepSeekClient, final_text


def main():
    """发送一条普通聊天请求，并打印请求、响应和最终回答。"""
    messages = [
        {"role": "system", "content": "你是一个简洁的中文助手。"},
        {"role": "user", "content": "用一句话解释什么是递归。"},
    ]
    with DeepSeekClient("01_http") as llm_client:
        print("trace_id:", llm_client.trace_id)
        response_data = llm_client.chat(messages)
        print("messages:", json.dumps(messages, ensure_ascii=False, indent=2))
        print(
            "response:", json.dumps(response_data, ensure_ascii=False, indent=2)
        )
        print("回答:", final_text(response_data))


if __name__ == "__main__":
    main()
