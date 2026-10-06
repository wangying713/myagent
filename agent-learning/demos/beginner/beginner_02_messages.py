"""第二课：你负责保存历史，下一次请求显式带上它。"""

from llm import DeepSeekClient, final_text, message_of


def main():
    """连续发送两轮对话，演示应用如何保存并回传消息历史。"""
    messages = [{"role": "system", "content": "用简短中文回答。"}]
    with DeepSeekClient("02_messages") as model:
        print("trace_id:", model.trace_id)
        for question in [
            "我给一只虚构小猫取名叫豆包，请记住。",
            "我给小猫取了什么名字？",
        ]:
            messages.append({"role": "user", "content": question})
            data = model.chat(messages)
            messages.append(message_of(data))
            print("用户:", question)
            print("模型:", final_text(data))
            print("本轮 usage:", data.get("usage"))


if __name__ == "__main__":
    main()
