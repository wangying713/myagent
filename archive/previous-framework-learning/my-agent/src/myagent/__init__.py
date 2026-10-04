"""myagent —— 我们自己的 Agent 项目。

包入口只干一件事：把"必须在 import agents 之前生效"的开关设好。
原因：SDK 有少数开关是**模块导入时读一次**的（下面这两个就是），
散在各个文件里设置容易漏，统一放包入口最稳。

生产环境注意：下面两个开关应该**保持关闭**（默认值），
并把 trace 送到专门的观测平台做脱敏存储。本地学习才打开。
"""

import os

os.environ.setdefault("OPENAI_AGENTS_DONT_LOG_MODEL_DATA", "0")
os.environ.setdefault("OPENAI_AGENTS_DONT_LOG_TOOL_DATA", "0")

__version__ = "0.1.0"
