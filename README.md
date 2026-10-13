# Python 与 Agent 开发学习

用普通 Python 和 HTTP 调用 DeepSeek，手写 Agent 循环，并通过 OpenObserve 查看模型请求、工具执行和运行结果。当前课程包含初级、中级、高级共 10 个 demo，另有独立的 Python 标准库练习。

本文件是当前项目的统一入口，集中说明安装、运行、配置和代码分工。具体原理与练习保留在 `doc/`，历史材料保留在 `old-workspace/`。

## 快速开始

需要 Python 3.11+、`uv`、DeepSeek API key 和可用的 OpenObserve 服务。本机按以下命令运行；其他机器将路径换成自己的 `agent-learning/` 目录：

```bash
cd /Users/wangying/apps/sakelei/ai/agent-learning
uv sync --locked

# 只验证观测链路，不调用模型
uv run python verify_observe.py

# 运行第一个真实模型示例
uv run python -m demos.beginner.beginner_01_chat
```

配置文件为 [agent-learning/.env.example](agent-learning/.env.example)。初次使用时复制为同目录的 `.env`，填写模型密钥和观测鉴权；已有 `.env` 时保留原文件。环境变量优先于 `.env`，配置路径不随启动目录变化。

问答示例会打印 `trace_id`。打开 [OpenObserve](http://localhost:5080/web/)，进入 Traces，选择 `default` stream 和最近的时间范围，按 trace ID 查找记录。普通问答包含 2 个 span，典型的单工具 Agent 包含 4 个。也可在命令行确认入库：

```bash
uv run python verify_observe.py --trace-id 实际32位trace_id --expect 2
```

真实模型 demo 和验收会消耗 token；离线测试使用假 HTTP 和内存观测。观测接入与排错见 [OpenObserve 文档](doc/openobserve.md)。

没有 `uv` 时，在 `agent-learning/` 目录执行 `python3 -m venv .venv`、`.venv/bin/python -m pip install -e ..`，随后用 `.venv/bin/python` 替代命令中的 `uv run python`。

## 课程与运行命令

以下命令均在 `agent-learning/` 目录执行。按初级 → 中级 → 高级、每级文件编号的顺序学习；各级文档提供阅读顺序、练习和通过标准。

| 级别 | 学习说明 | 内容 |
|---|---|---|
| 初级 | [初级课程](doc/levels/beginner.md) | HTTP、消息历史、工具协议、手写循环 |
| 中级 | [中级课程](doc/levels/intermediate.md) | JSON、流式输出、关键词 RAG、SQLite 会话 |
| 高级 | [高级课程](doc/levels/advanced.md) | 思考模式、固定用例评估、只读资料 Agent |

| 编号 | 运行命令 | 重点 |
|---|---|---|
| B01 | `uv run python -m demos.beginner.beginner_01_chat` | 请求、回答、用量和结束原因 |
| B02 | `uv run python -m demos.beginner.beginner_02_messages` | 第二轮请求携带第一轮消息历史 |
| B03 | `uv run python -m demos.beginner.beginner_03_agent_loop` | 工具执行、结果回填、有限循环 |
| I01 | `uv run python -m demos.intermediate.intermediate_01_json` | JSON 解析和业务校验 |
| I02 | `uv run python -m demos.intermediate.intermediate_02_stream` | 流式消息重组和观测 |
| I03 | `uv run python -m demos.intermediate.intermediate_03_rag` | 本地关键词检索、生成和引用校验 |
| I04 | `uv run python -m demos.intermediate.intermediate_04_memory` | SQLite 历史、会话隔离、版本冲突 |
| A01 | `uv run python -m demos.advanced.advanced_01_thinking` | 思考模式、答案验算、资源消耗 |
| A02 | `uv run python -m demos.advanced.advanced_02_evaluation` | 答案、实际工具证据和预算评估 |
| A03 | `uv run python -m demos.advanced.advanced_03_knowledge_agent` | 模型选择搜索词、工具回填、引用校验 |

A02 依次评估 `17×23`、`0×25`、`-3×8`、`12×8`，逐条输出 JSON 和 trace ID。全部通过时显示 `通过 4/4；真实模型评估。`，任一用例失败时退出码为 1；模型请求错误的具体说明见 `error_message`。

I04 默认使用临时 SQLite；跨进程保存历史的运行方式见中级课程。`.local/` 保存本地会话和验收报告，按需创建，不提交 Git。

## 测试与验收

以下命令仍在 `agent-learning/` 目录执行：

```bash
# 离线回归：不调用模型，不向 OpenObserve 写数据
uv run python -m unittest discover -s tests -v

# 按级真实验收：调用模型，并查回 OpenObserve 核对完整记录
uv run python verify_live.py --level beginner
uv run python verify_live.py --level intermediate
uv run python verify_live.py --level advanced

# 全部三级及四个评估用例，并保存报告
uv run python verify_live.py --level all --evaluate --report .local/verification-levels.json
```

`verify_live.py` 不带 `--level` 时默认验收全部三级，通常需要 13 次模型请求；加上 `--evaluate` 通常再增加 8 次。学习时按需运行单个 demo 或单级验收即可。已有结果与范围见 [验收记录](doc/verification.md) 和 [Review 记录](doc/review.md)。

## 配置

共享客户端默认读取 `agent-learning/.env`；真实配置被 Git 忽略。以下是代码默认值，已有环境变量或 `.env` 值会覆盖它们：

| 变量 | 用途或默认值 |
|---|---|
| `DEEPSEEK_API_KEY` | DeepSeek API 密钥 |
| `DEEPSEEK_BASE_URL` | `https://api.deepseek.com/v1`，客户端追加 `/chat/completions` |
| `DEEPSEEK_MODEL` | `deepseek-flash` |
| `DEEPSEEK_TIMEOUT` | 读取、写入和连接池等待超时，默认 60 秒；连接超时为 10 秒 |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OpenObserve 组织地址，默认 `http://localhost:5080/api/default` |
| `OTEL_EXPORTER_OTLP_HEADERS` | 观测鉴权，例如 `Authorization=Basic <凭据>` |
| `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` / `OTEL_EXPORTER_OTLP_TRACES_HEADERS` | 可选的 traces 专用配置，优先于上述通用配置；端点以 `/v1/traces` 结尾 |
| `OTEL_SERVICE_NAME` | 服务名，默认 `agent-learning` |
| `TELEMETRY_ENABLED` | 是否上报观测，默认 `true` |
| `TELEMETRY_CAPTURE_CONTENT` | 是否采集请求、响应和工具正文，默认 `true`；关闭时保留元信息 |
| `TELEMETRY_CONTENT_MAX_CHARS` | 每份正文长度上限，默认 32768 字符，超出明确标记截断 |

`DEEPSEEK_TIMEOUT` 是单项网络操作的超时，不是整个 Agent 任务的硬截止时间。模型能力和接口参数以 [DeepSeek 官方文档](https://api-docs.deepseek.com/) 与实际请求结果为准。

## 目录与代码分工

```text
ai/
├── README.md            # 安装、运行、配置与学习导航
├── agent-learning/      # 10 个 demo、业务工具、资料、测试与验收脚本
│   ├── demos/           # beginner / intermediate / advanced
│   ├── toolbox.py       # 乘法工具
│   ├── retrieval.py     # 本地资料检索与引用校验
│   ├── data/            # 虚构知识库与评估用例
│   ├── tests/           # 离线回归测试
│   ├── offline_lab.py   # 测试用假 HTTP 与内存观测
│   ├── verify_observe.py
│   ├── verify_live.py
│   ├── pyproject.toml
│   ├── uv.lock
│   └── .env.example
├── agent_toolkit/       # 共享 Python 包
├── doc/                 # 分级课程、原理、观测与代码规范
├── python-learning/     # 独立的 Python 标准库练习
├── old-workspace/       # 历史框架练习、第三方源码与旧资料
├── pyproject.toml       # 共享工具包的安装配置
├── pyrightconfig.json   # 类型检查配置
└── ruff.toml            # 格式、导入排序与命名检查
```

| 共享模块 | 职责 |
|---|---|
| [llm.py](agent_toolkit/llm.py) | DeepSeek HTTP 请求、普通与流式响应、token 统计、工具执行观测 |
| [agent_loop.py](agent_toolkit/agent_loop.py) | 模型与工具循环、消息回填、轮数与调用次数限制、token 软预算 |
| [tools.py](agent_toolkit/tools.py) | 生成工具描述、校验参数、分发调用 |
| [settings.py](agent_toolkit/settings.py) | 加载与校验模型、观测配置，支持指定 `.env` 路径 |
| [telemetry.py](agent_toolkit/telemetry.py) | span 生命周期、脱敏、正文限长、OTLP 上报 |
| [session_store.py](agent_toolkit/session_store.py) | SQLite 会话历史、会话隔离、版本冲突检查 |

`httpx` 发送模型请求，`python-dotenv` 读取配置，OpenTelemetry 负责观测；`langchain-core` 仅用于工具声明、schema 生成和参数校验。消息管理和 Agent 循环由本项目实现。

课程通过本地可编辑依赖导入 `agent_toolkit`，`uv sync --locked` 会完成安装；修改共享源码后无需反复安装。具体提示词、乘法工具和资料检索保留在课程代码中。

### 复用 Agent 循环

在课程目录中可以直接导入共享循环并传入工具列表：

```python
from agent_toolkit.agent_loop import run_agent
from agent_toolkit.llm import DeepSeekClient
from toolbox import multiply

with DeepSeekClient("my_experiment") as llm_client:
    answer = run_agent(
        llm_client,
        "计算 12 乘以 8，最终只输出数字。",
        system_prompt="你是计算助手。乘法必须调用工具，最终只输出数字。",
        tools=[multiply],
    )
    print(answer)
```

业务工具使用 `@tool` 装饰器、普通类型注解和默认值声明。调用方传 `tools=[tool_a, tool_b]`，内部 `ToolSet` 在进入循环前准备描述和执行映射，并统一执行严格类型校验、拒绝多余参数与非有限数值。`float` 参数接受整数和浮点数；业务有额外限制时再显式声明约束。协议说明见 [工具与 Agent 循环](doc/lessons/03-agent-loop.md)。

其他项目可以在自己的虚拟环境中安装仓库根目录的包，通过 `Settings.from_env(env_path="/path/to/project/.env")` 指定配置，再传给 `DeepSeekClient(..., settings=settings)`。客户端在 `with` 块内使用，退出时关闭 HTTP 连接并结束、刷出观测记录。

## 原理与专题文档

| 文档 | 内容 |
|---|---|
| [一次模型调用](doc/lessons/01-http.md) | HTTP 请求、响应和客户端生命周期 |
| [消息、上下文与 token](doc/lessons/02-messages.md) | 消息历史与用量 |
| [工具与 Agent 循环](doc/lessons/03-agent-loop.md) | 工具协议、执行与回填 |
| [结构化输出](doc/lessons/04-json.md) | JSON 和业务校验 |
| [流式与思考模式](doc/lessons/05-stream-thinking.md) | 流式重组、思考模式 |
| [失败、预算和安全](doc/lessons/06-reliability.md) | 错误处理与运行限制 |
| [检索、记忆与状态](doc/lessons/07-retrieval-memory.md) | 关键词 RAG、会话存储与状态 |
| [评估与交付](doc/lessons/08-evaluation.md) | 评估、验收与后续扩展 |
| [OpenObserve](doc/openobserve.md) | trace 查看、正文采集与排错 |
| [Python 代码规范](doc/python-style.md) | Google Python 命名、注释与 Ruff 配置 |

中级 RAG 由程序固定先检索再生成；高级资料 Agent 由模型选择搜索词，通过只读工具执行检索。当前课程是单机教学实现；向量库、自动长期记忆、分布式任务队列和生产部署属于后续扩展。

## Python 标准库练习

以下命令在仓库根目录执行，只使用标准库，无需 API key，不调用模型或上报观测：

```bash
python3 python-learning/with_simple.py
python3 python-learning/with_statement.py
```

[with_simple.py](python-learning/with_simple.py) 练习文件读取、缩进范围和自动关闭；[with_statement.py](python-learning/with_statement.py) 练习 `with` / `as`、同时读写文件、异常清理与 SQLite 事务。这两个练习独立于 Agent 课程，不计入 10 个 demo。
