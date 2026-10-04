import argparse
import asyncio
import os
import random

from agents import Agent, GenerateDynamicPromptData, Prompt, Runner

"""
本例提供两种模式：

1. 默认 `--mode local`：在本地动态渲染指令，兼容 DeepSeek 等 Chat Completions
   服务，适合观察变量如何进入最终 instructions。
2. `--mode hosted`：引用 OpenAI 平台 Prompt 模板，需要有效的 prompt ID，且模型
   provider 必须使用 Responses API。Chat Completions 会忽略平台 prompt。

使用步骤：
平台模板模式的准备步骤（仅 `--mode hosted` 需要）：
1. 打开 https://platform.openai.com/playground/prompts
2. 新建一个提示词变量，名为 `poem_style`。
3. 新建一个 system prompt，内容为：
```
Write a poem in {{poem_style}}
```
4. 用 `--mode hosted --prompt-id <你的 prompt ID>` 运行。
"""


class DynamicContext:
    def __init__(self, prompt_id: str | None = None):
        self.prompt_id = prompt_id
        self.poem_style = random.choice(["limerick", "haiku", "ballad"])
        print(f"[debug] DynamicContext 已初始化，poem_style: {self.poem_style}")


def _render_local_instructions(run_context, _agent) -> str:
    style = run_context.context.poem_style
    rendered = f"请用{style}风格，用中文解释用户提出的概念。"
    print(f"[debug] 最终渲染的 instructions: {rendered}")
    return rendered


async def _get_dynamic_prompt(data: GenerateDynamicPromptData) -> Prompt:
    ctx: DynamicContext = data.context.context
    if not ctx.prompt_id:
        raise ValueError("平台 Prompt 模式必须提供 prompt_id")
    return {
        "id": ctx.prompt_id,
        "version": "1",
        "variables": {"poem_style": ctx.poem_style},
    }


async def local_prompt():
    context = DynamicContext()
    agent = Agent(
        name="Assistant",
        instructions=_render_local_instructions,
    )

    result = await Runner.run(agent, "讲讲编程里的递归。", context=context)
    print(result.final_output)


async def static_prompt(prompt_id: str):
    agent = Agent(
        name="Assistant",
        prompt={
            "id": prompt_id,
            "version": "1",
            "variables": {
                "poem_style": "limerick",
            },
        },
    )

    result = await Runner.run(agent, "讲讲编程里的递归。")
    print(result.final_output)


async def dynamic_hosted_prompt(prompt_id: str):
    context = DynamicContext(prompt_id)
    agent = Agent(name="Assistant", prompt=_get_dynamic_prompt)
    result = await Runner.run(agent, "讲讲编程里的递归。", context=context)
    print(result.final_output)


if __name__ == "__main__":  # 直接运行本文件才走这里，被 import 时不执行
    parser = argparse.ArgumentParser()  # 建一个命令行参数解析器
    parser.add_argument("--mode", choices=("local", "hosted", "hosted-dynamic"), default="local")
    parser.add_argument("--prompt-id", type=str)
    args = parser.parse_args()

    if args.mode == "local":
        asyncio.run(local_prompt())
    elif not args.prompt_id:
        parser.error("hosted 模式需要传入 --prompt-id <你的 OpenAI prompt ID>")
    elif os.environ.get("OPENAI_DEFAULT_MODEL"):
        parser.error(
            "当前运行入口已配置 Chat Completions（DeepSeek 兼容模式），它不支持平台 Prompt。"
            "请用支持 OpenAI Responses API 的 OpenAI provider 单独运行本示例。"
        )
    elif args.mode == "hosted":
        asyncio.run(static_prompt(args.prompt_id))
    else:
        asyncio.run(dynamic_hosted_prompt(args.prompt_id))
