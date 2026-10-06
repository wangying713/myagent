import asyncio

from openai import AsyncOpenAI

from agents import Agent, OpenAIChatCompletionsModel, Runner, set_tracing_disabled

set_tracing_disabled(True)

# import logging
# logging.basicConfig(level=logging.DEBUG)

# 这是一个在 Ollama 上使用 gpt-oss 的示例。
# 详见 https://cookbook.openai.com/articles/gpt-oss/run-locally-ollama
# 如果你更想用 LM Studio，见 https://cookbook.openai.com/articles/gpt-oss/run-locally-lmstudio
gpt_oss_model = OpenAIChatCompletionsModel(
    model="gpt-oss:20b",
    openai_client=AsyncOpenAI(
        base_url="http://localhost:11434/v1",
        api_key="ollama",
    ),
)


async def main():
    # 注意：给 Agent 用自定义 outputType 时，gpt-oss 模型可能表现不佳。
    # 建议使用默认的 "text" 输出类型。
    # 另见：https://github.com/openai/openai-agents-python/issues/1414
    agent = Agent(
        name="Assistant",
        instructions="你是一个乐于助人的助手，会针对用户的问题给出简洁的回答。",
        model=gpt_oss_model,
    )

    result = await Runner.run(agent, "讲讲编程里的递归。")
    print(result.final_output)


if __name__ == "__main__":
    asyncio.run(main())
