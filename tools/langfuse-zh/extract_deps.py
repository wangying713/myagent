"""从 Next.js standalone 产物中提取 Turbopack external 模块依赖（含传递闭包）。

背景：
  .next/node_modules 下是符号链接，指向 standalone 的 .pnpm/<pkg>。
  pnpm 采用隔离结构：每个包的依赖也是 .pnpm/<dep> 的同级符号链接。
  因此必须做「传递闭包」收集，只复制直接引用会漏掉 d3-array 这类二级依赖。

用法：python3 extract_deps.py [--full]
  --full  直接搬运整个 .pnpm（230M），跳过递归分析
"""
import os
import pathlib
import shutil
import sys

STANDALONE = pathlib.Path(
    "/Users/wangying/apps/sakelei/ai/tools/langfuse-src/web/.next/standalone"
)
PNPM = STANDALONE / "node_modules" / ".pnpm"
NEXT_NM = STANDALONE / "web" / ".next" / "node_modules"
OUT = pathlib.Path("/tmp/lfzh-ctx/pnpm")


def pkg_of(abs_target):
    """从任意路径定位它所属的 .pnpm/<pkg> 目录。"""
    parts = abs_target.parts
    if ".pnpm" not in parts:
        return None
    i = parts.index(".pnpm")
    if i + 1 >= len(parts):
        return None
    return pathlib.Path(*parts[: i + 2])


def deps_of(pkg_dir):
    """返回 pkg_dir 内部 node_modules 链接指向的其它包。"""
    found = []
    nm = pkg_dir / "node_modules"
    if not nm.is_dir():
        return found
    for entry in nm.iterdir():
        if not entry.is_symlink():
            continue
        try:
            t = (nm / os.readlink(entry)).resolve()
        except OSError:
            continue
        p = pkg_of(t)
        if p and p != pkg_dir:
            found.append(p)
    return found


def copy_pkg(pkg_dir: pathlib.Path) -> bool:
    dest = OUT / pkg_dir.name
    if dest.exists() or not pkg_dir.is_dir():
        return False
    shutil.copytree(pkg_dir, dest, symlinks=True)
    return True


def main() -> int:
    if "--full" in sys.argv:
        if OUT.exists():
            shutil.rmtree(OUT)
        shutil.copytree(PNPM, OUT, symlinks=True)
        print("已搬运整个 .pnpm")
    else:
        OUT.mkdir(parents=True, exist_ok=True)

        # 1) 种子：.next/node_modules 直接引用的包
        queue, copied = [], 0
        for entry in sorted(NEXT_NM.iterdir()):
            if not entry.is_symlink():
                continue
            t = (NEXT_NM / os.readlink(entry)).resolve()
            p = pkg_of(t)
            if p:
                queue.append(p)

        # 2) BFS 展开传递依赖
        seen: set[str] = set()
        while queue:
            p = queue.pop()
            if p.name in seen:
                continue
            seen.add(p.name)
            if copy_pkg(p):
                copied += 1
            queue.extend(deps_of(p))

        print(f"已提取 {copied} 个包（传递闭包）-> {OUT}")

    total = sum(f.stat().st_size for f in OUT.rglob("*") if f.is_file())
    print(f"体积: {total / 1024 / 1024:.1f} MB")
    return 0


if __name__ == "__main__":
    sys.exit(main())
