"""登录页/区域选择器汉化（JSX 文本节点，需精确匹配）。

与 apply_i18n_zh.py 的区别：
  那个脚本只匹配 `title: "Word"` 这类属性位置，安全但覆盖不到 JSX 文本。
  登录页文案是 <CardTitle>Sign in to your account</CardTitle> 形态，
  必须逐个精确替换——只动这 2 个文件，不做全局替换。
"""
import pathlib
import re
import sys

WEB = pathlib.Path("/Users/wangying/apps/sakelei/ai/tools/langfuse-src/web/src")

RULES = {
    "features/auth/SignInPage.tsx": [
        # 标题
        (r"Sign in to your account", "登录你的账户"),
        # 跨行提示语（保留缩进由 \s*\n\s* 吞掉后重新排版）
        (
            r"If you are experiencing issues signing in, please force refresh this\s*\n\s*"
            r"page \(CMD \+ SHIFT \+ R\) or clear your browser cache\.",
            "如果登录遇到问题，请强制刷新此页面（CMD + SHIFT + R）或清除浏览器缓存。",
        ),
        (r"\(contact us\)", "(联系我们)"),
        # 表单
        (r"<FormLabel>Email</FormLabel>", "<FormLabel>邮箱</FormLabel>"),
        (r'Password\{" "\}', '密码{" "}'),
        (r"\(forgot password\?\)", "(忘记密码？)"),
        # 提交按钮（三元表达式）
        (r'\? "Sign in" : "Continue"', '? "登录" : "继续"'),
        # 注册区
        (r'No account yet\?\{" "\}', '还没有账户？{" "}'),
        (r"(<Link\b[^>]*>\s*)Sign up(\s*</Link>)", r"\1注册\2"),
    ],
    "features/auth/components/CloudRegionPicker.tsx": [
        (r"(>\s*\n\s*)Data Region(\s*\n)", r"\1数据区域\2"),
        (r"\(what is this\?\)", "(这是什么？)"),
        (r"<DialogTitle>Data Regions</DialogTitle>", "<DialogTitle>数据区域</DialogTitle>"),
        (
            r"Demo project is not available in the HIPAA data region\.",
            "Demo 项目在 HIPAA 数据区域不可用。",
        ),
    ],
}


def main() -> int:
    failed = []
    for rel, rules in RULES.items():
        path = WEB / rel
        if not path.is_file():
            failed.append(f"{rel}: 文件不存在")
            continue
        text = original = path.read_text(encoding="utf-8")
        hits = []
        for pattern, repl in rules:
            new, n = re.subn(pattern, repl, text)
            hits.append((pattern[:42], n))
            text = new
        if text != original:
            path.write_text(text, encoding="utf-8")
        print(f"\n{rel}")
        for pat, n in hits:
            mark = "✓" if n else "✗"
            print(f"  {mark} {n}  {pat}...")
            if not n:
                failed.append(f"{rel}: 未匹配 {pat}")

    if failed:
        print("\n⚠️ 有未匹配项:")
        for f in failed:
            print("  ", f)
        return 1
    print("\n全部替换成功")
    return 0


if __name__ == "__main__":
    sys.exit(main())
