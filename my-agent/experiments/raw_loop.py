"""第 1 课：不用框架，手写一个 Agent。

为什么要有这个文件：
    SDK 的 `Runner.run()` 看起来很魔法。但它做的全部事情，就是本文件 step 3 的那个 for 循环。
    手写一遍之后，"框架"就退化成了"帮我处理了各种边界情况的同一个东西"——黑盒被拆开了。

运行（在 my-agent/ 目录下）：
    uv run python experiments/raw_loop.py --step 1    # 裸调用：一次请求，没有工具
    uv run python experiments/raw_loop.py --step 2    # 带工具：看模型怎么"说"要调工具
    uv run python experiments/raw_loop.py --step 3    # 完整循环：这就是 Agent

注意：这是学习沙盒，不是业务代码。跑完可以删，不影响项目。
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path

from openai import OpenAI

from myagent.runtime.config import load_settings
from myagent.tools.guards import safe_resolve

SYSTEM = "你是一个代码仓库分析助手。需要了解目录结构时，调用 list_dir 工具。"


# ════════════════════════════════════════════════════════════════
# ① 工具 = 一个普通 Python 函数
#
#    没有装饰器、没有 SDK 类型。SDK 的 @function_tool 最终也是产出这两样东西：
#       (a) 函数的 JSON Schema —— 给模型看
#       (b) 函数本身           —— 给你执行
#    这里我们两样都手写，就能看清楚它们的关系。
# ════════════════════════════════════════════════════════════════


def list_dir(root: Path, path: str = ".") -> str:
    """普通 Python 函数。你项目里的护栏在这里照样能用。"""
    target = safe_resolve(root, path)
    if not target.is_dir():
        return f"{path} 不是目录"
    rows = sorted(
        (p.name + "/" if p.is_dir() else p.name)
        for p in target.iterdir()
        if not p.name.startswith(".")
    )
    return f"目录 [{path}] 下有 {len(rows)} 个子项：\n" + "\n".join(rows)


# ════════════════════════════════════════════════════════════════
# ② 给模型的"工具说明书" —— 就是一个纯 JSON，没有魔法
#
#    对比 tools/repo.py 里的 @function_tool：SDK 是从 docstring 和类型注解
#    自动生成下面这段 JSON 的。手写一遍就知道它长什么样了。
# ════════════════════════════════════════════════════════════════

TOOLS_SCHEMA = [
    {
        "type": "function",
        "function": {
            "name": "list_dir",
            "description": (
                "列出某个目录下的直接子项，用来快速了解项目结构。"
                "想知道文件内容请改用 read_file。"
            ),
            "parameters": {
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": "相对于工作根目录的目录路径，默认 '.' 表示根目录",
                    }
                },
                "required": [],
            },
        },
    }
]


# ════════════════════════════════════════════════════════════════
# step 1：一次请求，没有工具。这是"普通 LLM 调用"，还不是 Agent。
# ════════════════════════════════════════════════════════════════


def step1(client: OpenAI, model: str) -> None:
    print("=== step 1：裸调用（无工具）===\n")
    resp = client.chat.completions.create(
        model=model,
        messages=[
            {"role": "system", "content": SYSTEM},
            {"role": "user", "content": "用一句话说明：Agent 和普通 LLM 调用差在哪？"},
        ],
    )
    choice = resp.choices[0]
    print("模型回答  :", choice.message.content)
    print("tool_calls:", choice.message.tool_calls, "  ← 一定是 None")
    print("finish原因:", choice.finish_reason)
    print("\n用量      :", resp.usage)


# ════════════════════════════════════════════════════════════════
# step 2：带上工具。看模型"说"要调什么 —— 它不执行，只是开口。
# ════════════════════════════════════════════════════════════════


def step2(client: OpenAI, model: str) -> None:
    print("=== step 2：带工具（模型只会「说」）===\n")
    resp = client.chat.completions.create(
        model=model,
        messages=[
            {"role": "system", "content": SYSTEM},
            {"role": "user", "content": "这个项目有哪些目录？"},
        ],
        tools=TOOLS_SCHEMA,
    )
    choice = resp.choices[0]
    print("模型 content :", repr(choice.message.content))
    print("finish 原因  :", choice.finish_reason, "  ← 变成 tool_calls 了")

    for tc in choice.message.tool_calls or []:
        print(f"\n  [工具请求]")
        print(f"    调用 ID : {tc.id}")
        print(f"    工具名  : {tc.function.name}")
        print(f"    参数    : {tc.function.arguments}")
        print(f"               ↑ 注意是【字符串】，不是 dict。模型只会吐文本。")

    print("\n" + "─" * 60)
    print("重要：到这一步为止，list_dir() 一次都没被执行过。")
    print("模型只是产出了一段结构化文本，内容是「请帮我调这个工具」。")
    print("执行 = 你的代码的责任。")


# ════════════════════════════════════════════════════════════════
# step 3：把 step 2 的东西包进 for 循环 —— 这就是 Agent 的全部。
# ════════════════════════════════════════════════════════════════

TOOL_IMPL = {"list_dir": list_dir}  # 工具名 → 真正的函数（SDK 内部也有这么一张表）


def step3(client: OpenAI, model: str, root: Path, max_turns: int = 8) -> None:
    print("=== step 3：完整循环 = 一个 Agent ===\n")
    messages: list = [
        {"role": "system", "content": SYSTEM},
        {"role": "user", "content": "这个项目有哪些目录？src 下面又有什么？"},
    ]
    total_in = total_out = 0

    for turn in range(1, max_turns + 1):
        print(f"--- 第 {turn} 轮 ---")

        # ① Reason：把 系统提示词 + 全部历史 + 工具清单 发给模型
        resp = client.chat.completions.create(
            model=model, messages=messages, tools=TOOLS_SCHEMA
        )
        msg = resp.choices[0].message
        total_in += resp.usage.prompt_tokens
        total_out += resp.usage.completion_tokens
        print(f"  prompt={resp.usage.prompt_tokens}  completion={resp.usage.completion_tokens}")

        # ② 收敛判断：模型不再要工具 → 结束
        if not msg.tool_calls:
            print("\n========== 最终回答 ==========")
            print(msg.content)
            print("==============================")
            print(f"累计：请求 {turn} 次 | prompt={total_in} | completion={total_out}")
            return

        # 把模型这轮的"话"放进历史。
        # ⚠️ 这行很容易漏：漏了的话，模型下一轮看不到自己刚说过要调什么工具。
        messages.append(msg)

        # ③ Act：执行工具 —— 真正干活的永远是你的代码
        for tc in msg.tool_calls:
            args = json.loads(tc.function.arguments)  # 字符串 → dict
            print(f"  执行 {tc.function.name}({args})")
            result = TOOL_IMPL[tc.function.name](root, **args)
            print(f"  返回 {len(result)} 字符")

            # ④ Observe：结果塞回历史，带上一模一样的 tool_call_id
            messages.append(
                {
                    "role": "tool",
                    "tool_call_id": tc.id,  # ← 必须和请求里的 id 对上，否则报错
                    "content": result,
                }
            )
        # 回到 ①

    print(f"\n达到 max_turns={max_turns} 熔断，没有收敛（生产上要告诉用户，而不是静默失败）")


def main() -> int:
    parser = argparse.ArgumentParser(description="第 1 课：手写最小 Agent")
    parser.add_argument("--step", type=int, choices=[1, 2, 3], default=1)
    parser.add_argument("--max-turns", type=int, default=8)
    args = parser.parse_args()

    settings = load_settings()
    client = OpenAI(base_url=settings.base_url, api_key=settings.api_key)
    print(f"[模型] {settings.model} @ {settings.base_url}")
    print(f"[根目录] {settings.root}\n")

    if args.step == 1:
        step1(client, settings.model)
    elif args.step == 2:
        step2(client, settings.model)
    else:
        step3(client, settings.model, settings.root, args.max_turns)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
