"""配置层：全项目**只有这里**读文件和环境变量。

好处：业务代码拿到的是一个不可变的 Settings 对象，
不用到处 os.getenv，也不会出现"这里读到了那里没读到"的诡异问题。
测试时直接 new 一个假的 Settings 就行。
"""

from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path

# config.py 的位置：<项目根>/src/myagent/runtime/config.py
PROJECT_ROOT = Path(__file__).resolve().parents[3]
CONFIG_PATH = PROJECT_ROOT / "config.env"

# 这些键允许被临时环境变量覆盖（越临时越优先，方便调试）
OVERRIDABLE = ("OPENAI_API_KEY", "OPENAI_BASE_URL", "MODEL", "AGENT_ROOT")


@dataclass(frozen=True)
class Settings:
    api_key: str
    base_url: str
    model: str
    root: Path


def _read_env_file(path: Path) -> dict[str, str]:
    cfg: dict[str, str] = {}
    if not path.exists():
        return cfg
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        cfg[key.strip()] = value.strip().strip('"').strip("'")
    return cfg


def load_settings(env_file: Path | None = None, root_override: str = "") -> Settings:
    """读配置并校验。校验不过就抛 ValueError，错误信息说清"去哪改"。"""
    cfg = _read_env_file(env_file or CONFIG_PATH)
    for key in OVERRIDABLE:
        if os.environ.get(key):
            cfg[key] = os.environ[key].strip()

    api_key = cfg.get("OPENAI_API_KEY", "")
    # 没填 / 还是占位符 / 掺了非 ASCII（粘贴常出问题）都在这里拦掉
    if not api_key.isascii() or "在这里" in api_key or len(api_key) < 20:
        raise ValueError(f"没有有效的 API Key，请打开 {CONFIG_PATH} 填写 OPENAI_API_KEY")

    # expanduser：让 --root ~/apps/xxx 这种写法也能用（Path 不会自动展开 ~）
    root = Path(root_override or cfg.get("AGENT_ROOT") or PROJECT_ROOT).expanduser().resolve()
    if not root.is_dir():
        raise ValueError(f"工作根目录不存在：{root}")

    return Settings(
        api_key=api_key,
        base_url=cfg.get("OPENAI_BASE_URL", "https://api.deepseek.com/v1"),
        model=cfg.get("MODEL", "deepseek-chat"),
        root=root,
    )
