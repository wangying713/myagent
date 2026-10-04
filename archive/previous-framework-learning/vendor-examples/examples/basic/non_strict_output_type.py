import asyncio
import json
from dataclasses import dataclass
from typing import Any

from openai import BadRequestError

from agents import (
    Agent,
    AgentOutputSchema,
    AgentOutputSchemaBase,
    ModelBehaviorError,
    Runner,
    UserError,
)

"""本示例演示如何使用「非严格模式」的输出类型。严格模式
可以保证 JSON 输出合法，但有些 Schema 与严格模式不兼容。

下面我们定义一个与严格模式不兼容的输出类型，
然后用 strict_json_schema=False 来运行 Agent。

同时还会演示一个自定义的输出类型。

想了解哪些 Schema 与严格模式兼容，见：
https://platform.openai.com/docs/guides/structured-outputs?api-mode=responses#supported-schemas
"""


@dataclass
class OutputType:
    jokes: dict[int, str]
    """笑话列表，以笑话编号为索引。"""


class CustomOutputSchema(AgentOutputSchemaBase):
    """自定义输出 Schema 的演示。"""

    def is_plain_text(self) -> bool:
        return False

    def name(self) -> str:
        return "CustomOutputSchema"

    def json_schema(self) -> dict[str, Any]:
        return {
            "type": "object",
            "properties": {"jokes": {"type": "object", "properties": {"joke": {"type": "string"}}}},
        }

    def is_strict_json_schema(self) -> bool:
        return False

    def validate_json(self, json_str: str) -> Any:
        json_obj = json.loads(json_str)
        # 仅为演示，这里直接返回一个列表。
        return list(json_obj["jokes"].values())


async def run_with_provider_fallback(agent, input: str, schema: AgentOutputSchemaBase, json_shape: str):
    """Try SDK structured output, then demonstrate local parsing if the provider rejects it."""
    try:
        result = await Runner.run(agent, input)
        return result.final_output
    except BadRequestError as e:
        if "response_format" not in str(e) and "response_format" not in str(getattr(e, "body", "")):
            raise
        print("当前模型接口不接受 SDK 的 response_format；改用普通请求 JSON，再由 schema 本地解析。")

    fallback_agent = Agent(
        name="JSON fallback assistant",
        instructions=(
            "请严格按要求只返回一个 JSON 对象，不要加 Markdown 代码围栏或解释文字。"
            f"JSON 结构：{json_shape}"
        ),
    )
    result = await Runner.run(fallback_agent, input)
    print("[debug] 模型原始文本：", result.final_output)
    return schema.validate_json(result.final_output)


async def main():
    agent = Agent(
        name="Assistant",
        instructions="你是一个乐于助人的助手。",
        output_type=OutputType,
    )

    input = "给我讲 3 个短笑话。"

    # 先试严格模式：这里应该会抛异常。
    try:
        await Runner.run(agent, input)
    except UserError as e:
        print(f"错误（预期内）：{e}")
    else:
        raise AssertionError("严格 Schema 校验本应抛出 UserError")

    # 现在换成非严格输出类型再试一次，这次应该能跑通。
    # 但有时也会报错 —— Schema 不是严格的，
    # 模型可能产出不合法的 JSON。
    non_strict_schema = AgentOutputSchema(OutputType, strict_json_schema=False)
    agent.output_type = non_strict_schema
    try:
        output = await run_with_provider_fallback(
            agent,
            input,
            non_strict_schema,
            '{"jokes": {"1": "笑话一", "2": "笑话二", "3": "笑话三"}}',
        )
        print(output)
    except ModelBehaviorError as e:
        print(f"非严格输出校验失败（这是可能发生的情况）：{e}")

    # 最后试一下自定义输出类型。
    custom_schema = CustomOutputSchema()
    agent.output_type = custom_schema
    output = await run_with_provider_fallback(
        agent,
        input,
        custom_schema,
        '{"jokes": {"joke": "一个简短笑话"}}',
    )
    print(output)


if __name__ == "__main__":
    asyncio.run(main())
