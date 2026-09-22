# Agent 开发实战课程

> **面向**：Go 后端工程师，Python 小白，目标是通过面试 + 能上手做真东西
> **实训场**：`my-agent/`（脚手架已搭好，本课程只往里填肉）
> **参考源**：`../openai-agents-python/`（框架源码 + 218 个官方示例，只读不改）

---

## 0. 怎么用这份文档

三个用途，按需跳转：

| 你在干什么 | 去哪 |
|---|---|
| 明天要面试，突击 | 第 3 章（考点）、第 5 章（题库） |
| 想系统学，动手做 | 第 4 章（实训路线），从 Stage 0 顺序走 |
| 想查某个概念是什么 | 第 2 章（术语对照）、第 3 章（模块详解） |

**标记约定**：
- 【面试】= 高频考点，会给出答题骨架
- 【实战】= 动手任务，有可运行产物
- 【Go 类比】= 拿你已有的知识挂靠，减少学习成本
- 📁 = 本机可验证的源码位置（结论都核对过，可自行复查）

---

## 1. 先破除神秘感

### 1.1 一句话定义

**Agent = LLM + 循环 + 工具 + 状态。没有别的。**

市面上 90% 的"Agent 框架"都在包装同一个东西：一个 for 循环。用 Go 写出来就是这样：

```go
func Run(agent Agent, input []Message, maxTurns int) (string, error) {
    msgs := input
    for turn := 0; turn < maxTurns; turn++ {
        // ① Reason：把「指令 + 全部历史 + 工具清单」发给模型
        resp := callLLM(agent.Model, agent.Instructions, msgs, agent.Tools)

        // ② 收敛判断：模型不再要工具，就结束
        if len(resp.ToolCalls) == 0 {
            return resp.Content, nil
        }

        // ③ Act：模型只是"说"要调什么，真正执行的是你的代码
        msgs = append(msgs, resp.Message)
        for _, tc := range resp.ToolCalls {
            out := executeTool(tc, agent.Ctx)   // ← 这就是你的 handler
            msgs = append(msgs, ToolMessage{ID: tc.ID, Content: out})
        }
        // ④ Observe：结果回到 msgs，下一轮模型就能看到 → 回到 ①
    }
    return "", ErrMaxTurnsExceeded   // 熔断
}
```

真实实现：📁 `openai-agents-python/src/agents/run.py`（主循环入口）+ `run_internal/`（真正干活的地方，18.9k 行，全是流式 / 护栏 / 交接 / 重试 / 断点恢复的边界情况）。

### 1.2 三条必须刻进脑子的认知

**① 模型不做任何事，它只会"说"。**
所有副作用（读文件、查库、下单、发消息）都是**你的代码**在执行。这直接推出安全设计的全部原则：模型是不可信客户端。

**② 工具的返回值会拼进 prompt 发给远端模型。**
所以「模型能看到什么」= 你的安全边界 = 你的成本边界。读到 `config.env` 就等于把 Key 连同 trace 一起传出去了。

**③ 每一轮循环都要重发全部历史。**
第 k 轮的 prompt token ≈ 前 k-1 轮之和。对话 n 轮的总 prompt 成本是 **O(n²)**。这不是 bug，是 Attention 的物理特性。📁 你项目里 `runtime/cli.py` 的 `print_usage()` 就是专门把这个账单打出来给你看的。

### 1.3 【面试】Agent vs Workflow —— 第一道分水岭题

这是现在面试最爱问的第一题，答错直接掉档次。

| | Workflow（工作流） | Agent（智能体） |
|---|---|---|
| 流程由谁定 | **你**（代码里写死） | **模型**（运行时自己决定） |
| 可预测性 | 高，可画出完整流程图 | 低，每次路径可能不同 |
| 可测试性 | 高，每步独立单测 | 低，需要轨迹（trace）评估 |
| 成本 | 可控（步数固定） | 不可控（可能空转 20 轮） |
| 适合 | 步骤明确、有 SLA 要求 | 步骤无法穷举、需要探索 |

**Workflow 的四种标准形态**（面试常追问"那 workflow 具体怎么做"）：
1. **Prompt Chaining**：前一步输出喂后一步，串行
2. **Routing**：先分类，再路由到专门的 prompt / 模型
3. **Parallelization**：扇出多个 LLM 并行，或投票取多数
4. **Orchestrator-Workers**：一个"工头"动态拆任务分给"工人"

**标准答题骨架**（背下这段）：
> "我会**先用最简方案**：能一次调用解决就一次调用；流程固定就用 workflow 编排，因为可测、可控、可估成本。只有当**路径无法预先穷举**、需要根据中间结果动态决定下一步时，才升级成 Agent。生产上常见的做法是**外层 workflow、内层局部 agent**——用 workflow 保证 SLA 和可观测性，把不确定性圈在一个可控的格子里。直接上全自主 Agent 通常意味着成本和稳定性失控。"

### 1.4 【面试】什么时候不该用 Agent

反向题，答得好非常加分：
- 步骤固定 → 用 workflow，Agent 只会带来不确定性
- 对延迟敏感（<1s）→ 一次 LLM 调用都嫌慢，别提多轮
- 要 100% 确定性（对账、计费）→ 用规则代码，LLM 不能用在这
- 上下文里没有信息可探索 → 工具再多也变不出来

---

## 2. 术语对照表（Go 后端 → Agent）

把新概念挂到你已有的知识树上，是学得最快的方式。

| Agent 概念 | Go 后端里对应什么 | 差异点（面试可能追问） |
|---|---|---|
| `Agent` 定义 | 一份 service 配置 / 装配 | 它是**数据**不是行为，可运行时拼装 |
| `tools` | handler 注册表 | 注册给模型选，不是给路由选 |
| 工具函数签名 → JSON Schema | Protobuf IDL / OpenAPI | Schema 是**给模型的文档**，写不好模型就选错 |
| `@function_tool` 装饰器 | `rpc.RegisterHandler` 之类的注册宏 | Python 装饰器等价于 Go 的代码生成 + 注册 |
| `RunContextWrapper[TContext]` | `context.Context` | 强类型泛型，且能读到 usage / 当前轮次 |
| `Session` | Redis / MySQL 存会话 | 存的是 messages 列表 |
| `Runner.run()` | 一个 HTTP 请求的处理入口 | 内部是 for 循环，一次调用可能打多次下游 |
| `guardrails` | middleware / 拦截器 | 检查对象是**自然语言**，不是 header |
| `handoff` | 服务间转发 | 但转发决策是模型做的 |
| `tracing` | OpenTelemetry | SDK 内置，开箱有 span 树 |
| `max_turns` | timeout / 重试上限 | 这是防死循环的熔断器 |
| `output_type` | Protobuf 出参 / 强类型响应 | 模型直接产出结构化对象 |

**一句话**：你已有的工程直觉（分层、幂等、超时、可观测、限流）在 Agent 领域**一字不差地适用**，只是多了"模型"这个不确定组件。这也是你的优势。

---

## 3. 七大核心模块（考点详解）

### M1 工具调用（Function Calling）

**原理链路**：
```
Python 函数签名 + docstring
  → SDK 解析成 JSON Schema              📁 function_schema.py
  → 随请求发给模型
  → 模型返回 tool_call {id, name, arguments(JSON)}
  → SDK 反序列化参数、调用你的函数
  → 返回值转成字符串，作为 tool message 回填
```

**你项目里的示范**：📁 `my-agent/src/myagent/tools/repo.py`

**关键设计（可以直接当面试案例讲）**：

1. **docstring 就是给模型看的 API 文档**，要写清"什么时候用 / 什么时候别用"。参数描述走 Google 风格的 `Args:`，SDK 自动转 JSON Schema。
   ```python
   """列出某个目录下的直接子项，用来快速了解项目结构。

   当你想知道"这个项目有哪些文件"时用它。
   想读某个文件的具体内容请改用 read_file；想按关键字找东西请改用 search_content。
   """
   ```
   注意"想读文件请改用 read_file"这句 —— **工具选择的准确率，一半靠这句话**。

2. **`ctx` 参数不进 Schema**。`RunContextWrapper[FsContext]` 由 SDK 注入，模型看不到它。
   📁 你有单测守着这点：`tests/test_repo_tools.py::test_context_param_is_injected_not_exposed`
   > 面试价值：说明你分清了"工具参数"（模型决定）和"运行时依赖"（你注入）——这是新手最常混的一件事。

3. **参数零信任**。模型是客户端，参数必须校验。📁 `guards.py` 的 `safe_resolve`：拒绝绝对路径 + 拒绝 `../` 越界。

4. **返回要裁剪**。工具返回会进 prompt，返回一个 5MB 文件 = 烧钱 + 冲爆上下文。📁 `repo.py` 的 `limit` 上限 500 行、`MAX_SEARCH_HITS=30`。

**工具粒度（面试高频）**：

| 反例 | 问题 | 正例 |
|---|---|---|
| `run_shell(cmd)` | 万能工具，提示词注入的天堂，`rm -rf /` 一句话搞定 | `list_dir` / `read_file` / `search_content` |
| `get_everything()` | 参数爆炸，模型选不明白 | 按语义拆成几个窄工具 |
| 20 个细碎工具 | 模型选择困难，准确率下降 | 按业务域归组，>8 个就拆文件 |

**判断标准**：一个工具应该对应**一个语义清晰的动作**，且它的**权限边界你要能一句话说清**。

📁 参考：`docs/zh/tools.md`、`examples/basic/tools.py`

---

### M2 上下文与状态（Context / Session）

这是**最容易在生产翻车**的地方，也是面试区分度最高的地方。

**先分清三个"上下文"**（经常被混为一谈）：

| 名字 | 是什么 | 在你项目里的位置 |
|---|---|---|
| **运行上下文** `RunContextWrapper[TContext]` | 注入给工具的外部依赖（根目录、DB 连接、用户身份） | `FsContext(root=Path)` 📁 `guards.py:25` |
| **对话历史** messages | 模型看到的全部消息（user / assistant / tool） | `cli.py` 的 `history` 列表 |
| **模型上下文窗口** context window | 模型一次能吃多少 token（硬上限） | 由模型决定，如 64K / 128K |

**会话状态管理的三种方案**：

| 方案 | 怎么做 | 优点 | 代价 |
|---|---|---|---|
| 手搓 | 自己存 list，每次 `input=history + [新消息]` | 完全可控，零依赖 | 自己处理裁剪、并发、持久化 |
| `Session` | 传给 `Runner.run(session=...)`，SDK 自动读写 | 开箱即用，换存储只换一个类 | 抽象多一层 |
| `previous_response_id` | 让 OpenAI 服务端存历史，只传 ID | 省 token（不重发历史） | 绑定厂商，跨轮状态在别人手里 |

📁 SDK 提供的 Session 实现（**Redis 那个你可以直接说**）：`SQLiteSession` / `AsyncSQLiteSession` / `AdvancedSQLiteSession` / `RedisSession` / `SQLAlchemySession` / `MongoDBSession` / `DaprSession` / `EncryptedSession`
位置：`openai-agents-python/src/agents/extensions/memory/`

**你项目当前的实现**：手搓方案。📁 `cli.py:55-56`
```python
history.clear()
history.extend(result.to_input_list())
```
`to_input_list()` 把"模型原始输出 + 工具结果 + 用户消息"完整还原成可以再次作为 input 的格式。这里 `clear` + `extend` 而不是 `history = ...`，是为了让闭包外持有同一个列表引用 —— 细节但有味道。

**上下文膨胀的治理手段（面试能答出 3 条就很好）**：
1. **裁剪**：只保留最近 N 轮（滑动窗口）
2. **摘要**：老对话让模型压缩成一段摘要（compaction）
3. **外部化**：大内容落到外部存储，上下文里只放引用/ID（RAG 的本质）
4. **工具返回裁剪**：从源头控制（📁 `repo.py` 的 `limit` / `MAX_SEARCH_HITS`）
5. **开新会话**：最土但最有效（📁 你 readme 里写的"交互模式注意用 exit 重开"）
6. **Prompt Caching**：前缀不变的部分命中缓存，省钱不提准确率

> **面试加分句**："上下文管理是 Agent 的成本中心。我通常从工具返回值裁剪和滑动窗口做起，这两招零成本；摘要和 RAG 是第二步；`previous_response_id` 这类服务端方案能省 token 但会绑定厂商，需要权衡。"

📁 参考：`docs/zh/context.md`、`docs/zh/sessions.md`、`examples/memory/`

---

### M3 结构化输出（Structured Output / `output_type`）

**问题**：默认 `final_output` 是字符串。要接下游系统就得写正则去抠 —— 脆弱且愚蠢。

**解法**：给 `Agent` 传 `output_type=某个类型`，模型直接产出结构化对象。

```python
from pydantic import BaseModel

class RepoReport(BaseModel):
    layers: list[str]            # 项目分层
    file_count: int              # 文件数
    risky_files: list[str]       # 可疑文件
    summary: str                 # 一句话总结

agent = Agent[FsContext](
    name="仓库分析助手",
    instructions=...,
    model=model,
    tools=list(REPO_TOOLS),
    output_type=RepoReport,      # ← 加这一行
)
# 用的时候：result.final_output 是 RepoReport 实例，不是 str
```

**原理**：SDK 把 Pydantic 模型转成 JSON Schema（📁 `agent_output.py`、`strict_schema.py`），要求模型按 schema 输出，然后校验反序列化。**校验失败会重试**。

【Go 类比】这就是把 `string` 出参换成 `*pb.RepoReport`。你在 Go 里绝不会用正则解析 RPC 响应，Agent 里也一样。

**什么时候必须用**：
- 输出要喂给下游代码（而不是给人看）
- 输出要入库
- 需要程序化断言（测试、评估）

**坑**：
- 模型可能给不出合法 JSON → 字段设计要简单，别嵌套五层
- `strict` 模式下部分类型受限（如 `Optional` 的语义、`dict` 的 key 必须是 string）
- 强制结构化会**略微降低回答自由度**，探索类任务别硬套

📁 参考：`docs/zh/agents.md`、`examples/basic/` 里的 output 相关示例

---

### M4 多 Agent 与 Handoff

**核心洞见（这句话能镇住面试官）**：
> **Handoff 本质上就是一个工具调用。**

📁 证据：`openai-agents-python/src/agents/handoffs/__init__.py:214`
```python
@classmethod
def default_tool_name(cls, agent: AgentBase[Any]) -> str:
    return _transforms.transform_string_function_style(
        f"transfer_to_{agent.name}", ...)
```

每个 handoff 都会注册一个叫 `transfer_to_<agent名>` 的工具。模型"调用"它 → 运行时的 `current_agent` 换人 → 下一轮用新 Agent 的 instructions 和 tools 继续跑。**没有额外的一次 LLM 调用，也不是什么黑魔法。**

**什么时候用 handoff，什么时候加工具（最常做错的判断）**：

| 变化的是 | 用什么 | 例子 |
|---|---|---|
| **能力**（能做什么） | 加**工具** | 给现有 Agent 加"查订单"能力 |
| **职责 / 人设 / 提示词**（是谁、怎么想） | 用 **handoff** | 从"仓库分析"切到"代码审查"，关注点完全不同 |

**判断口诀**：如果你发现需要在一个提示词里写"当你做 A 时按规则甲，做 B 时按规则乙，两者冲突时……" —— 那就该拆成两个 Agent 了。提示词里的"分情况讨论"就是拆分的信号。

**多 Agent 的替代方案（面试可以主动提，显示知识面）**：
- **Handoff**：控制权转移，一个时刻只有一个 Agent 活跃
- **Agents as Tools**（Agent 工具化）：主 Agent 把子 Agent 当工具调用，**控制权不转移**，主 Agent 汇总结果
- 区别：要"接管对话"用 handoff；要"问一句拿答案"用 agents-as-tools

**坑**：
- handoff 会丢上下文（可用 `input_filter` 裁剪传给下一个 Agent 的历史）
- 两个 `definitions/*.py` 互相 import = 循环依赖，改用 handoff 或抽公共层
- 交接链路长了，调试难度指数上升 —— **trace 是唯一可行的调试手段**

📁 参考：`docs/zh/handoffs.md`、`docs/zh/multi_agent.md`、`examples/handoffs/message_filter.py`、`examples/agent_patterns/`

---

### M5 护栏（Guardrails）—— 三层，别搞混

**⚠️ 这是面试最容易露怯的地方。Agent 圈说的"护栏"至少有三种，层级完全不同。**

| 层 | 是什么 | 在哪拦 | 你项目里的位置 |
|---|---|---|---|
| **① 工具参数护栏** | 校验模型传进来的参数（路径越界、SQL 注入、命令注入） | 工具函数内部 | 📁 `guards.py: safe_resolve` / `reject_sensitive` |
| **② 模型 I/O 护栏** | 检查**输入 prompt** 和**最终输出**的安全性 | 模型边界 | SDK 的 `@input_guardrail` / `@output_guardrail` |
| **③ 工具级 guardrail** | 针对某次工具调用做检查（比①更结构化） | 工具调用前后 | SDK 的 `tool_input_guardrails` |

**② 的用法**：
```python
from agents import input_guardrail, output_guardrail, GuardrailFunctionOutput, RunContextWrapper

@input_guardrail
async def block_credential_theft(ctx: RunContextWrapper[FsContext], agent, input) -> GuardrailFunctionOutput:
    text = str(input)
    hit = "config.env" in text or "api key" in text.lower()
    return GuardrailFunctionOutput(output_info={"hit": hit}, tripwire_triggered=hit)
```
`tripwire_triggered=True` 会抛 `InputGuardrailTripwireTriggered`，整个 run 中止。

**⚠️ 关键细节（说出来很加分）**：
📁 `run_internal/run_loop.py:1213-1217`
```python
all_input_guardrails = (
    starting_agent.input_guardrails + (run_config.input_guardrails or [])
    if current_turn == 0 and not is_resumed_state
    else []
)
```
**input guardrail 只在 `current_turn == 0` 且用 `starting_agent` 跑。** 后续轮次不再对全量历史跑一遍 —— 这是精心设计的性能取舍，也是新手最容易误解的地方（"我加在子 Agent 上的输入护栏怎么没生效？"）。

output guardrail 则是**每次产生最终输出时**都跑。

**护栏设计的实战原则**：
1. **确定性规则放代码里，别指望模型自觉**。路径校验、正则、白名单 —— 这些用代码做 100% 可靠，用 prompt 做 0% 可靠。
2. **护栏要拦在"信息能流出去"之前**。模型读到的东西 = 会进 prompt + 落 trace。
3. **护栏失败要给出可执行的纠错信息**，因为这条信息会回给模型：
   ```python
   raise ValueError(f"文件太大（{human_size(size)}），超过上限；请改用 search_content 定位关键内容")
   #                ↑ 模型读到这句会自己换工具         ↑ 明确下一步动作
   ```
   这是你项目里做得非常漂亮的一点，面试可以直接讲。
4. **注意：输入护栏检查的是**用户输入，不是**模型输出**。别拿它防"模型泄露"。

📁 参考：`docs/zh/guardrails.md`、`examples/basic/output_guardrails.py`

---

### M6 可观测性 Tracing 与成本

**为什么 Agent 的可观测性比普通后端更难**：非确定性。同一个输入，两次运行的路径可能不同 —— 传统"看日志找 bug"不够，需要看**完整轨迹（trace）**。

**SDK 内置 tracing**，span 类型（📁 `tracing/span_data.py`）：

| Span | 含义 |
|---|---|
| `AgentSpanData` | 一次 Agent 运行 |
| `GenerationSpanData` / `ResponseSpanData` | 一次模型调用（含 prompt/输出/token） |
| `FunctionSpanData` | 一次工具调用 |
| `HandoffSpanData` | 一次 Agent 交接 |
| `GuardrailSpanData` | 一次护栏检查 |
| `MCPListToolsSpanData` | MCP 工具列举 |
| `CustomSpanData` | 自定义 |

**接法**（📁 你项目的 `runtime/llm.py:30-33`）：
```python
set_trace_processors([BatchTraceProcessor(ConsoleSpanExporter())])  # 打到控制台
set_trace_processors([])                                            # 关掉
```

**生产上要做的（面试常问"你怎么保证 Agent 系统不出事"）**：
- 换成 OTLP 导出到观测平台（📁 你本机已有 OpenObserve + Langfuse，见 `doc/readme.md`）
- 按 OTel GenAI 语义约定打属性：`gen_ai.request.model` / `gen_ai.usage.input_tokens` / `gen_ai.tool.name`
- 记录：每轮 token、工具耗时、失败率、**熔断次数**、最终是否收敛
- **脱敏**：prompt 里可能有用户 PII，落库前要洗

**成本控制的六个抓手**：
1. `max_turns` 熔断（📁 你项目默认 12，SDK 默认是 10 —— 📁 `run_config.py:45`）
2. 工具返回裁剪
3. 上下文裁剪 / 摘要
4. Prompt Caching
5. 小模型做路由/分类，大模型只做难任务
6. 输出长度限制

> **面试加分句**："Agent 的 cost 是 O(n²) 的，因为每轮重发历史。所以我会在三个位置埋点：工具返回值的字节数、每轮 prompt token、总轮数。这三个指标任意一个异常增长，就说明链路有问题。"

📁 参考：`docs/zh/tracing.md`、`docs/zh/usage.md`

---

### M7 执行控制：并发、HITL、沙箱

**并发**（Go 工程师的主场）：
- SDK 支持并行工具调用（模型一次返回多个 tool_call）
- `@function_tool` 支持 `run_in_parallel` 之类的开关
- 输入护栏也支持并行/串行（📁 `run_loop.py` 里 `sequential_guardrails` / `parallel_guardrails`）
- ⚠️ **陷阱**：工具并行执行 = 你的 services 层必须**并发安全**。读写同一份状态要有锁或幂等设计。

**HITL（Human-in-the-Loop，人工确认）**：
危险操作（删数据、发消息、下单）不能让模型自己按。
📁 SDK 的 `RunState`（`run_state.py`）就是这个：
- 跑到需要审批的工具时**中断**，`state.get_interruptions()` 拿到待审批项
- 把 state 序列化落库，等人点"同意"
- `Runner.run(agent, state)` 从断点**恢复**继续跑
- 有 schema version（当前 1.18）保证跨版本兼容

【Go 类比】这就是**分布式事务里的"悬挂/补偿"**问题：中断点必须可序列化、可恢复、且恢复时不能重放副作用（📁 源码注释里专门讲了 pending session write 的 reconciliation）。

> 这个类比说出来，面试官会觉得你是真懂系统，而不只是会调 SDK。

**沙箱**：
- SDK 的 `sandbox` 概念专指**隔离容器**（让 Agent 在里面跑长任务），跟你 `guards.py` 的"参数级护栏"不是一回事 📁（你源码注释里专门澄清过这个命名冲突，是个好习惯）
- 📁 SDK 支持：Docker / Unix local / E2B / Modal / Daytona / Vercel / Cloudflare / Runloop / Blaxel
- 生产上让 Agent 执行代码/命令，**必须**在容器里，且要限 CPU/内存/网络/文件系统

📁 参考：`docs/zh/human_in_the_loop.md`、`docs/zh/sandbox_agents.md`、`docs/zh/sandbox/`

---

## 4. 实训路线（Stage 0 → 8）

**总原则**：
- **一次只动一个变量**，否则出问题分不清是谁的错
- **每关结束跑 `uv run pytest -q`**，并给自己新写的逻辑补单测
- 每关结束 commit 一次（便于回看）
- 卡住了先跑一遍再问我，别先问

---

### Stage 0 · 看清主循环（30 分钟，不写代码）

**做**：`uv run myagent`（交互模式），问「这个项目分几层？护栏在哪个文件？」，**不要加 `--no-trace`**。

**验收**：能回答
- 这一问模型调了几次工具？分别是什么？
- 每次请求的 `prompt` token 是多少？为什么第 3 次比第 1 次大很多？
- `read_file` 被调用时，模型传的 `path` 是什么？（对照 trace 里的 arguments）

**新概念**：Reason-Act-Observe 循环、context 膨胀、trace。

**要悟到的**：`readme.md` 附录那张图不是比喻，就是 trace 里那串 span。

📁 `docs/zh/running_agents.md`

---

### Stage 1 · 加你的第一个工具

**做**：新建 `tools/git_info.py`，实现 `git_log(ctx, limit=10)`，用 `subprocess` 调 `git log --oneline`；加进 `REPO_TOOLS`。

**验收**：
- `uv run myagent -q "最近的提交都是什么"` 能答对
- `uv run pytest -q` 全绿
- 【关键】新工具的 docstring 里写清了"什么时候用/什么时候别用"

**新概念**：`@function_tool`、docstring → JSON Schema、工具粒度设计。

**思考题**：`list_dir` 为什么拒绝绝对路径？如果模型想分析 `/tmp/x` 怎么办？
（答案：用 `--root` 换工作目录，而不是拆护栏。**能力边界靠架构，不靠提示词。**）

📁 `docs/zh/tools.md`、`examples/basic/tools.py`

---

### Stage 2 · 把逻辑抽到 services/

**做**：把 Stage 1 的 git 调用搬到 `services/git_service.py`，`tools/git_info.py` 只剩「校验参数 → 调 service → 裁剪返回」；给 service 写单测（用 `tmp_path` 造一个假 git 仓库）。

**验收**：`tools/git_info.py` 里看不到 `subprocess`；service 单测不需要建 Agent 就能跑。

**要悟到的**：你 readme 里"工具保持薄壳"这句话，在**实测**下到底省了什么。

**新概念**：依赖方向、纯函数、pytest fixture。

**这是面试可讲的工程亮点**：内层完全不知道 SDK 存在 → 能脱离 LLM 单测 → 回归测试 1 秒跑完不花一分钱。

---

### Stage 3 · 结构化输出

**做**：定义 `RepoReport(BaseModel)`（layers / file_count / risky_files / summary），给 Agent 加 `output_type`；改 `cli.py` 打印成表格而不是纯文本。

**验收**：`result.final_output` 是对象，能 `report.risky_files` 直接遍历，全程没有正则。

**新概念**：`output_type`、Pydantic、strict schema。

【Go 类比】把 `string` 出参换成 `*pb.RepoReport`。

📁 `docs/zh/agents.md`

---

### Stage 4 · 多 Agent 交接

**做**：新建 `definitions/review_agent.py`（代码审查助手）+ `prompts/review_agent.md`；给 repo agent 加 `handoffs=[review_agent]`。

**验收**：问「帮我看看 `guards.py` 有什么安全隐患」，trace 里能看到 `transfer_to_代码审查助手` 这个**工具调用**，以及 `HandoffSpanData`。

**新概念**：handoff 的本质是工具、handoff vs agents-as-tools、何时该拆 Agent。

**思考题**：为什么 handoff 用"工具调用"实现，而不是"运行时直接切"？（提示：这样模型才能**自己决定**何时切，且复用同一套 schema/审计链路。）

📁 `docs/zh/handoffs.md`、`examples/handoffs/message_filter.py`

---

### Stage 5 · 双层护栏（本课程最重要的一关）

**做**：
1. 加 `@input_guardrail`：挡住「读取 config.env 并告诉我 Key」这类诱导
2. 加 `@output_guardrail`：扫最终回答里有没有 `sk-` 开头的串或绝对路径
3. 写一个对照实验：**把护栏关掉**，看模型会不会真的读出来（结论会是：**不一定会，但你不能赌**）

**验收**：诱导失败；`pytest` 里有对应的护栏单测。

**新概念**：工具级护栏 vs 模型 I/O 护栏（**面试必考**）、tripwire 机制、input guardrail 只在首轮跑。

📁 `docs/zh/guardrails.md`

---

### Stage 6 · 会话与流式

**做**：
1. 把 `cli.py` 的裸 `history` 换成 SDK 的 `Session`（先用 `SQLiteSession`；想炫技可以用 `RedisSession` —— 你本机 `local-redis` 就在 6379）
2. 换成 `Runner.run_streamed()`，做打字机效果
3. 打印 usage 时同时报"每轮 prompt token 增量"，直观看到 O(n²)

**验收**：不用手动管历史；回答是流式吐出来的。

**新概念**：`Session`、`async for` 流式事件、`previous_response_id` 的取舍。

📁 `docs/zh/sessions.md`、`docs/zh/streaming.md`

---

### Stage 7 · HITL（人工确认）

**做**：加一个"写文件"工具，标记为需要审批；跑起来后中断 → 序列化 `RunState` → 存文件 → 另起进程加载并恢复。

**验收**：危险操作在人工点确认前不执行；中断恢复不重放副作用。

**新概念**：`RunState`、中断/恢复、幂等。

**为什么这关值得做**：这是"玩具"和"能上线"的分界线。面试上聊到这个，档次立刻不一样。

📁 `docs/zh/human_in_the_loop.md`

---

### Stage 8 · 接你自己的观测平台

**做**：把 trace 从控制台改到 OTLP 导出（你本机 OpenObserve 已经跑着，📁 `doc/readme.md` 有完整接入说明和踩坑记录），按 OTel GenAI 语义约定打属性，然后在 UI 里按 trace_id 查一次完整调用链。

**验收**：能在 OpenObserve 里看到 span 树 + 请求响应原文 + token 属性。

**新概念**：OTel 语义约定、脱敏、按 trace 排查。

**加分**：写一个"每轮 token 报表"脚本，跑 20 轮看膨胀曲线。

---

## 5. 面试题库

### 5.1 基础题（必答）

**Q1. Agent 和普通 LLM 调用有什么区别？**
> Agent 多了三样东西：**工具的调用能力**（模型可以要求执行副作用）、**循环**（根据执行结果决定下一步）、**状态**（跨轮次累积上下文）。本质是一个受模型驱动的 while 循环。普通 LLM 调用是单次无状态转换。

**Q2. Function Calling 的原理？**
> 把函数签名 + docstring 转成 JSON Schema 随请求发给模型；模型返回结构化的 tool_call（id/name/arguments）；本地执行后把结果作为 tool message 回填，再发一轮。**模型从头到尾没有执行任何代码**，它只是"说"要调什么。

**Q3. 为什么工具的参数必须校验？**
> 模型是**不可信客户端**。参数由模型生成，可能被提示词注入操纵，可能幻觉出越界路径。所以工具层必须做参数校验和权限收敛，不能指望 prompt 约束。我项目里 `safe_resolve` 拒绝绝对路径和 `../` 越界，`reject_sensitive` 拦截凭据文件。

**Q4. 工具选择不准怎么优化？**
> 三板斧：① 把"什么时候用/什么时候别用"写进 docstring（工具描述是给模型看的文档）；② 减少工具数量、按语义归组，太多了就拆 Agent；③ 参数设计要窄、要有默认值降低调用负担。

---

### 5.2 进阶题（区分度在这）

**Q5. Handoff 是怎么实现的？**
> 本质上是一个工具调用。SDK 为每个目标 Agent 注册一个 `transfer_to_<name>` 的工具（源码 `handoffs/__init__.py:214`），模型"调用"它就切换 current_agent，下一轮用新 Agent 的 instructions 和 tools。**没有额外的 LLM 调用**。这也解释了为什么 handoff 天然可被模型自主决策、也可被审计。

**Q6. 什么时候用 handoff，什么时候用 agents-as-tools？**
> 需要**转移控制权**（后续对话由新角色接管）用 handoff；只需要**借个能力问一嘴**（主 Agent 汇总结果）用 agents-as-tools。判断信号：如果提示词里开始出现"当你做 A 时…做 B 时…"，就该拆成 handoff 了。

**Q7. Agent 的对齐/护栏你怎么做？**
> 三层：**工具参数护栏**（路径/SQL/命令白名单，确定性代码，100% 可靠）、**模型 I/O 护栏**（input/output guardrail，检查自然语言）、**工具级 guardrail**。原则是**确定性规则放代码，不放 prompt**；护栏要拦在信息流出之前；护栏的报错信息要能指导模型自我纠正。⚠️ 补充细节：input guardrail 只在第一轮和 starting_agent 上跑，这是性能取舍。

**Q8. Agent 上下文怎么管理？成本怎么控？**
> 先说清三个"上下文"的区别（运行上下文 / 消息历史 / 模型窗口）。成本是 O(n²) 的，因为每轮重发历史。治理手段按成本排序：工具返回裁剪（零成本）→ 滑动窗口 → 摘要压缩 → 外部化（RAG，只放引用）→ `previous_response_id`（省 token 但绑定厂商）。埋点看三个指标：工具返回字节数、每轮 prompt token、总轮数。

**Q9. 生产上怎么防止 Agent 空转烧钱？**
> ① `max_turns` 熔断；② 单次 run 的 token 预算硬上限；③ 看 trace 的收敛率指标；④ 给工具加重试上限和超时；⑤ 关键路径用 workflow 编排而不是全自主 Agent。

**Q10. Agent 系统怎么做测试？**
> 分两层：**确定性部分**（工具函数、护栏、services）用传统单测，毫秒级、不花钱 —— 靠的是分层，让内层不依赖 SDK。**非确定性部分**（模型行为）做**轨迹评估**：构造一批用例，断言"是否调用了正确的工具""是否在 N 轮内收敛""结构化输出是否合法"，跑多次看通过率。不要期望断言最终文案的字面值。

**Q11. 危险操作怎么处理（Human-in-the-Loop）？**
> 用可中断/可恢复的运行状态：跑到需要审批的工具时中断，把 RunState 序列化落库，人工确认后从断点恢复。工程难点在**恢复时不能重放已有副作用**——本质是个分布式事务的悬挂/补偿问题，需要幂等设计。SDK 的 `RunState` 就是这个抽象（当前 schema 1.18）。

**Q12. 提示词注入（Prompt Injection）怎么防？**
> 关键是认清**信任边界**：工具返回值会进 prompt，所以**外部内容（网页、文件、用户上传）是不可信输入**。手段：① 权限最小化（Agent 拿不到危险工具）；② 工具返回值裁剪与净化，别把整篇外部文本塞进去；③ 输出护栏扫描；④ 危险操作强制人工确认；⑤ 结构化输出约束模型只能"填表"。**不存在 100% 防御**，所以架构上要假设它会被攻破。

---

### 5.3 架构设计题（大厂常出）

**Q13. 设计一个企业知识库问答 Agent。**
> 要考虑的点：**RAG 检索质量**（切块策略 / 混合检索 / 重排）、**引用与可溯源性**（回答必须带出处，否则不可信）、**权限过滤**（检索阶段就要按用户权限过滤，不能事后过滤 —— 否则模型已经看到了）、**多轮改写**（把追问改写成独立 query）、**"答不出"的处理**（宁可说不知道，别幻觉）、**成本**（小模型改写 query，大模型只做最终生成）、**可观测**（检索命中率、引用准确率、无答案率）。

**Q14. 一个 Agent 服务怎么部署？并发怎么处理？**
> 服务本身是无状态的（状态在 Session 存储里），可以水平扩。注意：① 一次 `Runner.run` 可能持续几十秒且含多次下游调用，要考虑连接池和超时；② 工具并行执行时 services 层必须并发安全；③ 长任务要考虑异步化（提交 → 轮询/回调）避免 HTTP 超时；④ 按租户/用户做限流，因为 token 成本高；⑤ 上下文和 trace 里的 PII 要脱敏。

**Q15. 模型换了 / 提示词改了，怎么保证不退化？**
> 建 **Eval 集**：一批有标注的输入 + 判定标准（工具调用正确性 / 结构化输出合法性 / 关键事实命中）。改提示词或换模型前后跑同一套 Eval 对比通过率。这就是 A/B 测试在 LLM 场景的落地。提示词要**版本化管理**（你项目把提示词抽到 `prompts/` 单独文件，就是为了这个）。

---

### 5.4 反问与软性问题

- 「你们现在 Agent 的**评估**体系是怎么建的？」（问这个显得专业）
- 「线上 Agent 的**成本**和**收敛率**有关注指标吗？」
- 「我的经验是，Agent 的不确定性要用**外围的工程约束**去框住 —— 想了解你们这块的取舍？」

---

## 6. 生产化清单（你的主场，面试可以主动展开）

Go 后端的工程直觉在 Agent 领域**完全适用**。以下是你的优势区：

| 议题 | Agent 场景下的具体化 |
|---|---|
| **超时** | 模型调用超时（几十秒）≠ 工具超时（毫秒）；要分别设 |
| **重试** | 模型调用可重试（幂等），**有副作用的工具不可盲目重试** |
| **幂等** | 中断恢复、重试都不能重放副作用 —— 需要业务侧幂等键 |
| **限流** | 按租户限 token/请求数；模型厂商也有配额 |
| **并发** | 工具并行执行 → services 必须并发安全 |
| **降级** | 模型不可用时怎么办？降级到规则 / 返回兜底话术 |
| **配置管理** | 📁 你项目已有：全项目只有 `runtime/config.py` 读配置 |
| **灰度** | 提示词 / 模型按流量比例灰度，看 Eval 指标 |
| **可观测** | trace + metrics + 日志三件套，按 trace_id 串起来 |
| **安全** | 凭据不进 prompt / 不进 trace；工具权限最小化 |
| **成本** | 按租户/功能维度分摊，做预算和告警 |

---

## 7. 学习节奏与纪律

### 7.1 六周计划（面试在 1-2 个月后，时间富余）

按"先懂原理、再会实现"的顺序。**每周一个主题，主题内的 Stage 可以分几次做**。

| 周 | 主题 | 内容 | 本周结束时你应该能…… |
|---|---|---|---|
| **第 1 周** | **打破黑盒** | 手写最小 Agent（`experiments/raw_loop.py`）→ 对照 Stage 0 的 trace | 白板画出 Reason-Act-Observe，并说清 tool_call 的数据结构 |
| **第 2 周** | **工具设计** | Stage 1 + Stage 2 | 独立加一个工具；说清 docstring → JSON Schema 的映射；认同"工具是薄壳" |
| **第 3 周** | **上下文与成本** | Stage 3 前置：做裁剪实验、滑动窗口、画 token 膨胀曲线 | 手算一次 O(n²) 账单；列出 5 种上下文治理手段及其代价 |
| **第 4 周** | **两个"升级"** | Stage 3（结构化输出）+ Stage 4（多 Agent 交接） | 讲清 `output_type` 的原理；讲清 handoff 为什么是工具调用 |
| **第 5 周** | **安全与护栏** | Stage 5（双层护栏）+ Stage 6（会话/流式） | 分清三种护栏的层级和触发时机；说出 prompt injection 的防御思路 |
| **第 6 周** | **工程化** | Stage 7（HITL）+ Stage 8（OTel 观测） | 讲清"可中断可恢复"的工程难点；说清 Agent 系统的可观测性方案 |
| **第 7-8 周** | **面试准备** | 整理项目讲述稿、建 Eval 集、刷第 5 章题库 | 把 `my-agent` 讲成一个"有取舍的技术故事" |

**第 1 周最关键**：它决定了后面 5 周你是"在学框架 API"还是"在设计 Agent 系统"。区别很大。

### 7.2 三条纪律

1. **一次只改一个变量**（改工具就别同时改提示词，出问题才分得清是谁的锅）
2. **每关跑 `uv run pytest -q`**，新逻辑必须补单测。你用 Go 写过测试，这习惯移植过来就是降维打击
3. **每关 commit 一次**

### 7.3 卡住时的排查顺序

先自己跑一遍看 trace → 再看框架源码 → 再看官方示例 → 最后问我。

**读框架源码的时机**：只在**这一层卡住**的时候读，别一开始就啃。入口是 📁 `openai-agents-python/src/agents/run.py`。

### 7.4 学习沙盒

`my-agent/experiments/` 放"为了理解原理而写的一次性代码"，**不属于业务分层，可以随时删**。
- 📁 `raw_loop.py` —— 第 1 课：不用框架手写 Agent 主循环

---

## 8. 参考索引

**本地**（离线可搜）：
| 内容 | 路径 |
|---|---|
| **官方示例学习指南（本课程的配套素材库）** | `doc/official-examples-guide.md` |
| 中文文档（全） | `openai-agents-python/docs/zh/` |
| 官方示例（218 个） | `openai-agents-python/examples/` |
| 示例运行器（接 DeepSeek 的适配层） | `openai-agents-python/run_example.py` |
| 跑示例的工具 | `cd ../openai-agents-python && uv run run_example.py --list` |
| 框架主循环 | `openai-agents-python/src/agents/run.py` |
| 框架内部实现 | `openai-agents-python/src/agents/run_internal/` |
| HITL 状态机 | `openai-agents-python/src/agents/run_state.py` |
| 本机观测平台接入 | `doc/readme.md` |

**外网**：
- 中文文档：https://openai.github.io/openai-agents-python/zh/
- Anthropic《Building Effective Agents》（**必读**，Agent vs Workflow 那套分类就出自这里）：https://www.anthropic.com/engineering/building-effective-agents
- OTel GenAI 语义约定：https://opentelemetry.io/docs/specs/semconv/gen-ai/

---

## 附录：关键数字速查（面试可能被问到）

| 项 | 值 | 出处 |
|---|---|---|
| SDK 默认 `max_turns` | 10 | `run_config.py:45` |
| 你项目的 `max_turns` | 12 | `cli.py:96` |
| handoff 工具名格式 | `transfer_to_<agent名>` | `handoffs/__init__.py:214` |
| RunState schema 版本 | 1.18 | `run_state.py:232` |
| 你项目单测数 | 9 | `tests/test_repo_tools.py` |
| 框架规模 | src ~25k 行，run_internal ~18.9k 行 | — |
