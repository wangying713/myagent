"""hello_world + 可观测接入（接入逻辑都在 runtime/observability.py）。

业务代码里几乎看不到「可观测」：只调一次 setup()，
之后日志 / span / HTTP 请求就自动上报了。
"""

# 让类型注解不在运行时求值（避免低版本 Python 直接报错），模板代码。
from __future__ import annotations

import asyncio              # 异步相关，文件末尾的 asyncio.run() 用它
import logging              # 打日志
import sys                  # 下面要改 sys.path，得先导入它
from pathlib import Path    # 处理文件路径

# 把 <项目根>/src 加进模块搜索路径，下面的 myagent.xxx 才导入得到。
# 正式安装过 `uv pip install -e .` 之后，这行就不需要了。
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

# from A import B：从 A 这个包里取出 B 这个名字，之后直接用 B 就行。
from agents import Agent, Runner, custom_span, trace

# 项目自己做的小分层，一个文件只管一件事：
from myagent.runtime.config import load_settings          # 读配置
from myagent.runtime.llm import configure                # 选定模型
from myagent.runtime.observability import flush, setup   # 接观测平台


# 逐个词读这行：
#   async def   声明「异步函数」，函数体里可以用 await 等 IO
#   main        函数名
#   ()          参数列表，空的表示不用传参
#   -> None     返回值标注：不返回任何东西（只是标注，运行时不检查）
#   :           冒号，下面缩进的那一块就是函数体
async def main() -> None:
    # 读取配置，拿到 api_key / base_url / model
    settings = load_settings()

    # 顺序不能反！
    # configure() 会重置 trace 处理器，必须先执行；
    # setup() 要在它之后把处理器挂上去，否则数据不会上报，而且不报错。
    configure(settings, console_trace=False)
    setup()

    # 取一个具名 logger。setup() 已经把日志接到观测平台，
    # 所以这里打的日志会自动带上 trace_id。
    log = logging.getLogger("hello")

    # 提问内容，后面两处都要用它
    question = "讲讲编程里的递归。"

    # 创建 Agent。这三行是「参数名=值」的写法，参数顺序随便调。
    agent = Agent(
        name="Assistant",                    # 名字，观测平台上看到的 span 名
        instructions="你只用中文俳句回答。",   # 系统提示词
        model=settings.model,                # 用哪个模型，来自配置
    )

    # with 是「成对的开和关」：进入时开一个 span，退出时结束并上报。
    # 嵌套时按栈式退出（后进先出）：先关 custom_span，再关 trace。
    # 日志写在 span 里面，才会带上 trace_id。
    with trace("hello_world 单轮问答"):
        with custom_span("启动"):
            # %s 是占位符，会被后面的 question 替换掉
            log.info("开始执行：%s", question)

        # await = 等这个耗时操作做完再往下走（等的时候不占 CPU）。
        # Runner.run 跑一整轮：调模型 → 可能要调工具 → 再调模型……
        # 直到出最终答案，结果存进 result。
        result = await Runner.run(agent, question)

        with custom_span("收尾"):
            log.info("执行完成，输出 %d 字", len(result.final_output))

    print("\n───── 模型输出 ─────")     # 打印到终端
    print(result.final_output)          # result 里的最终答案

    # 数据是批量缓冲发送的，脚本马上退出会丢，所以必须刷一次。
    flush()


# 只有「直接运行这个文件」时，__name__ 才等于 "__main__"；
# 被别人 import 时这段不会执行。（Python 的入口约定）
if __name__ == "__main__":
    # 启动事件循环来运行 main()。异步函数不能像普通函数那样直接调用。
    asyncio.run(main())
