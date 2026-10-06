"""hello_world + 可观测接入（接入逻辑都在 runtime/observability.py）。

"""

# 让类型注解不在运行时求值（避免低版本 Python 直接报错），模板代码。
from __future__ import annotations

import asyncio              # 异步相关，文件末尾的 asyncio.run() 用它
from agents import Agent, Runner


# 逐个词读这行：
#   async def   声明「异步函数」，函数体里可以用 await 等 IO
#   main        函数名
#   ()          参数列表，空的表示不用传参
#   -> None     返回值标注：不返回任何东西（只是标注，运行时不检查）
#   :           冒号，下面缩进的那一块就是函数体
async def main() -> None:
    # 提问内容，后面两处都要用它
    question = "讲讲编程里的递归。"

    # 创建 Agent。这几行是「参数名=值」的写法，参数顺序随便调。
    agent = Agent(
        name="Assistant",                    # 名字，观测平台上看到的 span 名
        instructions="你只用中文俳句回答。",   # 系统提示词
    )

    result = await Runner.run(agent, question)

    print(result.final_output)          # result 里的最终答案


# 只有「直接运行这个文件」时，__name__ 才等于 "__main__"；
# 被别人 import 时这段不会执行。（Python 的入口约定）
if __name__ == "__main__":
    # 启动事件循环来运行 main()。异步函数不能像普通函数那样直接调用。
    asyncio.run(main())
