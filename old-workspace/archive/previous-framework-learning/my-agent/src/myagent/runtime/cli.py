"""入口层：只管命令行交互，不含任何 Agent 逻辑。

分层的好处在这里最明显：换 CLI、加 HTTP 接口、加定时任务，
都只是再加一个"入口"，下面几层一动不动。
"""

from __future__ import annotations

import argparse
import asyncio

from agents import Agent, Runner, TResponseInputItem, Usage

from ..definitions import build_repo_agent
from ..tools.guards import FsContext
from .config import load_settings
from .llm import configure


def print_usage(usage: Usage) -> None:
    """打账单。request_usage_entries 是"每次请求"的明细，能看出上下文越滚越大。"""
    print("\n──────── 用量 ────────")
    for index, item in enumerate(usage.request_usage_entries, start=1):
        print(f"  第 {index} 次请求: prompt={item.input_tokens} completion={item.output_tokens}")
    print(
        f"  累计: 请求 {usage.requests} 次 | prompt={usage.input_tokens} "
        f"completion={usage.output_tokens} total={usage.total_tokens}"
    )


async def ask(
    agent: Agent[FsContext],
    fs_ctx: FsContext,
    question: str,
    history: list[TResponseInputItem],
    max_turns: int,
) -> None:
    """问一句。history 是之前几轮的消息，用来把上下文拼回去。"""
    try:
        result = await Runner.run(
            agent,
            input=history + [{"role": "user", "content": question}],
            context=fs_ctx,       # ← 上下文从这里注入到工具体内
            max_turns=max_turns,  # ← 熔断
        )
    except Exception as exc:  # noqa: BLE001 - 入口层，把错误打清楚比精细分层更有用
        print(f"\n[出错] {type(exc).__name__}: {exc}")
        return

    print("\n========== 回答 ==========")
    print(result.final_output)
    print("==========================")
    print_usage(result.context_wrapper.usage)

    history.clear()
    history.extend(result.to_input_list())


async def _run(args: argparse.Namespace) -> int:
    settings = load_settings(root_override=args.root)
    configure(settings, console_trace=not args.no_trace)

    agent = build_repo_agent(settings.model)
    fs_ctx = FsContext(root=settings.root)

    print("──────── myagent ────────")
    print(f"模型      : {settings.model} @ {settings.base_url}")
    print(f"工作根目录: {settings.root}")
    print(f"工具      : {', '.join(t.name for t in agent.tools)}")
    print(f"熔断      : max_turns={args.max_turns}")
    print("──────────────────────────")

    history: list[TResponseInputItem] = []
    if args.question:
        await ask(agent, fs_ctx, args.question, history, args.max_turns)
        return 0

    print("输入问题回车；exit 退出")
    while True:
        try:
            question = input("\n> ").strip()
        except (EOFError, KeyboardInterrupt):
            break
        if not question:
            continue
        if question in {"exit", "quit"}:
            break
        await ask(agent, fs_ctx, question, history, args.max_turns)
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description="myagent：基于 openai-agents 的仓库分析助手")
    parser.add_argument("-q", "--question", default="", help="问一句就走；不传则进交互模式")
    parser.add_argument("--root", default="", help="工作根目录，默认用 config.env 里的 AGENT_ROOT")
    parser.add_argument("--max-turns", type=int, default=12, help="单次提问最多几轮，默认 12")
    parser.add_argument("--no-trace", action="store_true", help="关掉控制台的 trace 输出")
    args = parser.parse_args()

    try:
        return asyncio.run(_run(args))
    except ValueError as exc:
        print(f"[配置错误] {exc}")
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
