# Agent 开发底层实验：初级 / 中级 / 高级

完整入口：[学习文档](../doc/README.md)。全程普通 Python + HTTP，所有真实模型交互统一上报 OpenObserve。代码注释与格式按[项目 Python 规范](../doc/python-style.md)维护。

| 级别 | Demo | 学习说明 |
|---|---|---|
| 初级（3） | `demos/beginner/beginner_01_chat.py`、`demos/beginner/beginner_02_messages.py`、`demos/beginner/beginner_03_agent_loop.py` | [初级](../doc/levels/beginner.md) |
| 中级（5） | `demos/intermediate/intermediate_01_json.py`、`demos/intermediate/intermediate_02_stream.py`、`demos/intermediate/intermediate_03_reliability.py`、`demos/intermediate/intermediate_04_rag.py`、`demos/intermediate/intermediate_05_memory.py` | [中级](../doc/levels/intermediate.md) |
| 高级（4） | `demos/advanced/advanced_01_thinking.py`、`demos/advanced/advanced_02_recovery.py`、`demos/advanced/advanced_03_evaluation.py`、`demos/advanced/advanced_04_knowledge_agent.py` | [高级](../doc/levels/advanced.md) |

## 从这里开始

```bash
uv sync --locked
uv run python verify_observe.py
uv run python -m demos.beginner.beginner_01_chat
```

使用已有 `.env`；新机器才从 `.env.example` 复制并填写配置。demo 按级别放在 `demos/beginner/`、`demos/intermediate/`、`demos/advanced/`，从本目录使用 `uv run python -m demos.级别.模块名` 运行。例如：`uv run python -m demos.beginner.beginner_01_chat`。共享客户端、工具、观测和配置代码留在本目录，课程仍不依赖 Agent 框架。

## 检查与验收

```bash
# 完全离线、不消耗 token、不向 OpenObserve 写测试数据
uv run python -m unittest discover -s tests -v
uv run python -m demos.intermediate.intermediate_03_reliability
uv run python -m demos.advanced.advanced_02_recovery
uv run python -m demos.advanced.advanced_03_evaluation

# 分级真实验收，会消耗 token 并核对入库
uv run python verify_live.py --level beginner
uv run python verify_live.py --level intermediate
uv run python verify_live.py --level advanced

# 全部三级 + 四个真实评估用例，通常共 21 次模型请求
uv run python verify_live.py --level all --evaluate --report .local/verification-levels.json
```

`demos/intermediate/intermediate_05_memory.py` 默认使用临时 SQLite，跨进程保存方式见中级文档。`demos/advanced/advanced_03_evaluation.py` 默认使用假模型，`--live` 才测试真实 DeepSeek；不要把离线通过率解释成模型准确率。`.local/` 存本地练习数据和报告，不提交 Git。
