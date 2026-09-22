# 官方示例学习指南（OpenAI Agents SDK）

> **对象**：218 个官方示例（`../openai-agents-python/examples/`）
> **配套**：`agent-course.md`（6 周计划）。本文是它的"官方素材库"。
> **原则**：**不是从头看到尾**，而是按概念分层读。一个示例 = 一个概念，读完要能说出"它解决了什么问题"。

---

## 0. 三条铁律

**① 别按顺序读 218 个。**
它们是"能力清单"，不是"教程"。`make examples-run` 会全部跑一遍 —— 那既烧钱（几百次 API 调用）也会让你淹没在细节里。

**② 每个示例只读 3 样东西**：
- 顶部 docstring（作者写的意图）
- `Agent(...)` 的构造参数（**这是示例的核心**，看它用了哪个新字段）
- `main()` 里 `Runner.run` 前后那几行（通常就是新知识的落点）

剩下的样板代码不用细看。**Agent 定义就是"配置"**，看配置就知道它想演示什么。

**③ 示例是"最佳实践的参考"，不是"生产代码"。**
很多示例为了演示一个特性会故意简化（无错误处理、无护栏、内存存状态）。看的时候要问自己：**"这个写法放到生产会炸在哪？"** —— 这正是面试的追问方向。

---

## 1. 怎么跑

### 1.1 运行器

```bash
cd /Users/wangying/apps/sakelei/ai/openai-agents-python

uv run run_example.py --list                          # 看官方推荐入口
uv run run_example.py examples/basic/hello_world.py   # 跑单个示例
```

**已验证**：`hello_world.py` 在本机跑通，用的是你的 DeepSeek Key。

### 1.2 它是怎么让 DeepSeek 跑起官方示例的（重要，读一下这个文件）

📁 `run_example.py`（1 个文件，126 行，**自包含**）—— 它做的事就是"不改官方示例一行代码，先把 SDK 配好"：

```python
# run_example.py:71-76
def setup_sdk(cfg):
    client = AsyncOpenAI(base_url=cfg["OPENAI_BASE_URL"], api_key=cfg["OPENAI_API_KEY"])
    set_default_openai_client(client, use_for_tracing=False)
    set_default_openai_api("chat_completions")      # ← 关键：官方示例默认走 Responses API
    set_trace_processors([BatchTraceProcessor(ConsoleSpanExporter())])
```

再加两个小技巧：
- `os.environ.setdefault("OPENAI_DEFAULT_MODEL", cfg["MODEL"])`（`run_example.py:110`）→ 把示例里写死的 `gpt-4o` 之类顶掉
- `runpy.run_path(target, run_name="__main__")`（`run_example.py:120`）→ 以 `__main__` 身份执行，示例底部的 `asyncio.run(main())` 才会触发

> 🔑 **这 6 行代码值得记住**：它就是你在 `my-agent/src/myagent/runtime/llm.py` 里做的事。
> 也就是说 —— **你已经掌握了"让 SDK 接入任意厂商"的完整方法**，只是你之前不知道它就是全部。

⚠️ **踩坑点**：`run_example.py` 读的是 `openai-agents-python/config.env`（**独立的一份**，`run_example.py:27`），跟 `my-agent/config.env` 不是同一个文件。改了这边那边不生效。

### 1.3 ⚠️ 兼容边界：哪些示例在 DeepSeek 上跑不了（实测结论）

**核心规则**：凡是走 OpenAI **Responses API 托管工具**（Hosted Tools）的示例，在 DeepSeek + Chat Completions 上**必然失败**。

实测证据（跑 `examples/tools/web_search.py`）：
```
agents.exceptions.UserError: Hosted tools are not supported with the ChatCompletions API.
Got tool type: <class 'agents.tool.WebSearchTool'>
```
抛出位置：📁 `src/agents/models/chatcmpl_converter.py:1034`

| 示例 / 目录 | 能跑吗 | 原因 |
|---|---|---|
| `basic/hello_world.py`、`tools.py`、`usage_tracking.py`、`stream_text.py` 等 | ✅ | 纯 function calling，提供商无关 |
| `agent_patterns/` 全部 | ✅ | 本地工具 + 本地护栏逻辑 |
| `handoffs/`、`memory/`、`mcp/` | ✅ | 本地实现 |
| `tools/web_search.py`、`file_search.py`、`code_interpreter.py`、`computer_use.py`、`image_generator.py` | ❌ | **托管工具**，需要 Responses API |
| `hosted_mcp/` 全部 | ❌ | 托管 MCP，同上 |
| `reasoning_content/` | ❌ | 依赖 gpt-5 / gpt-oss 的 reasoning 字段 |
| `realtime/`、`voice/` | ❌ | 需要 OpenAI Realtime / 音频 API |
| `sandbox/` | ⚠️ | 需要 Docker 或云沙箱后端（`sandbox/basic.py` 可试） |
| `model_providers/litellm_auto.py`、`any_llm_auto.py` | ❌ | 需要额外装 `litellm` / `any-llm` |

**所以你的精读范围是那 60% 左右的 ✅ 部分 —— 而这部分恰好覆盖了面试的全部核心考点。**

> 💡 **面试加分点**：能说清"托管工具 vs 本地工具"的区别，说明你理解 SDK 的抽象分层。
> 托管工具 = 模型厂商替你把工具跑在它那边（省事、但锁厂商、且你无法审计）；
> 本地工具 = 你的进程执行（可控、可护栏、跨厂商）。**这是选型时必须讲清的权衡。**

### 1.4 交互式示例怎么自动跑

很多示例用 `input()` 提问，会卡住。解决办法：

```bash
EXAMPLES_INTERACTIVE_MODE=auto uv run run_example.py examples/agent_patterns/input_guardrails.py
```

原理：📁 `examples/auto_mode.py` 提供了三个 helper，示例统一调用它们：
```python
is_auto_mode()                                    # 读 EXAMPLES_INTERACTIVE_MODE
input_with_fallback(prompt, fallback)             # auto 模式返回 fallback
confirm_with_fallback(prompt, default)            # auto 模式返回 default
```
**这是"让示例可自动化测试"的标准做法** —— 把交互点收敛成可注入的函数。值得学，你写 CLI 工具时用得上。

---

## 2. 阅读顺序（对齐你的 6 周计划）

| 周 | 主题 | 官方示例 | 目标 |
|---|---|---|---|
| **1** | 打破黑盒 | `basic/hello_world.py` → `basic/tools.py` → `basic/usage_tracking.py` → `basic/stream_text.py` | 对照 `raw_loop.py`，确认"框架 = 你手写的那个循环 + 边界处理" |
| **2** | 工具设计 | `basic/tools.py` → `tools/web_search.py`（只读）/ `tools/apply_patch.py` / `tools/shell.py` / `tools/tool_search.py` | 看清工具的四种形态：本地 / 托管 / 需审批 / 海量延迟加载 |
| **3** | 上下文与成本 | `basic/previous_response_id.py` → `memory/sqlite_session_example.py` → `memory/redis_session_example.py` → `memory/compaction_session_example.py` | 手算账单 + 掌握三种会话方案 |
| **4** | 结构化输出 + 多 Agent | `agent_patterns/deterministic.py` → `agent_patterns/routing.py` → `agent_patterns/agents_as_tools.py` → `handoffs/message_filter.py` | 讲清 workflow vs agent、handoff vs agents-as-tools |
| **5** | 安全与护栏 | `agent_patterns/input_guardrails.py` → `agent_patterns/output_guardrails.py` → `basic/tool_guardrails.py` → `agent_patterns/streaming_guardrails.py` | 分清三种护栏；说清 tripwire |
| **6** | 工程化 | `agent_patterns/human_in_the_loop.py` → `mcp/filesystem_example/` → `customer_service/main.py` → `research_bot/` → `financial_research_agent/` | HITL + MCP + 完整多 Agent 项目 |

**第 7-8 周（面试准备）再精读**：`financial_research_agent/manager.py`（生产级多 Agent 编排）。

---

## 3. 逐目录详解

### 3.1 `basic/` — 单点能力，最小示范（24 个文件）

**定位**：每个文件只演示**一个** SDK 特性。**最该先读的目录。**

| 文件 | 教什么 | 关键 API |
|---|---|---|
| `hello_world.py` | 最小形态：20 行，无工具 | `Agent(name=, instructions=)`、`Runner.run` |
| `tools.py` | 加工具 | `@tool`、返回 Pydantic 对象 |
| `usage_tracking.py` | 读 token 账单 | `result.context_wrapper.usage`、`usage.request_usage_entries` |
| `stream_text.py` | 流式输出 | `Runner.run_streamed`、`result.stream_events()` |
| `stream_items.py` / `stream_function_call_args.py` | 流式的高级事件类型 | `RunItemStreamEvent`、参数增量 |
| `dynamic_system_prompt.py` | 提示词用函数动态生成 | `instructions=` 传 callable |
| `prompt_template.py` | 提示词模板化 | — |
| `previous_response_id.py` | 让服务端存历史（省 token） | `previous_response_id=` |
| `non_strict_output_type.py` | 结构化输出的宽松模式 | `output_type` + `AgentOutputSchema(..., strict_json_schema=False)` |
| `tool_guardrails.py` | **工具级**护栏（第三种护栏） | `@tool_input_guardrail`、`@tool_output_guardrail` |
| `trace_redaction.py` | trace 脱敏（**生产必看**） | `Trace(..., )` + 脱敏回调 |
| `lifecycle_example.py` / `agent_lifecycle_example.py` | 钩子：agent/run 生命周期 | `AgentHooks`、`RunHooks` |
| `retry.py` | 模型调用重试 | `ModelSettings` + retry 配置 |
| `local_file.py` / `local_image.py` / `remote_pdf.py` | 多模态输入 | `input=` 里放文件/图片 |
| `image_tool_output.py` | 工具返回图片 | 工具返回非文本 |
| `hello_world_gpt_5.py` / `hello_world_gpt_oss.py` | 特定模型的写法 | ❌ 本机跑不了 |
| `stream_ws.py` | WebSocket 流式 | 进阶 |

**先读这 4 个**：`hello_world` → `tools` → `usage_tracking` → `stream_text`。

---

### 3.2 `agent_patterns/` — 架构模式（⭐ 最重要）

**定位**：这个目录**直接对应 Anthropic《Building Effective Agents》里那套模式分类**。它的 `README.md` 就是一份现成的教学大纲，**建议先读 README 再读代码**。

📁 `examples/agent_patterns/README.md`

| 文件 | 模式 | 面试价值 |
|---|---|---|
| `deterministic.py` | **Prompt Chaining**：A→check→B，代码控制流程 | ⭐⭐⭐ |
| `routing.py` | **Routing**：triage agent 按语言分流 | ⭐⭐⭐ |
| `agents_as_tools.py` | **Agents as Tools**：主 Agent 把子 Agent 当工具调 | ⭐⭐⭐ |
| `agents_as_tools_streaming.py` | 同上 + 嵌套事件流 | ⭐⭐ |
| `agents_as_tools_structured.py` | 同上 + 结构化输入 | ⭐⭐ |
| `agents_as_tools_conditional.py` | 同上 + 动态决定用哪些工具 | ⭐⭐ |
| `parallelization.py` | **Parallelization**：并行跑多次，投票选最好 | ⭐⭐⭐ |
| `llm_as_a_judge.py` | **Evaluator-Optimizer**：生成 → 评判 → 修订 | ⭐⭐⭐ |
| `input_guardrails.py` | 输入护栏 + tripwire | ⭐⭐⭐ |
| `output_guardrails.py` | 输出护栏 | ⭐⭐⭐ |
| `streaming_guardrails.py` | 流式场景下的护栏 | ⭐⭐ |
| `human_in_the_loop.py` | 工具审批 + 中断恢复 | ⭐⭐⭐ |
| `human_in_the_loop_stream.py` / `_server.py` / `_custom_rejection.py` | HITL 的流式/服务端/自定义拒绝 | ⭐⭐ |
| `forcing_tool_use.py` | 强制模型必须调某个工具 | ⭐⭐ |
| `hosted_multi_agent_beta.py` | 托管多 Agent（实验特性） | ⭐ |

**读完这个目录，你就掌握了"Agent 架构设计"的全部标准答案。**

---

### 3.3 `handoffs/` — Agent 之间怎么传话

| 文件 | 教什么 | 关键 API |
|---|---|---|
| `message_filter.py` | 交接时**裁剪历史**（只给下一个 Agent 看该看的） | `handoff(agent, input_filter=...)` |
| `message_filter_streaming.py` | 同上 + 流式 | — |

> 为什么要 `input_filter`？因为 handoff 默认会把**全部历史**传过去，包括无关的工具输出。
> **这既是成本问题（token）也是安全问题（信息泄露给不该看的 Agent）** —— 面试可以主动提。

---

### 3.4 `memory/` — 会话记忆（15 个文件）

**定位**：讲"怎么让 Agent 记住上一轮说了什么"。从最简单到生产级都有。

| 文件 | 教什么 | 关键 API | 面试 |
|---|---|---|---|
| `sqlite_session_example.py` | **最小会话**：一次 run 后自动带上下文 | `SQLiteSession(session_id)`、`Runner.run(..., session=)` | ⭐⭐ |
| `redis_session_example.py` | **Redis 存会话**（你有本地 Redis 6379，重点跑） | `RedisSession.from_url(...)`、`ping()`、`ttl`、`key_prefix` | ⭐⭐⭐ |
| `compaction_session_example.py` | **长对话自动压缩**省 token | `OpenAIResponsesCompactionSession`、`run_compaction()` | ⭐⭐⭐ |
| `advanced_sqlite_session_example.py` | 会话当数据库：用量统计 + **对话分支** | `store_run_usage`、`create_branch_from_turn`、`switch_to_branch` | ⭐⭐ |
| `encrypted_session_example.py` | 加密存储 | `EncryptedSession` | ⭐ |
| `mongodb_session_example.py` / `dapr_session_example.py` / `sqlalchemy_session_example.py` | 其它后端 | 对应 Session 类 | ⭐ |
| `openai_session_example.py` | 服务端存储 | `OpenAIConversationsSession` | ⭐⭐ |
| `file_session.py` / `hitl_session_scenario.py` / `file_hitl_example.py` | 会话 + HITL 组合场景 | — | ⭐⭐ |

**Redis 的实际用法**：
```python
session = RedisSession.from_url(session_id, url="redis://localhost:6379/0")
if not await session.ping(): ...                       # 连通性自检
result = await Runner.run(agent, "问题", session=session)   # 自动带历史
# session_id 即键名；ttl 设过期；key_prefix 做多租户隔离
```
> `key_prefix` 做多租户隔离 —— **这是你面试可以讲的、把后端经验迁移到 Agent 的实例**。

**compaction vs advanced_sqlite 的区别（别搞混）**：
- **compaction** 解决**太贵/太长** → 把历史压成一条摘要项
- **advanced_sqlite** 解决**可分析/可编辑** → 用量统计 + 从任意一轮分叉出新对话

---

### 3.5 `tools/` — 工具的四种形态（16 个文件）

**相比 `basic/tools.py` 的进阶点**：`basic` 只教你包一个本地函数；这里覆盖**四类不同性质的工具**。

| 文件 | 工具的形态 | 关键 API | 面试 |
|---|---|---|---|
| `web_search.py` | **托管工具**（厂商替你跑） | `WebSearchTool()` | ⭐⭐ ❌跑不了 |
| `file_search.py` | 托管 RAG | `FileSearchTool(vector_store_ids=)` | ⭐⭐⭐ ❌ |
| `code_interpreter.py` | 托管代码沙箱 | `CodeInterpreterTool()` | ⭐⭐ ❌ |
| `apply_patch.py` | **危险工具：每次改动先审批** | `ApplyPatchTool`、`apply_diff` | ⭐⭐⭐ |
| `shell.py` | **危险工具：跑命令 + 逐条审批 + 环境变量白名单** | `ShellTool(needs_approval=, on_approval=)` | ⭐⭐⭐ |
| `shell_human_in_the_loop.py` | 同上 + 完整 HITL | — | ⭐⭐⭐ |
| `tool_search.py` | **海量工具：延迟加载 + 命名空间** | `ToolSearchTool()`、`tool_namespace`、`@tool(defer_loading=True)` | ⭐⭐⭐ |
| `programmatic_tool_calling.py` | 模型**写程序**来并发编排工具 | `ProgrammaticToolCallingTool()` | ⭐⭐ |
| `computer_use.py` | 浏览器操作 | `ComputerTool` + Playwright | ⭐ ❌ |
| `image_generator.py` | 生成图片 | — | ⭐ ❌ |
| `codex.py` / `codex_same_thread.py` | 把 Codex 当工具 | — | ⭐ |
| `skills/`（`SKILL.md` 等） | 把"技能"组织成文件 | — | ⭐⭐ |

**最值得读**：`apply_patch.py`、`shell.py`、`tool_search.py`。
前两个教你**危险工具怎么设计**（审批 + 白名单 + 拒绝），第三个教你**工具规模化**（工具多了不能全塞 prompt —— 用延迟加载 + 语义检索）。

> ⚠️ 这里有个重要判断：**"工具越多越好"是错的。**
> 我们第 1 周实测过：一个工具的 JSON Schema 约 280 token，每轮固定重发。
> `tool_search.py` 演示的就是解法：**只把工具名+简介放 prompt，要用时再检索出完整 schema。**

---

### 3.6 `mcp/` — 接外部工具服务器（17 个文件）

**定位**：MCP = Model Context Protocol。一个标准协议，让 Agent **不用写代码**就能插上现成的工具服务器。

**⭐ MCP 入门首选** → `mcp/filesystem_example/`
理由：一行 `MCPServerStdio` + `npx` 就能起服务，自带样本文件，开箱即跑。

```python
# mcp/filesystem_example/main.py 的核心
server = MCPServerStdio(params={"command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", ...]})
agent = Agent(name=..., mcp_servers=[server])
```

| 文件 | 教什么 | 关键 API | 面试 |
|---|---|---|---|
| `filesystem_example/` | ⭐ MCP 入门（stdio） | `MCPServerStdio` | ⭐⭐⭐ |
| `git_example/` | 更小，接 Git MCP + 缓存工具列表 | `cache_tools_list=True` | ⭐⭐ |
| `streamablehttp_example/` | **生产常用传输：远程 HTTP** | `MCPServerStreamableHttp` | ⭐⭐⭐ |
| `tool_filter_example/` | **只暴露部分工具 + 逐次审批** | `create_static_tool_filter`、`require_approval="always"` | ⭐⭐⭐ |
| `get_all_mcp_tools_example/` | 预取工具并转严格 schema | `MCPUtil.get_all_function_tools` | ⭐⭐ |
| `manager_example/` | 用 `MCPServerManager` 管生命周期 + FastAPI | `MCPServerManager` | ⭐⭐ |
| `sse_example/`、`sse_remote_example/`、`streamable_http_remote_example/` | 其它传输方式 | — | ⭐ |
| `prompt_server/` | MCP 里放 prompt（不只放工具） | — | ⭐⭐ |

**面试要点**：stdio（本地子进程）vs Streamable HTTP（远程服务）—— 生产上多用 HTTP，因为**多实例部署时不能每个 pod 都起一个子进程**。

---

### 3.7 `model_providers/` — 换厂商 / 私有化部署

| 文件 | 教什么 | 关键 API |
|---|---|---|
| `custom_example_global.py` | **全局换成一个 OpenAI 兼容端点** | `set_default_openai_client` + `set_default_openai_api("chat_completions")` |
| `custom_example_agent.py` | 只给某个 Agent 换模型 | `Agent(model=自定义 Model)` |
| `custom_example_provider.py` | 自定义 `ModelProvider` | 进阶 |
| `litellm_auto.py` / `litellm_provider.py` | 用 LiteLLM 路由到任意厂商 | `model="litellm/..."` ❌需装包 |
| `any_llm_auto.py` / `any_llm_provider.py` | 同上，any-llm | `model="any-llm/..."` ❌ |

> 💡 `custom_example_global.py` **就是 `run_example.py` 和你的 `runtime/llm.py` 的官方版本**。三处对照看一遍，这部分知识就闭环了。

---

### 3.8 `reasoning_content/` — 读模型的"思考过程"

| 文件 | 教什么 | 关键 API |
|---|---|---|
| `runner_example.py` | 用 Runner 取 reasoning 摘要（流式+非流式） | `ModelSettings(reasoning=Reasoning(effort="high", summary="auto"))`、`ReasoningItem` |
| `main.py` | 直接调底层 Model 接口取 reasoning | 想看 SDK 内部时读 |

⚠️ 需要 gpt-5 / gpt-oss，本机跑不了。但**概念要懂**：reasoning 模型会额外产出"思考摘要"，可以展示给用户（提升信任）或用于调试。**面试问"怎么让 Agent 更可解释"时可以答这个。**

---

### 3.9 `hosted_mcp/` — OpenAI 侧托管的 MCP（5 个文件）

跟 `mcp/` **相反**：MCP 服务器不用你本地跑，由 OpenAI 代你连。

| 文件 | 教什么 | 关键 API |
|---|---|---|
| `simple.py` | 托管 MCP 最简 | `HostedMCPTool(tool_config={"type": "mcp", "server_url": ...})` |
| `human_in_the_loop.py` | 托管 MCP 也能逐次审批 | `require_approval="always"`、`state.approve/reject` |
| `connectors.py` | 官方 connector（如 Google Calendar）+ OAuth | `tool_config={"connector_id": ..., "authorization": ...}` |
| `on_approval.py` | 审批回调 | — |

均 ❌ 跑不了（需要 Responses API），但**对比 `mcp/` 读一遍很有价值**：托管 vs 自托管是选型题。

---

### 3.10 两个完整多 Agent 项目（⭐ 面试素材）

这是两个"不像玩具"的示例，**值得精读**。

#### `research_bot/`（3 Agent，线性流水线，好读）

```
planner_agent (输出结构化 WebSearchPlan)
    ↓
search_agent × N  （asyncio.create_task 并行）
    ↓
writer_agent (输出结构化 ReportData)
```
关键：`research_bot/manager.py` 里的并行搜索（`asyncio.gather` / `as_completed`）+ `trace()` + `gen_trace_id()`。
**没有验证环节**，代码短，**先读这个**。

#### `financial_research_agent/`（6 Agent，带验证-修订闭环，工程化）

比上面多出四个**生产级要素**：
1. **子分析师暴露成工具**：`financials_agent.as_tool(custom_output_extractor=...)`
2. **验证 Agent**：`verifier_agent` 审计证据（核对引用 URL、时间截止）
3. **修订循环**：验证不通过 → `REVISION_PROMPT` 重写一轮
4. **预算控制**：`MAX_SEARCHES` 限制检索次数

**这就是"Evaluator-Optimizer"模式在真实项目里的样子。**

> 🎯 **建议读法**：`research_bot/manager.py` → `financial_research_agent/manager.py`，两个文件对照读，差异一眼看清。

---

### 3.11 `customer_service/main.py` — 单文件多 Agent 客服

**定位**：triage / FAQ / 订座 三个 Agent + handoff + **强类型共享上下文**。

关键 API：
- `handoff(agent=..., on_handoff=..., tool_name_override=...)`
- `RunContextWrapper[AirlineAgentContext]` —— 工具里 `context.context.seat_number = ...`（**跨 Agent 共享状态**）
- `result.last_agent`、`result.to_input_list()`、`ItemHelpers.text_message_output`
- `MessageOutputItem` / `HandoffOutputItem` / `ToolCallItem` 事件打印

**面试价值 ⭐⭐⭐**：这是回答"多 Agent 怎么协作"的标准素材 —— **handoff + 共享 typed context + 循环驱动多轮**。
⚠️ 注意它用内存 `input_items` 存历史，**生产必须换成持久 Session**（这正是它作为面试题的追问点）。

---

### 3.12 `sandbox/` — 隔离执行环境（71 个文件，进阶）

**先搞清概念区别（面试常考）**：

| | 参数级护栏 | 沙箱（sandbox） |
|---|---|---|
| 拦什么 | **单次工具调用的参数**（路径、SQL、命令） | **整个执行环境**（进程/文件系统/网络边界） |
| 在哪实现 | 你的工具函数里（`my-agent/tools/guards.py`） | 容器 / 云沙箱 |
| 强度 | 逻辑校验，有绕过风险 | OS 级隔离 |
| 关系 | **互补**，都要有 | |

**最有教学价值的文件**：

| 文件 | 教什么 | 关键 API |
|---|---|---|
| `sandbox/basic.py` | 沙箱入门：声明工作区 → 起 Agent → 流式 | `Manifest(entries={...})`、`SandboxAgent(capabilities=[Shell()])`、`SandboxRunConfig` |
| `sandbox/sandbox_agent_with_tools.py` | 沙箱能力 + 宿主自定义工具混用 | `SandboxAgent(capabilities=..., tools=[...])` |
| `sandbox/sandbox_agents_as_tools.py` | 沙箱 Agent 变成另一个 Agent 的工具 | `.as_tool()` |
| `sandbox/tutorials/dataroom_qa/main.py` | 检索优先：在挂载的财报语料上作答 + **引用溯源** | `Manifest(entries={"data": LocalDir(...)})` |
| `sandbox/tutorials/repo_code_review/main.py` | **编码 Agent**：挂 git 仓库 → 读代码 → 跑测试 → 出 review | `GitRepo(...)` + 结构化输出 |

**云后端**在 `sandbox/extensions/`（E2B / Modal / Daytona / Vercel / Cloudflare / Runloop / Blaxel）。

⚠️ README 明确警告：Unix-local 后端在 Linux 上**没有 OS 级隔离**，macOS 只有文件系统限制。**不受信命令必须用 Docker 或云后端。**

> 💡 这是"让 Agent 自主执行代码"能不能上生产的**分水岭**。

---

### 3.13 `realtime/` 与 `voice/`（简要）

- `realtime/`：实时语音对话。`realtime/app/agent.py`（`RealtimeAgent` + `realtime_handoff`）、`cli/demo.py`、`app/server.py`（WebSocket）、`twilio/`（电话接入）
- `voice/`：STT → Agent → TTS 流水线。`static/main.py`（静态）、`streamed/main.py` + `my_workflow.py`（流式）

**面试价值中等**：属于加分项（如果你面的是语音/客服方向则重要）。均 ❌ 本机跑不了。

---

## 4. 精读笔记（我已逐行读过的，含代码要点）

### `basic/tools.py`（40 行）
```python
class Weather(BaseModel):            # 工具返回结构化对象
    city: str = Field(description="The city name")
    ...

@tool                                # ← 注意：新写法
def get_weather(city: Annotated[str, "The city to get the weather for"]) -> Weather:
    """Get the current weather information for a specified city."""
    return Weather(city=city, temperature_range="14-20C", conditions="Sunny with wind.")
```
**三个要点**：
1. `@tool` 和 `@function_tool` **是同一个东西** —— 已核实：📁 `src/agents/decorators.py:10` 写着 `tool = function_tool`（只是别名）
2. 参数描述用 `Annotated[str, "..."]` 写，或 docstring 的 `Args:` 写 —— **两者都会被塞进 JSON Schema 给模型看**
3. **工具可以返回 Pydantic 对象**（不是必须返回字符串），SDK 会序列化

### `agent_patterns/deterministic.py`（84 行）—— Workflow 模式
```python
class OutlineCheckerOutput(BaseModel):     # ① 结构化输出
    good_quality: bool
    is_scifi: bool

outline_checker_agent = Agent(..., output_type=OutlineCheckerOutput)

with trace("Deterministic story flow"):    # ② 整个 workflow 一条 trace
    outline = await Runner.run(story_outline_agent, input_prompt)
    checked = await Runner.run(outline_checker_agent, outline.final_output)   # ③ 手动传参
    assert isinstance(checked.final_output, OutlineCheckerOutput)             # ④ 类型收窄
    if not checked.final_output.good_quality: exit(0)                         # ⑤ 代码做门禁
    story = await Runner.run(story_agent, outline.final_output)
```
**这就是 Workflow 的教科书实现**：**流程由代码控制，LLM 只是每个格子里的执行者**。
面试时"Agent vs Workflow"题的现成答案。

### `agent_patterns/routing.py`（77 行）—— Handoff 路由
```python
triage_agent = Agent(
    instructions="Handoff to the appropriate agent based on the language of the request.",
    handoffs=[french_agent, spanish_agent, english_agent],     # ① 注册交接目标
)
with trace("Routing example", group_id=conversation_id):        # ② group_id 串多轮会话
    result = Runner.run_streamed(agent, input=inputs)
    async for event in result.stream_events(): ...              # ③ 流式事件过滤
agent = result.current_agent                                    # ④ ★ 交接后当前是谁
```
**`result.current_agent`** —— 这个属性就是 handoff 的可观测出口。**面试问"怎么知道交接发生了"，答它。**

### `agent_patterns/agents_as_tools.py`（83 行）—— 关键差异
```python
spanish_agent = Agent(..., handoff_description="An english to spanish translator")

orchestrator_agent = Agent(
    instructions="You never translate on your own, you always use the provided tools.",  # ★
    tools=[
        spanish_agent.as_tool(
            tool_name="translate_to_spanish",
            tool_description="Translate the user's message to Spanish",
        ),
        ...
    ],
)
# 跑完后再用 synthesizer_agent 汇总
synthesizer_result = await Runner.run(synthesizer_agent, orchestrator_result.to_input_list())
```
**和 routing.py 对照读，差异一目了然**：

| | `routing.py`（handoff） | `agents_as_tools.py` |
|---|---|---|
| 控制权 | **转移**给子 Agent | **留在**编排 Agent |
| 子 Agent 能看到历史吗 | 能 | 不能（只看到传进去的参数） |
| 能并行调多个吗 | 不能 | 能（`translate to French AND Spanish`） |
| 返回值 | 子 Agent 直接对用户说话 | 回到编排 Agent 手里再加工 |

**判断口诀**：要"接管对话"用 handoff；要"借个能力"用 agents-as-tools。

### `agent_patterns/input_guardrails.py`（122 行）—— 护栏
```python
@input_guardrail
async def math_guardrail(context, agent, input) -> GuardrailFunctionOutput:
    result = await Runner.run(guardrail_agent, input, context=context.context)   # ★ 用另一个 Agent 做检查
    final_output = result.final_output_as(MathHomeworkOutput)
    return GuardrailFunctionOutput(
        output_info=final_output,
        tripwire_triggered=final_output.is_math_homework,     # ★ 触发即中止整个 run
    )

agent = Agent(..., input_guardrails=[math_guardrail])         # 挂在 Agent 上

try:
    result = await Runner.run(agent, input_data)
except InputGuardrailTripwireTriggered:                        # ★ 捕获并给兜底回复
    ...
```
**三个要点**：
1. **护栏本身可以是一个 Agent**（`guardrail_agent`）—— 因为它输出结构化 bool，所以可靠
2. `tripwire_triggered=True` → 抛 `InputGuardrailTripwireTriggered` → **整个 run 中止**
3. **input guardrail 只在第一轮、且只在 starting_agent 上跑**（已核实源码：📁 `run_internal/run_loop.py:1213`）—— 这是性能取舍，也是新手最常误解的点

### `agent_patterns/human_in_the_loop.py`（146 行）—— 中断恢复
```python
async def _needs_temperature_approval(_ctx, params, _call_id) -> bool:
    return "Oakland" in params.get("city", "")        # ① 动态判断：只有 Oakland 需要审批

@tool(needs_approval=_needs_temperature_approval)
async def get_temperature(city: str) -> str: ...

result = await Runner.run(agent, "What is the weather and temperature in Oakland?")
while result.interruptions:                            # ② 中断循环
    state = result.to_state()                          # ③ 序列化（可跨进程/跨机器）
    json.dump(state.to_json(), f)
    state = await RunState.from_json(agent, stored)    # ④ 反序列化
    for it in result.interruptions:
        state.approve(it)   # 或 state.reject(it)      # ⑤ 人工决策
    result = await Runner.run(agent, state)            # ⑥ 从断点恢复
```
⚠️ 文件顶部有关键安全警告：
> *"The saved file must remain under trusted application control. Do not replace it with a snapshot supplied by a browser or another untrusted client."*

**含义**：`RunState` 是**可信状态**，反序列化一个用户传来的 state = 把你的工具执行权交给攻击者。**这是面试问"HITL 的安全陷阱"的标准答案。**

---

## 5. 与 `my-agent` 项目的对照

| 官方示例的写法 | 你的项目 | 说明 |
|---|---|---|
| `@tool`（`agents.decorators`） | `@function_tool`（`agents`） | **同一函数**，`tool = function_tool` |
| 单文件写完 agent + tools + main | 分 `definitions/` / `tools/` / `services/` / `runtime/` | 示例为了可读性不分层；**你的分层更适合长大** |
| 工具直接读模块级变量 | `ctx.context.root` 注入 | 你的写法支持多租户，示例不行 |
| 示例基本无参数校验 | `safe_resolve` / `reject_sensitive` | **示例是教学，不是生产** |
| 内存存 `input_items` | 待接 Session | 你的 Stage 6 要补上 |
| `set_default_openai_client` 写在示例里 | 收敛到 `runtime/llm.py` | 你做得更好 |

> 💡 **面试时可以这么讲**："官方示例为了演示单点特性都是平铺的单文件；我的项目按 `runtime→definitions→tools→services` 单向分层，内层不依赖 SDK，所以能脱离 LLM 做毫秒级单测。"
> 这句话展示的是**工程判断力**，比"我会用 SDK"值钱得多。

---

## 6. 示例 → 面试考点 映射

| 面试题 | 读哪个示例 |
|---|---|
| Agent 和普通 LLM 调用差在哪 | `basic/hello_world.py` vs 手写的 `raw_loop.py` |
| Function Calling 原理 | `basic/tools.py` + `raw_loop.py --step 2` |
| 工具越多越好吗 | `tools/tool_search.py`（延迟加载） |
| Agent vs Workflow | `agent_patterns/deterministic.py` |
| Routing 怎么做 | `agent_patterns/routing.py` |
| Handoff vs Agents-as-tools | `routing.py` **对照** `agents_as_tools.py` |
| Handoff 怎么实现 | `handoffs/__init__.py:214`（`transfer_to_*` 工具）+ `result.current_agent` |
| 护栏怎么设计 | `input_guardrails.py` / `output_guardrails.py` / `basic/tool_guardrails.py` |
| 上下文怎么管、成本怎么控 | `memory/compaction_session_example.py` + `basic/usage_tracking.py` |
| 会话状态存哪 | `memory/redis_session_example.py` / `sqlite_*` / `openai_*` 三种对比 |
| 危险操作怎么办 | `agent_patterns/human_in_the_loop.py` + `tools/shell.py` |
| 提示词注入怎么防 | `input_guardrails.py` + HITL 的 RunState 警告 |
| MCP 是什么 | `mcp/filesystem_example/` + 对比 `hosted_mcp/simple.py` |
| 多 Agent 系统怎么搭 | `research_bot/` → `financial_research_agent/` |
| 怎么评估 Agent | `financial_research_agent/`（verifier + 修订循环） |
| 换模型厂商 | `model_providers/custom_example_global.py` |
| 让 Agent 执行代码 | `sandbox/basic.py` + README 的隔离警告 |

---

## 7. 快速索引（按你的优先级排序）

**第一梯队（必读，全都能跑）**
```
basic/hello_world.py
basic/tools.py
basic/usage_tracking.py
basic/stream_text.py
agent_patterns/deterministic.py
agent_patterns/routing.py
agent_patterns/agents_as_tools.py
agent_patterns/input_guardrails.py
agent_patterns/output_guardrails.py
agent_patterns/human_in_the_loop.py
agent_patterns/README.md          ← 先读这个
```

**第二梯队（工程化）**
```
memory/sqlite_session_example.py
memory/redis_session_example.py
memory/compaction_session_example.py
mcp/filesystem_example/
handoffs/message_filter.py
basic/tool_guardrails.py
basic/previous_response_id.py
tools/shell.py
tools/apply_patch.py
tools/tool_search.py
```

**第三梯队（完整项目 / 进阶）**
```
customer_service/main.py
research_bot/（manager.py 是核心）
financial_research_agent/（manager.py 是核心）
sandbox/basic.py
sandbox/tutorials/repo_code_review/
mcp/streamablehttp_example/
```

**只读不跑（本机跑不了，但概念要懂）**
```
tools/web_search.py、file_search.py、code_interpreter.py、computer_use.py
hosted_mcp/*
reasoning_content/*
realtime/*、voice/*
```

---

## 8. 一句话总结

官方示例的价值不在于"教你写代码"，而在于**它把 Agent 工程的所有标准模式都摆出来了**：

- **架构模式** → `agent_patterns/`（对应 Anthropic 那套分类）
- **工具形态** → `tools/`（本地 / 托管 / 需审批 / 规模化）
- **状态管理** → `memory/`（内存 / SQLite / Redis / 服务端 / 压缩）
- **安全边界** → 三种护栏 + HITL + sandbox
- **完整系统** → `research_bot/` 和 `financial_research_agent/`

**你的任务不是"看完"，而是"看完后能对着自己的 `my-agent` 说出：这个特性我该加在哪一层、为什么这么加"。**
