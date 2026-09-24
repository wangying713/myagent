import asyncio
import logging

from agents import (  # 本次要用到的几样东西
    Agent,
    Runner,
    ToolOutputImage,      # 「工具返回图片」的对象写法
    ToolOutputImageDict,  # 同上，字典写法
)
# 从 agents/decorators.py 这个文件里取出 tool 这个名字，下面用 @tool 登记工具
from agents.decorators import tool

# 实验脚本入口：读配置 + 指向模型 + 接观测平台（退出时自动 flush）
from myagent.runtime.lab import start

# 取一个 logger。名字随便起，日志由 start() 接到观测平台。
log = logging.getLogger("tool")

# 开关：True 走字典写法，False 走对象写法，用来对比两者
return_typed_dict = True

# 要返回给模型的图片地址
URL = "https://images.unsplash.com/photo-1505761671935-60b3a7427bad?auto=format&fit=crop&w=400&q=80"


# 逐个词读下面这个定义：
#   @tool                                     装饰器：把普通函数注册成模型可调用的工具
#   def fetch_random_image                    函数名，同时也是工具名
#   ()                                        没有参数，模型不需要传值
#   -> ToolOutputImage | ToolOutputImageDict  返回值标注：两种类型都允许
#       （`|` 是 Python 3.10 才有的写法，等于旧写法 Union[ToolOutputImage, ToolOutputImageDict]）
@tool
def fetch_random_image() -> ToolOutputImage | ToolOutputImageDict:
    """获取一张随机图片。"""

    # ⚠️ 上面那行必须是函数体的第一句（前面只能有注释），否则它就不是
    #   「文档字符串」了 —— SDK 拿不到工具说明，模型就不知道这工具干什么。

    log.info("图片工具被调用")   # 上报到平台；终端里看不到（见下面那行 print）
    print("图片工具被调用")      # 只打终端，用来本地确认工具真被调了

    if return_typed_dict:
        log.info("走字典写法")
        # 字典写法：type 固定为 image，detail 是模型看图的分辨率
        return {"type": "image", "image_url": URL, "detail": "auto"}

    # 对象写法，和上面那个字典完全等价，二选一即可
    return ToolOutputImage(image_url=URL, detail="auto")


async def main():
    start()  # ← 一行：模型 + 观测都接好；结尾也不用写 flush()

    agent = Agent(
        name="Assistant",
        instructions="你是一个乐于助人的助手。",
        tools=[fetch_random_image],  # 传函数本身（不加括号），交给模型自己决定调不调
    )

    result = await Runner.run(
        agent,
        input="用 random_image 工具取一张图片，然后描述它",
        # 这里故意写成 random_image（真实工具名是 fetch_random_image），演示模型能自己对上
    )

    log.info("模型的最终回答: %s", result.final_output)
    print(result.final_output)  # 模型的最终回答


if __name__ == "__main__":
    asyncio.run(main())
