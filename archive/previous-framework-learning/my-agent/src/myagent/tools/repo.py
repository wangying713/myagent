"""代码仓库相关的工具（业务域：repo）。

这是"工具 = handler"的示范：每个函数只做薄转换，
真正的护栏在 guards.py，将来真正的业务逻辑放 services/。

注意第一个参数 ctx：RunContextWrapper[FsContext]。
SDK 会把运行时上下文注入进来，**这个参数不会出现在给模型的 Schema 里**。
所以工具不需要知道"根目录是什么"，也就不需要全局变量。
"""

import re

from agents import RunContextWrapper, function_tool

from .guards import (
    MAX_FILE_BYTES,
    MAX_SEARCH_HITS,
    FsContext,
    human_size,
    reject_sensitive,
    safe_resolve,
    walk_files,
)


@function_tool
def list_dir(ctx: RunContextWrapper[FsContext], path: str = ".") -> str:
    """列出某个目录下的直接子项，用来快速了解项目结构。

    当你想知道"这个项目有哪些文件、某个目录里有什么"时用它。
    想读某个文件的具体内容请改用 read_file；想按关键字找东西请改用 search_content。

    Args:
        path: 相对于工作根目录的目录路径，默认 "." 表示根目录本身。
    """
    root = ctx.context.root
    target = safe_resolve(root, path)
    if not target.is_dir():
        raise ValueError(f"{path} 不是一个目录，请改用 read_file 读取文件内容")

    rows: list[str] = []
    for entry in sorted(target.iterdir(), key=lambda e: (e.is_file(), e.name)):
        if entry.name.startswith(".") or entry.name in {".git", "node_modules", "__pycache__", ".venv"}:
            continue
        if entry.is_dir():
            rows.append(f"  {entry.name}/  (目录)")
        else:
            rows.append(f"  {entry.name}  (文件, {human_size(entry.stat().st_size)})")

    if not rows:
        return f"目录 [{path}] 下没有可展示的子项"
    return f"目录 [{path}] 下共 {len(rows)} 个直接子项：\n" + "\n".join(rows[:200])


@function_tool
def read_file(ctx: RunContextWrapper[FsContext], path: str, offset: int = 1, limit: int = 200) -> str:
    """读取文本文件内容，返回带行号的正文。

    不确定该读哪个文件时，先用 search_content 按关键字定位。
    如果返回里提示"已省略后续行"，就再用 offset 接着往下读。

    Args:
        path: 相对于工作根目录的文件路径。
        offset: 从第几行开始读，从 1 开始。
        limit: 本次最多返回多少行，默认 200，上限 500。
    """
    root = ctx.context.root
    target = safe_resolve(root, path)
    if not target.exists():
        raise ValueError(f"文件不存在：{path}。请先用 list_dir 确认路径和大小写")
    if target.is_dir():
        raise ValueError(f"{path} 是目录，请改用 list_dir 列出它的子项")
    reject_sensitive(target)

    size = target.stat().st_size
    if size > MAX_FILE_BYTES:
        raise ValueError(
            f"文件太大（{human_size(size)}），超过 {human_size(MAX_FILE_BYTES)} 上限；"
            "请改用 search_content 定位关键内容"
        )

    offset = max(1, int(offset))
    limit = max(1, min(int(limit), 500))
    lines = target.read_text(encoding="utf-8", errors="replace").splitlines()
    chunk = lines[offset - 1 : offset - 1 + limit]
    if not chunk:
        return f"文件 [{path}] 共 {len(lines)} 行，offset={offset} 已超出末尾，请把 offset 调小"

    body = "\n".join(f"{offset + i:>5}| {line}" for i, line in enumerate(chunk))
    rest = len(lines) - (offset - 1 + len(chunk))
    tail = ""
    if rest > 0:
        # 提示里给出"下一步动作"，模型下一轮会自己照做
        tail = f"\n[已省略后续 {rest} 行。如需继续，请再调用 read_file 并传 offset={offset + limit}]"
    return (
        f"文件 [{path}]（共 {len(lines)} 行，当前显示 {offset}-{offset + len(chunk) - 1} 行）\n"
        f"{body}{tail}"
    )


@function_tool
def search_content(ctx: RunContextWrapper[FsContext], pattern: str, path: str = ".") -> str:
    """按正则表达式搜索文件内容，返回「文件:行号: 匹配内容」。

    用来回答"这个函数在哪定义、谁调用了它、这个配置在哪配的"。
    只看目录结构请用 list_dir；看某个文件全文请用 read_file。

    Args:
        pattern: 正则表达式，例如 "def main" 或 "class\\s+\\w+"。
        path: 从哪个子目录开始搜，默认 "." 表示整个工作根目录。
    """
    root = ctx.context.root
    try:
        regex = re.compile(pattern)
    except re.error as exc:
        raise ValueError(f"正则表达式不合法（{exc}）：{pattern}。请传一个合法的正则") from exc

    hits: list[str] = []
    scanned = 0
    for path_obj in walk_files(root, safe_resolve(root, path)):
        scanned += 1
        try:
            text = path_obj.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        for no, line in enumerate(text.splitlines(), start=1):
            if regex.search(line):
                hits.append(f"{path_obj.relative_to(root)}:{no}: {line.strip()[:160]}")
                if len(hits) >= MAX_SEARCH_HITS:
                    break
        if len(hits) >= MAX_SEARCH_HITS:
            break

    if not hits:
        return f"没有匹配 [{pattern}] 的内容（已扫描 {scanned} 个文件）"
    more = ""
    if len(hits) >= MAX_SEARCH_HITS:
        more = f"\n[结果已截断到 {MAX_SEARCH_HITS} 条。请把 pattern 写得更精确，或用 path 缩小范围]"
    return f"匹配 [{pattern}] 共 {len(hits)} 条（扫描 {scanned} 个文件）：\n" + "\n".join(hits) + more


# 这个业务域对外暴露的工具清单。
# 加新工具就往这个列表里加 —— 别的地方不用改。
REPO_TOOLS = [list_dir, read_file, search_content]
