"""实验脚本的入口：一行把「模型 + 观测」接好。

用法（脚本开头）：
    from myagent.runtime.lab import start
    start()

为什么值得单独一个文件：
    configure() 和 setup() 是进程级的一次性初始化，而且**顺序不能反**
    （configure 会重置 trace 处理器）。散在每个实验脚本里，迟早有人写反，
    写反的后果是数据悄悄不上报、还不报错。收口到一处，脚本里就不用记这个坑。

和 cli.py 的关系：都是"入口"，只是服务的对象不同 ——
    cli.py  面向使用者（命令行问答）
    lab.py  面向你自己（临时实验脚本）
"""

from __future__ import annotations

import atexit
import os

from .config import Settings, load_settings
from .llm import configure
from .observability import flush, setup

_started = False


def start() -> Settings:
    """读配置 → 指向模型 → 接观测平台。幂等，重复调用无副作用。

    返回值就是配置对象，需要时可以用 settings.model / settings.root。

    跑完自动 flush：脚本退出前把缓冲区里的日志/span 刷出去，
    所以实验脚本结尾不用再手写 flush()。
    """
    global _started
    if _started:
        return load_settings()

    settings = load_settings()

    # Agent 不传 model= 时，SDK 会去读这个环境变量，默认值是 "gpt-5.6-luna"
    # （打到 DeepSeek 必然 400）。这里补成配置里的模型名，
    # 好让"照抄官方示例、不写 model"的脚本也能直接跑起来。
    # 用 setdefault：临时 export 的环境变量优先，跟 config.py 的优先级一致。
    os.environ.setdefault("OPENAI_DEFAULT_MODEL", settings.model)

    # console_trace 固定 False：setup() 会把 SDK 的 trace 处理器整体接管，
    # 控制台那路留着也没用（所以这里不给开关，避免误配）。
    configure(settings, console_trace=False)
    setup()
    atexit.register(flush)
    _started = True
    return settings
