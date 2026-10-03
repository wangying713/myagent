import argparse
import asyncio
import random

from agents import Agent, GenerateDynamicPromptData, Prompt, Runner

"""
注意：本示例开箱不能用，因为这里默认的 prompt ID
在你的项目里并不存在。

使用步骤：
1. 打开 https://platform.openai.com/playground/prompts
2. 新建一个提示词变量，名为 `poem_style`。
3. 新建一个 system prompt，内容为：
```
Write a poem in {{poem_style}}
```
4. 用 `--prompt-id` 参数运行本示例。
"""

DEFAULT_PROMPT_ID = "pmpt_6965a984c7ac8194a8f4e79b00f838840118c1e58beb3332"


class DynamicContext:
    def __init__(self, prompt_id: str):
        self.prompt_id = prompt_id
        self.poem_style = random.choice(["limerick", "haiku", "ballad"])
        print(f"[debug] DynamicContext 已初始化，poem_style: {self.poem_style}")


async def _get_dynamic_prompt(data: GenerateDynamicPromptData) -> Prompt:
    ctx: DynamicContext = data.context.context
    return {
        "id": ctx.prompt_id,
        "version": "1",
        "variables": {
            "poem_style": ctx.poem_style,
        },
    }


async def dynamic_prompt(prompt_id: str):
    context = DynamicContext(prompt_id)

    agent = Agent(
        name="Assistant",
        prompt=_get_dynamic_prompt,
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


if __name__ == "__main__":                                          # 直接运行本文件才走这里，被 import 时不执行
    parser = argparse.ArgumentParser()                              # 建一个命令行参数解析器
    parser.add_argument("--dynamic", action="store_true")          # 开关参数：传了就是 True，不传是 False
    parser.add_argument("--prompt-id", type=str, default=DEFAULT_PROMPT_ID)  # 字符串参数，带默认值
    args = parser.parse_args()                                      # 真正解析命令行，结果存进 args

    if args.dynamic:                                                # 根据 --dynamic 决定走哪个分支
        asyncio.run(dynamic_prompt(args.prompt_id))                # 走动态提示词
    else:
        asyncio.run(static_prompt(args.prompt_id))                 # 走静态提示词
