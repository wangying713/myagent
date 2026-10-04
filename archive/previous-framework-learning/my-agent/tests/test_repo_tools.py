"""工具层单测：不联网、不花 token、毫秒级返回。

这就是分层最直接的回报 —— 工具是普通 Python 函数，
可以像写后端代码一样给它写测试，不需要真模型参与。

跑：uv run pytest -q
"""

import pytest

from myagent.tools.repo import REPO_TOOLS, list_dir, read_file
from myagent.tools.guards import reject_sensitive, safe_resolve


# ── 护栏：这些是"必须拦住"的，所以每条都写一个用例 ──


def test_safe_resolve_rejects_absolute_path(tmp_path):
    with pytest.raises(ValueError, match="绝对路径"):
        safe_resolve(tmp_path, "/etc/passwd")


def test_safe_resolve_rejects_escape(tmp_path):
    with pytest.raises(ValueError, match="越界"):
        safe_resolve(tmp_path, "../../etc/passwd")


def test_safe_resolve_allows_normal_path(tmp_path):
    (tmp_path / "sub").mkdir()
    (tmp_path / "sub" / "a.txt").write_text("hi", encoding="utf-8")
    assert safe_resolve(tmp_path, "sub/a.txt").name == "a.txt"


def test_reject_sensitive_blocks_credentials(tmp_path):
    for name in ("config.env", ".env", "id_rsa", "server.pem"):
        target = tmp_path / name
        target.write_text("secret", encoding="utf-8")
        with pytest.raises(ValueError, match="拒绝访问"):
            reject_sensitive(target)


def test_reject_sensitive_allows_normal_file(tmp_path):
    target = tmp_path / "readme.txt"
    target.write_text("ok", encoding="utf-8")
    reject_sensitive(target)  # 不抛异常即通过


# ── 工具元信息：模型能不能看懂它，靠的就是这几个字段 ──


def test_tools_have_model_facing_documentation():
    for tool in REPO_TOOLS:
        assert tool.name, "工具必须有名字"
        assert tool.description and len(tool.description) > 20, (
            f"{tool.name} 缺少给模型看的说明，模型会选错工具"
        )


def test_context_param_is_injected_not_exposed():
    """ctx 参数由 SDK 注入，绝不能出现在给模型的 Schema 里。"""
    for tool in REPO_TOOLS:
        props = tool.params_json_schema.get("properties", {})
        assert "ctx" not in props, f"{tool.name} 把上下文参数漏给模型了"
        assert props, f"{tool.name} 没有参数"


def test_read_file_declares_paging_params():
    """翻页参数必须有默认值，模型的调用负担才轻。"""
    props = read_file.params_json_schema["properties"]
    assert "offset" in props and "limit" in props
    assert props["offset"]["default"] == 1
    assert props["limit"]["default"] == 200


def test_list_dir_has_default_path():
    props = list_dir.params_json_schema["properties"]
    assert props["path"]["default"] == "."
