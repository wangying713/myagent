"""Run an upstream Agents SDK example as a module inside the lab wrapper.

Usage (from my-agent):
    uv run python -m myagent.runtime.lab experiments/run_official.py examples.basic.hello_world
"""
from __future__ import annotations

from pathlib import Path
import runpy
import sys
from opentelemetry import trace


def main() -> None:
    if len(sys.argv) < 2:
        raise SystemExit("用法：run_official.py examples.basic.hello_world [示例参数…]")

    module = sys.argv[1]
    repo_root = Path(__file__).resolve().parents[2] / "openai-agents-python"
    if not (repo_root / "examples").is_dir():
        raise SystemExit(f"找不到官方示例仓库：{repo_root}")

    # Module execution preserves package context for examples with relative imports.
    trace.get_current_span().set_attribute("app.run.target", module)
    sys.path.insert(0, str(repo_root))
    sys.argv = [module, *sys.argv[2:]]
    runpy.run_module(module, run_name="__main__", alter_sys=True)


if __name__ == "__main__":
    main()
