"""Langfuse B2 首批汉化：只替换 UI 属性位置的英文字符串。

安全约束：
  1. 只匹配 (title|header|label|name|placeholder|description|tooltip): "Word" 形态
  2. 跳过测试、stories、node_modules
  3. 绝不触碰 Xxx.Traces 这类枚举成员（本模式天然不匹配，因为要求前面是 attr:）
"""
import pathlib
import re
import sys

# 与 langfuse-zh/translations/zh.json 保持一致；未覆盖的取自社区 issue #12890
MAPPING = {
    "Traces": "追踪",
    "Observations": "观测",
    "Dashboard": "仪表盘",
    "Settings": "设置",
    # 以下沿用已有词典
    "Scores": "评分",
    "Sessions": "会话",
    "Prompts": "提示词",
    "Datasets": "数据集",
    "Users": "用户",
    # ── 主导航（routes.tsx）──
    "Organizations": "组织",
    "Projects": "项目",
    "Home": "首页",
    "Dashboards": "仪表盘",
    "Tracing": "调用链",
    "Alerts": "告警",
    "Playground": "演练场",
    "Evaluators": "评估器",
    "Human Annotation": "人工标注",
    "Experiments": "实验",
    "Cloud Status": "云状态",
    "Upgrade Plan": "升级套餐",
    "Support": "支持",
}

ATTRS = r"(?:title|header|label|name|placeholder|description|tooltip|breadcrumb)"
PATTERN = re.compile(
    r"(" + ATTRS + r":\s*)" + r'"(' + "|".join(MAPPING.keys()) + r')"'
)

SKIP_MARKERS = (".test.", "clienttest", ".stories.", "node_modules")


def main() -> int:
    root = pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else "web/src")
    total = 0
    changed = []

    for f in sorted(root.rglob("*.tsx")):
        s = str(f)
        if any(m in s for m in SKIP_MARKERS):
            continue
        try:
            text = f.read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError):
            continue

        def repl(m: re.Match) -> str:
            return m.group(1) + '"' + MAPPING[m.group(2)] + '"'

        new_text, n = PATTERN.subn(repl, text)
        if n:
            f.write_text(new_text, encoding="utf-8")
            total += n
            changed.append((s, n))

    print(f"替换 {total} 处，涉及 {len(changed)} 个文件\n")
    for s, n in changed:
        print(f"  {n:>2}  {s}")
    return total


if __name__ == "__main__":
    sys.exit(0 if main() else 1)
