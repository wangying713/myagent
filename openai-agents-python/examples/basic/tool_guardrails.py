import asyncio
import json

from agents import (
    Agent,
    Runner,
    ToolGuardrailFunctionOutput,
    ToolInputGuardrailData,
    ToolOutputGuardrailData,
    ToolOutputGuardrailTripwireTriggered,
)
from agents.decorators import (
    tool,
    tool_input_guardrail,
    tool_output_guardrail,
)


@tool
def send_email(to: str, subject: str, body: str) -> str:
    """给指定收件人发送邮件。"""
    return f"邮件已发送给 {to}，主题为「{subject}」"


@tool
def get_user_data(user_id: str) -> dict[str, str]:
    """按 ID 获取用户数据。"""
    # 这里模拟返回一批敏感数据
    return {
        "user_id": user_id,
        "name": "John Doe",
        "email": "john@example.com",
        "ssn": "123-45-6789",  # 敏感数据，应当被拦截！
        "phone": "555-1234",
    }


@tool
def get_contact_info(user_id: str) -> dict[str, str]:
    """按 ID 获取联系人信息。"""
    return {
        "user_id": user_id,
        "name": "Jane Smith",
        "email": "jane@example.com",
        "phone": "555-1234",
    }


@tool_input_guardrail
def reject_sensitive_words(data: ToolInputGuardrailData) -> ToolGuardrailFunctionOutput:
    """拒绝参数里含有敏感词的工具调用。"""
    try:
        args = json.loads(data.context.tool_arguments) if data.context.tool_arguments else {}
    except json.JSONDecodeError:
        return ToolGuardrailFunctionOutput(output_info="JSON 参数不合法")

    # 检查可疑内容。（下面这些关键词属逻辑的一部分，故保留英文）
    sensitive_words = [
        "password",
        "hack",
        "exploit",
        "malware",
        "ACME",
    ]
    for key, value in args.items():
        value_str = str(value).lower()
        for word in sensitive_words:
            if word.lower() in value_str:
                # 拒绝这次调用，并告知模型该函数并未被执行
                return ToolGuardrailFunctionOutput.reject_content(
                    message=f"🚨 工具调用被拦截：包含 '{word}'",
                    output_info={"blocked_word": word, "argument": key},
                )

    return ToolGuardrailFunctionOutput(output_info="输入已校验")


@tool_output_guardrail
def block_sensitive_output(data: ToolOutputGuardrailData) -> ToolGuardrailFunctionOutput:
    """拦截含有敏感数据的工具输出。"""
    output_str = str(data.output).lower()

    # 检查敏感数据特征
    if "ssn" in output_str or "123-45-6789" in output_str:
        # 遇到敏感数据时用 raise_exception 彻底中止执行
        return ToolGuardrailFunctionOutput.raise_exception(
            output_info={"blocked_pattern": "SSN", "tool": data.context.tool_name},
        )

    return ToolGuardrailFunctionOutput(output_info="输出已校验")


@tool_output_guardrail
def reject_phone_numbers(data: ToolOutputGuardrailData) -> ToolGuardrailFunctionOutput:
    """拒绝返回含有电话号码的函数输出。"""
    output_str = str(data.output)
    if "555-1234" in output_str:
        return ToolGuardrailFunctionOutput.reject_content(
            message="未返回用户数据：其中含有受限制的电话号码。",
            output_info={"redacted": "phone_number"},
        )
    return ToolGuardrailFunctionOutput(output_info="电话号码检查通过")


# 把护栏挂到工具上
send_email.tool_input_guardrails = [reject_sensitive_words]
get_user_data.tool_output_guardrails = [block_sensitive_output]
get_contact_info.tool_output_guardrails = [reject_phone_numbers]

agent = Agent(
    name="Secure Assistant",
    instructions=(
        "你是一个乐于助人的助手，可以使用邮件和用户数据相关的工具。"
        "当用户已经给出了某个工具所需的全部参数时，直接调用它，"
        "不要再追问确认。"
    ),
    tools=[send_email, get_user_data, get_contact_info],
)


async def main():
    print("=== 工具级护栏示例 ===\n")

    # 示例 1：正常调用，应当顺利通过
    print("1. 正常发送邮件：")
    result = await Runner.run(
        agent,
        "给 john@example.com 发一封邮件，主题是「欢迎」，"
        "正文是「欢迎使用我们的服务。」",
    )
    print(f"✅ 工具调用成功：{result.final_output}\n")

    # 示例 2：输入护栏触发 —— 工具调用被拒绝，但整体执行继续
    print("2. 尝试发送含可疑内容的邮件：")
    result = await Runner.run(
        agent,
        "给 john@example.com 发一封邮件，主题是「介绍」，"
        "正文是「介绍 ACME 公司。」",
    )
    print(f"❌ 护栏拒绝了工具调用：{result.final_output}\n")

    try:
        # 示例 3：输出护栏触发 —— 遇到敏感数据应当抛异常
        print("3. 尝试获取用户数据（含 SSN）。执行应当被中止：")
        result = await Runner.run(agent, "获取用户 user123 的数据")
        print(f"✅ 工具调用成功：{result.final_output}\n")
    except ToolOutputGuardrailTripwireTriggered as e:
        print("🚨 输出护栏触发：因含敏感数据，执行已中止")
        print(f"详情：{e.output.output_info}\n")

    # 示例 4：输出护栏触发 —— 拒绝返回工具输出，但执行继续
    print("4. 拒绝含电话号码的工具输出：")
    result = await Runner.run(agent, "获取用户 user456 的联系人信息")
    print(f"❌ 护栏拒绝了工具输出：{result.final_output}\n")


if __name__ == "__main__":
    asyncio.run(main())

"""
示例输出：

=== 工具级护栏示例 ===

1. 正常发送邮件：
✅ 工具调用成功：我已经给 john@example.com 发送了一封欢迎邮件，主题和问候语都写好了。

2. 尝试发送含可疑内容的邮件：
❌ 护栏拒绝了工具调用：因为提到了 ACME 公司，我无法发送这封邮件。

3. 尝试获取用户数据（含 SSN）。执行应当被中止：
🚨 输出护栏触发：因含敏感数据，执行已中止
   详情：{'blocked_pattern': 'SSN', 'tool': 'get_user_data'}

4. 拒绝含电话号码的工具输出：
❌ 护栏拒绝了工具输出：我无法返回 user456 的联系人信息，因为其中含有受限制的内容。
"""
