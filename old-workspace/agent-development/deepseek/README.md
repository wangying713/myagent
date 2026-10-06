# 用 DeepSeek 学习 Agent 开发

这个项目从直接调用 DeepSeek API 开始，暂时不使用 Agent 框架。先看清框架背后封装了哪些步骤。

## 准备环境

1. 运行 `cp .env.example .env`，再编辑 `.env` 填入 DeepSeek API Key。
2. 在本目录运行 `uv sync` 安装依赖。
3. 运行 `uv run python 01_chat.py`，观察一次普通模型调用。
4. 运行 `uv run python 02_agent_loop.py`，观察模型请求本地工具、Python 执行工具，再把结果交还模型。

第一个示例只是模型调用。第二个示例才是 Agent 循环：模型可以请求使用工具，但不会亲自执行 Python 函数；程序检查请求后，调用对应的本地函数。

天气工具返回固定的演示数据，不会访问天气服务。

## 学习顺序

先看 `01_chat.py` 发给模型的 `messages` 和模型回复。再按顺序读 `02_agent_loop.py`：工具说明、模型发起工具调用、本地分发、工具结果、模型下一轮回复。理解这些步骤后，再对照 Agents SDK 的对应示例。
