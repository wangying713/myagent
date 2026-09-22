"""Agent 定义层。每个文件定义一个 Agent，只做"组装"，不写逻辑。

为什么不叫 `agents/`：
框架自己的包就叫 `agents`。如果我们的目录也叫 agents，读代码时会撞上这种
让人发懵的对照（左边是框架，右边是我们）：

    from agents import Agent                     # 框架提供的能力
    from myagent.agents import build_repo_agent  # 我们自己的定义

所以这里改名 definitions —— 名字不撞，一眼能分清哪个是"被依赖的"。
"""

from .repo_agent import build_repo_agent

__all__ = ["build_repo_agent"]
