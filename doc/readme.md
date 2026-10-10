# 从零学习 Agent 开发：DeepSeek + 手写 Python

面向第一次开发 Agent 的读者。先掌握 HTTP、消息、工具和循环，再理解工程设计。学习材料采用 **Markdown 文档 + 可运行的小实验 + OpenObserve 记录**，初期不用幻灯片，也不引入 Agent 框架。

## 先跑通这三步

```bash
cd /Users/wangying/apps/sakelei/ai/agent-learning
# 已有 .env，不要覆盖；初次在其他机器使用时才执行 cp .env.example .env。
uv sync --locked
uv run python verify_observe.py
uv run python -m demos.beginner.beginner_01_chat
```

第一条验证命令只产生一条 `observability.check`，不调用模型。第二条调用 DeepSeek，并打印 `trace_id`。打开 [本机 OpenObserve](http://localhost:5080/web/)，进入 Traces，选择 `default`，把时间范围设为最近 15 分钟，按这个 ID 找到记录。命令行也能确认入库：

```bash
uv run python verify_observe.py --trace-id 替换成实际32位trace_id --expect 2
```

普通问答应看到一个运行根 span 和一个模型 span。模型记录里包含脱敏后的请求、响应、耗时、token 用量及结束原因。**观测已接到底层，每个示例都经过同一个客户端。** 接入细节和排错见 [OpenObserve](openobserve.md)。

本机已完成三级真实模型实验、计算评估和入库核对，trace ID 与验证范围见 [验收记录](verification.md)。

没有 `uv` 时，可以使用 Python 3.11+：`python3 -m venv .venv`，然后 `.venv/bin/python -m pip install httpx python-dotenv opentelemetry-api opentelemetry-sdk opentelemetry-exporter-otlp-proto-http`；之后用 `.venv/bin/python` 替代 `uv run python`。项目原有虚拟环境可直接使用。

## 按初级、中级、高级学习

本套课程已提供 **3 个级别、10 个可运行 demo**，均围绕真实模型交互。每级文档列出前置知识、运行命令、阅读顺序、练习、观测点和通过标准。先做初级，不必一次读完整套材料。

| 级别 | 目标与内容 | demo 数 | 分级文档 |
|---|---|---:|---|
| 初级 | HTTP、消息历史、工具协议、手写循环 | 3 | [初级课程](levels/beginner.md) |
| 中级 | JSON、流式、关键词 RAG、SQLite 会话 | 4 | [中级课程](levels/intermediate.md) |
| 高级 | 思考模式、固定用例评估、只读资料 Agent | 3 | [高级课程](levels/advanced.md) |

### Demo 对照表

所有命令均在 `agent-learning/` 目录运行，例如 `uv run python -m demos.beginner.beginner_01_chat`。

| 编号 | Demo | 运行模式 | 核心验收 |
|---|---|---|---|
| B01 | `demos/beginner/beginner_01_chat.py` | 真实模型 | 能读懂请求、回答、用量和结束原因 |
| B02 | `demos/beginner/beginner_02_messages.py` | 真实模型 | 第二次请求实际携带第一轮历史 |
| B03 | `demos/beginner/beginner_03_agent_loop.py` | 真实模型 + 本地工具 | 工具结果与最终答案一致，循环有限 |
| I01 | `demos/intermediate/intermediate_01_json.py` | 真实模型 | JSON 解析后还要通过业务校验 |
| I02 | `demos/intermediate/intermediate_02_stream.py` | 真实模型 | 正确重组消息，仅一个模型 span |
| I04 | `demos/intermediate/intermediate_04_rag.py` | 本地检索 + 真实模型 | 引用存在；未命中时明确依据不足 |
| I05 | `demos/intermediate/intermediate_05_memory.py` | SQLite + 真实模型 | 重新加载历史、隔离会话、防止覆盖 |
| A01 | `demos/advanced/advanced_01_thinking.py` | 真实模型 | 最终结果验算，并比较资源开销 |
| A03 | `demos/advanced/advanced_03_evaluation.py` | 真实模型 + 本地工具 | 同时校验答案、工具证据和预算 |
| A04 | `demos/advanced/advanced_04_knowledge_agent.py` | 真实模型 + 只读检索工具 | 搜索、回填、引用校验完整可见 |

真实模型交互统一经底层客户端上报。假 HTTP 和内存观测仅供回归测试使用，不向 OpenObserve 发送测试噪音。删除示例后保留原有编号，便于对照既有记录。

### 按级验收

```bash
uv run python -m unittest discover -s tests -v
uv run python verify_live.py --level beginner
uv run python verify_live.py --level intermediate
uv run python verify_live.py --level advanced
```

不带 `--level` 默认验收全部三级，通常共 13 次模型请求。添加 `--evaluate` 会再执行四个真实计算用例，通常增加 8 次请求。只需学习时按级运行，无须每次全量调用。当前全部已验收，详见 [验收记录](verification.md) 与 [Review 记录](review.md)。

### 原理文档

分级文档负责带你做实验，以下文章负责解释原理：

0. [Python 代码与注释规范](python-style.md)
1. [一次模型调用](lessons/01-http.md)
2. [消息、上下文与 token](lessons/02-messages.md)
3. [工具与 Agent 循环](lessons/03-agent-loop.md)
4. [结构化输出](lessons/04-json.md)
5. [流式与思考模式](lessons/05-stream-thinking.md)
6. [失败、预算和安全](lessons/06-reliability.md)
7. [检索、记忆与状态](lessons/07-retrieval-memory.md)
8. [评估与交付](lessons/08-evaluation.md)

向量库、自动长期记忆、分布式任务队列与生产部署属于后续扩展，本次提供的是可验证的单机教学实现。

## 文件怎么分工

```text
agent-learning/
├── demos/
│   ├── beginner/               # B01–B03：HTTP、消息、手写 Agent 循环
│   ├── intermediate/           # I01、I02、I04、I05：JSON、流式、RAG、会话
│   └── advanced/               # A01、A03、A04：思考、评估、综合 Agent
├── toolbox.py                  # 工具说明、白名单、参数检查、实际函数
├── retrieval.py                # 检索与引用校验
├── session_store.py            # 会话隔离与版本控制
├── offline_lab.py              # 回归测试用假 HTTP 和内存观测
├── data/                       # 虚构资料与评估用例
├── llm.py                      # 直接 HTTP 请求；所有模型/工具交互的统一入口
├── telemetry.py                # span、脱敏、限长、批量导出
├── settings.py                 # .env 配置
├── verify_observe.py           # 仅检验观测，不消耗模型 token
├── verify_live.py              # 真实模型 + 入库验收，会消耗 token
└── tests/                      # 假 HTTP + 内存 exporter，完全离线
```

从 `agent-learning/` 目录运行 Agent demo，使用模块形式，例如 `uv run python -m demos.beginner.beginner_01_chat`。这样课程示例集中在 `demos/`，同时仍能直接导入同目录的底层客户端和工具实现。

Python 语法练习单独放在仓库根目录的 [`python-learning/`](../python-learning/)；它不计入 Agent 课程级别和 demo 数量。

```text
python-learning/
├── README.md
└── with_statement.py       # with、as、异常清理和 SQLite 事务
```

`httpx` 负责 HTTP；`python-dotenv` 读取本地配置；OpenTelemetry SDK 负责观测。它们都不会替你决定 Agent 下一步。项目不使用模型 SDK，也不使用 Agent 框架，循环仍是普通 Python。

## 配置

`agent-learning/.env` 已有本机配置。仓库只提供不含真实值的 [.env.example](../agent-learning/.env.example)，真实 `.env` 被 Git 忽略。环境变量优先于 `.env`，脚本从任何工作目录启动都读取 `agent-learning/.env`。

| 变量 | 用途 |
|---|---|
| `DEEPSEEK_API_KEY` | DeepSeek API 密钥 |
| `DEEPSEEK_BASE_URL` | 默认 `https://api.deepseek.com/v1`；代码追加 `/chat/completions` |
| `DEEPSEEK_MODEL` | 默认 `deepseek-flash` |
| `DEEPSEEK_TIMEOUT` | HTTP 读取等操作的超时秒数，默认 60；不是整次任务的硬截止时间 |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OpenObserve 组织地址，如 `http://localhost:5080/api/default` |
| `OTEL_EXPORTER_OTLP_HEADERS` | 鉴权，形如 `Authorization=Basic <凭据>` |
| `OTEL_SERVICE_NAME` | 默认 `agent-learning`；本机已有值 `chat-demo` 会继续生效 |
| `TELEMETRY_CAPTURE_CONTENT` | 默认 `true`，记录教学请求/响应正文；`false` 只记录元信息 |
| `TELEMETRY_CONTENT_MAX_CHARS` | 每份正文最多 32768 字符，超出明确标记截断 |
| `TELEMETRY_ENABLED` | 默认 `true`；关闭后不会上报，学习时保持开启 |

核对日期：2026-10-05。当前模型名与思考参数已查阅 [DeepSeek 官方入门页](https://api-docs.deepseek.com/)；模型名称、定价和支持能力会变化，以官方信息与真实请求结果为准，不在课程中固定价格。

旧框架材料保留在 `old-workspace/`，当前学习路径不依赖它。先从 [第一节](lessons/01-http.md) 开始。
