"""文件访问护栏 + 工具上下文定义。

为什么叫 guards 而不是 sandbox：
SDK 里的 `sandbox` 专指"容器沙箱"（让 Agent 在隔离容器里跑长任务），
我们这里说的是"参数级的访问护栏"，不是一回事，名字撞车容易误导。
（这一点是同事 review 出来的，记一下。）

两个概念：

① FsContext —— 工具运行时需要的外部依赖（现在只有"工作根目录"）。
   通过 SDK 的 RunContextWrapper 注入，**不用全局变量**。
   好处：同一个进程里可以同时跑多个不同根目录的 Agent（多项目/多租户），
   而且工具函数是纯函数，测试时直接传一个假 root 就行。

② 护栏函数 —— 所有文件类工具都必须先过这两关，别在各自工具里各写一遍。
"""

from __future__ import annotations

import re
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class FsContext:
    """文件工具的运行上下文。将来要加什么依赖，往这里加。"""

    root: Path


# 检索时跳过的目录
SKIP_DIR = {
    ".git",
    "node_modules",
    "vendor",
    "dist",
    "build",
    "__pycache__",
    ".venv",
    ".idea",
    ".vscode",
    ".mypy_cache",
    ".ruff_cache",
}

# 凭据类文件绝不进上下文：Agent 读到的东西 = 会流进 prompt 发给远端 + 落进 trace
SENSITIVE = re.compile(r"(^config\.env$|\.env$|id_rsa|\.pem$|\.key$|credential|secret)", re.I)

MAX_FILE_BYTES = 512 * 1024
MAX_SEARCH_HITS = 30


def human_size(size: int) -> str:
    value = float(size)
    for unit in ("B", "KB", "MB", "GB"):
        if value < 1024:
            return f"{value:.1f} {unit}"
        value /= 1024
    return f"{value:.1f} TB"


def safe_resolve(root: Path, path: str) -> Path:
    """把路径锁死在 root 之内。绝对路径直接拒、越界直接拒。"""
    if Path(path).is_absolute():
        raise ValueError(f"只接受相对路径，收到的是绝对路径：{path}")
    target = (root / path).resolve()
    if target != root and root not in target.parents:
        raise ValueError(f"路径越界：{path} 跳出了工作根目录，已拒绝")
    return target


def reject_sensitive(target: Path) -> None:
    """凭据文件一律不让读 —— 在工具层拦，不指望模型自觉。"""
    if SENSITIVE.search(target.name):
        raise ValueError(
            f"拒绝访问 {target.name}：该文件可能是凭据文件，已被安全策略拦截。请读取其它文件"
        )


def walk_files(root: Path, base: Path):
    """收集待检索的文件，顺手跳过噪音目录、敏感文件、超大文件。"""
    if base.is_file():
        yield base
        return
    for path in sorted(base.rglob("*")):
        if any(part in SKIP_DIR or part.startswith(".") for part in path.parts):
            continue
        if not path.is_file() or SENSITIVE.search(path.name):
            continue
        try:
            if path.stat().st_size > MAX_FILE_BYTES:
                continue
        except OSError:
            continue
        yield path
