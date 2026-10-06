"""高级：在相同协议中切换思考模式；本实验只显示最终答案。

三引号包起来的文字叫文档字符串，用来说明这个 Python 文件的用途。
本实验会把一条用户消息发给 DeepSeek，开启思考模式，并打印最终答案和 token 用量。
"""

# 从项目里的 llm.py 文件导入两个可复用工具：
# DeepSeekClient 负责发 HTTP 请求并记录 OpenObserve trace；final_text 负责取出完整回答。
from llm import DeepSeekClient, final_text

# 常量：整个文件都会复用的题目。Python 通常用全大写名字表示运行中不打算改动的值。
QUESTION = "一个两位数，十位数字比个位大3，数位和为11。只输出这个两位数。"


# 定义一个函数。model 是调用方传进来的模型客户端；冒号后的 DeepSeekClient 是类型提示，
# -> str 表示这个函数预期返回字符串。类型提示主要帮助阅读和编辑器检查，不会自动转换数据。
def solve(model: DeepSeekClient) -> str:
    """使用思考模式解题，并返回模型的最终文本回答。"""
    # messages 是一个列表（方括号），列表里放字典（花括号）。
    # 每个字典表示一条对话消息：role 是角色，content 是消息正文。
    # 这里仅有 user 消息，没有 system 消息。
    messages = [
        {"role": "user", "content": QUESTION},
    ]

    # 调用客户端的 chat 方法，把 messages 和两个额外请求选项交给 DeepSeek。
    # thinking={"type": "enabled"} 开启 DeepSeek 的思考模式；这是请求参数，不是额外消息。
    # max_tokens=4096 限制本次回答最多生成的 token 数；它不是要求模型一定生成这么多。
    response = model.chat(
        messages,
        thinking={"type": "enabled"},
        max_tokens=4096,
    )

    # response 是模型返回的完整 JSON 字典。final_text 会检查回答是否正常结束，并取出回答文字。
    # return 把结果交还给调用 solve() 的地方。
    return final_text(response)


# main 是本文件的主流程函数；-> None 表示它不打算返回结果，而是直接打印信息。
def main() -> None:
    """运行思考模式示例并打印 trace ID、答案和 token 用量。"""
    # with 用于管理资源：进入时创建客户端，离开代码块时自动关闭并结束 trace。
    # 字符串 advanced_thinking 是这次运行的名称，方便在 OpenObserve 中辨认。
    with DeepSeekClient("advanced_thinking") as model:
        # print 可以打印多个值；逗号会让它们以空格分隔显示。
        print("trace_id:", model.trace_id)

        # 把客户端交给 solve 函数，接收它返回的最终回答文字。
        answer = solve(model)
        print("最终答案:", answer)

        # total_tokens 是客户端根据模型响应累计的 token 用量；token 可粗略理解为模型处理文字的计量单位。
        print("已知 token 总量:", model.total_tokens)


# 这是 Python 常见的直接运行检查。
# 用 python 文件.py 直接运行时，__name__ 会是 __main__，于是执行 main()。
# 如果这个文件被其他 Python 文件 import（导入），下面的主流程不会自动运行。
if __name__ == "__main__":
    main()
