# 初级：看懂模型调用，手写第一个 Agent

**目标：**能独立写出“构造消息 → 调用模型 → 执行受控工具 → 回填结果 → 停止”的程序。

**前置：**Python 字典、列表、函数、异常和 `with`。遇到不熟的语法可以先查本节代码，不需要先学习机器学习训练、向量数据库或框架。

所有命令在 `agent-learning/` 目录执行。已有 `.env` 不要覆盖。先运行 `uv sync --locked` 和 `uv run python verify_observe.py`，确保观测记录能查回。详见 [观测说明](../openobserve.md)。

## B01：第一次 HTTP 调用

- 文档：[一次模型调用](../lessons/01-http.md)。
- 运行：`uv run python -m demos.beginner.beginner_01_chat`。
- 阅读顺序：`demos/beginner/beginner_01_chat.py` → `llm.py` 的 `chat()` 和 `_body()`。
- 本节只有一次真实模型请求，正常 trace 为根节点 + 模型节点，共 2 条。

先找出 URL、Authorization header、请求 body 和响应 body。再把问题从“递归”换成“函数”，只改一个变量，比较 `app.request` 与 `app.response`。接着把 `max_tokens` 临时设为 1，观察为什么 HTTP 成功仍可能没有完整答案，实验结束后恢复。

**通过标准：**能从响应找到正文、结束原因和 token 用量；能解释正文和鉴权为什么要分开；能找到自己的 trace。终端输出与 trace 正文不是同一个东西，观测会脱敏和限长。

## B02：多轮对话与上下文

- 文档：[消息、上下文与 token](../lessons/02-messages.md)。
- 运行：`uv run python -m demos.beginner.beginner_02_messages`。
- 阅读：`messages.append()` 在请求前后各保存了什么。
- 默认 2 次模型请求，正常共 3 个 span。

观察第二次请求是否包含第一轮 user 和 assistant 消息。临时去掉历史，只传最后一句，再观察回答。此时所谓“忘记”首先是应用没有传入材料，不能直接归因于模型能力。

**通过标准：**能画出 messages 随轮次增长的过程；能区分一次请求上下文、应用保存的历史和跨进程记忆；能说明为什么后续轮次通常输入更多 token。

## B03：工具与循环

- 文档：[工具与手写 Agent](../lessons/03-agent-loop.md)。
- 运行：`uv run python -m demos.beginner.beginner_03_agent_loop`。
- 阅读顺序：`toolbox.py` 的 schema → `dispatch()` → `demos/beginner/beginner_03_agent_loop.py` 的 `run_agent()`。
- 典型链路：模型 → multiply → 模型，加根节点共 4 条 span。轨迹可以变化。

先运行 17×23，再改成 12×8。检查模型给出的 arguments、程序真正执行的结果、下一轮的 tool 消息。再将最大轮次设为 1，理解工具执行了并不意味着模型已完成任务。

**通过标准：**能解释 `tool_call_id` 如何配对；不会把模型生成字符串直接执行；能处理一轮多个工具；达到次数上限时明确报告未完成。

## 初级验收

```bash
uv run python verify_live.py --level beginner
```

通常共 5 次真实请求。验收会检查问答、多轮暗号、乘法最终答案和实际工具结果，并确认所有节点入库与父子关系。不能仅凭终端“没有报错”判断通过。

完成后，尝试不看原文件重写一个小循环，但模型调用仍用 `DeepSeekClient`，以保留观测。不要先阅读整个 `telemetry.py`；它是基础设施，不是本级学习重点。

下一阶段：[中级](intermediate.md)。
