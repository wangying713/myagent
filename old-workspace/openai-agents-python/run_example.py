#!/usr/bin/env python3
"""run_example.py —— 不改官方示例一行代码，直接用 DeepSeek 跑它。

为什么需要这个东西：
    examples/ 下有 218 个官方示例，但它们都假定你用 OpenAI 的模型、
    走 Responses API。拿 DeepSeek 直接跑必然失败。这个脚本先把 SDK
    配置好（指向 DeepSeek + 切到 Chat Completions + trace 打控制台），
    再以 __main__ 身份执行目标示例文件。

用法：
    uv run run_example.py examples/basic/hello_world.py
    uv run run_example.py examples/basic/tools.py
    uv run run_example.py examples/handoffs/message_filter.py
    uv run run_example.py --list          # 看推荐的入口

这个脚本是自包含的：只读同目录的 config.env，不依赖任何其它文件。
注意：需要额外依赖的示例（voice 要音频设备、litellm/any-llm 那几个要装
额外包）会跑不起来，换一个就行。
"""

import os
import runpy
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent
CONFIG_PATH = REPO_ROOT / "config.env"

# 打开完整报文打印。必须在 import agents 之前设置（这两个开关是导入时读一次的）。
os.environ.setdefault("OPENAI_AGENTS_DONT_LOG_MODEL_DATA", "0")
os.environ.setdefault("OPENAI_AGENTS_DONT_LOG_TOOL_DATA", "0")

from agents import (  # noqa: E402
    set_default_openai_api,
    set_default_openai_client,
    set_trace_processors,
)
from agents.tracing.processors import BatchTraceProcessor, ConsoleSpanExporter  # noqa: E402
from openai import AsyncOpenAI  # noqa: E402

# 推荐的阅读/运行顺序：从最简到最全
RECOMMENDED = [
    ("examples/basic/hello_world.py", "最小形态：8 行核心代码，没有工具"),
    ("examples/basic/tools.py", "加工具：看 @tool 怎么把函数变成工具"),
    ("examples/basic/usage_tracking.py", "看 token 账单怎么读"),
    ("examples/basic/stream_text.py", "流式输出：字一个个蹦出来"),
    ("examples/tools/", "各种工具形态（目录，里面有多个文件）"),
    ("examples/handoffs/message_filter.py", "多 Agent 交接：Agent 之间怎么传话"),
    ("examples/agent_patterns/", "官方实现的几种架构模式（对应 Anthropic 那篇）"),
    ("examples/mcp/filesystem_example/", "接 MCP Server：工具从外部协议来"),
    ("examples/memory/", "会话记忆：跨轮不丢上下文"),
]


def load_config() -> dict[str, str]:
    """读同目录的 config.env（跟 my-agent 项目里那份格式一样）。"""
    cfg: dict[str, str] = {}
    if CONFIG_PATH.exists():
        for raw in CONFIG_PATH.read_text(encoding="utf-8").splitlines():
            line = raw.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            key, value = line.split("=", 1)
            cfg[key.strip()] = value.strip().strip('"').strip("'")
    for key in ("OPENAI_API_KEY", "OPENAI_BASE_URL", "MODEL"):
        if os.environ.get(key):
            cfg[key] = os.environ[key].strip()
    return cfg


def setup_sdk(cfg: dict[str, str]) -> None:
    """三件事：指向厂商、切到 Chat Completions、trace 打到控制台。"""
    client = AsyncOpenAI(base_url=cfg["OPENAI_BASE_URL"], api_key=cfg["OPENAI_API_KEY"])
    set_default_openai_client(client, use_for_tracing=False)
    set_default_openai_api("chat_completions")
    set_trace_processors([BatchTraceProcessor(ConsoleSpanExporter())])


def show_list() -> None:
    print("推荐入口（按由浅入深）：\n")
    for path, desc in RECOMMENDED:
        print(f"  {path:<45} {desc}")
    print("\n全部示例：")
    total = sum(1 for _ in (REPO_ROOT / "examples").rglob("*.py"))
    print(f"  examples/ 下共 {total} 个 .py，上面只是推荐，其它自己挑也行")
    print("\n跑法：uv run run_example.py <示例文件路径>")


def main() -> int:
    argv = sys.argv[1:]
    if not argv or argv[0] in {"-l", "--list", "-h", "--help"}:
        show_list()
        return 0

    cfg = load_config()
    api_key = cfg.get("OPENAI_API_KEY", "")
    if not api_key.isascii() or "在这里" in api_key or len(api_key) < 20:
        print(f"还没填 Key：打开 {CONFIG_PATH}，把 OPENAI_API_KEY= 换成真实 Key。")
        return 1

    target = Path(argv[0])
    if not target.is_absolute():
        target = REPO_ROOT / target
    if not target.exists():
        print(f"找不到示例文件：{target}")
        return 1

    setup_sdk(cfg)
    # 让官方示例里"不指定 model"的 Agent 也用我们的模型
    os.environ.setdefault("OPENAI_DEFAULT_MODEL", cfg.get("MODEL", "deepseek-chat"))
    # 示例里常用相对路径读文件，统一在仓库根目录下跑
    os.chdir(REPO_ROOT)

    print(f"──────── 运行官方示例：{target.relative_to(REPO_ROOT)} ────────")
    print(f"模型: {os.environ['OPENAI_DEFAULT_MODEL']} @ {cfg['OPENAI_BASE_URL']}")
    print("（示例里写死的 gpt-* 名字会被换成上面这个）\n")

    # 以 __main__ 身份执行，示例文件底部的 asyncio.run(main()) 才会触发
    sys.argv = [str(target), *argv[1:]]
    runpy.run_path(str(target), run_name="__main__")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
