"""仓库分析 Agent —— 一个 Agent 就是一份"配置"。

它由四样东西组装而成：
    instructions  → prompts/repo_agent.md（单独放，便于改和审计）
    tools         → tools/repo.py 的 REPO_TOOLS
    model         → config.env 里的 MODEL
    （将来还可以加）output_type / handoffs / guardrails

**这里不写任何逻辑。** 逻辑在 tools/ 和 services/ 里。
一个 Agent 定义文件超过 100 行，通常意味着有东西放错层了。
"""

from pathlib import Path

from agents import Agent

from ..tools import REPO_TOOLS
from ..tools.guards import FsContext

PROMPT_PATH = Path(__file__).resolve().parents[1] / "prompts" / "repo_agent.md"


def build_repo_agent(model: str) -> Agent[FsContext]:
    """造一个仓库分析 Agent。传入 model 而不是自己读配置 —— 便于测试时替换。"""
    return Agent[FsContext](
        name="仓库分析助手",
        instructions=PROMPT_PATH.read_text(encoding="utf-8"),
        model=model,
        tools=list(REPO_TOOLS),
    )
