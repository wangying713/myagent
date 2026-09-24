import asyncio

from openai.types.shared import Reasoning

from agents import Agent, ModelSettings, Runner

# 如果你确实需要用 Chat Completions，可以这样配置模型，
# 然后把 chat_completions_model 传给 Agent 的构造函数。
# from openai import AsyncOpenAI
# client = AsyncOpenAI()
# from agents import OpenAIChatCompletionsModel
# chat_completions_model = OpenAIChatCompletionsModel(model="gpt-5.6-sol", openai_client=client)


async def main():
    agent = Agent(
        name="Knowledgeable GPT-5 Assistant",
        instructions="你是一个知识渊博的助手，总能给出有趣的回答。",
        model="gpt-5.6-sol",
        model_settings=ModelSettings(
            reasoning=Reasoning(effort="low"),  # 可选值："none"、"low"、"medium"、"high"、"xhigh"
            verbosity="low",  # 可选值："low"、"medium"、"high"
        ),
    )
    result = await Runner.run(agent, "讲讲编程里的递归。")
    print(result.final_output)


if __name__ == "__main__":
    asyncio.run(main())
