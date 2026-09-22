# my-agent

用 [OpenAI Agents SDK](https://openai.github.io/openai-agents-python/zh/) 搭的**仓库分析助手**。

**这里是我们的项目目录，业务代码都写在这。** SDK 是从 PyPI 装的依赖，不是克隆源码改——
框架源码放在隔壁 `../openai-agents-python/`，那份只当参考资料读。

---

## 一、目录结构

```
my-agent/
├── pyproject.toml              # 依赖声明（openai-agents）+ 命令行入口定义
├── config.env                  # 配置：API Key / 厂商地址 / 模型名 / 工作根目录
├── .gitignore                  # 已挡住 config.env，Key 不会入库
│
├── src/myagent/
│   ├── __init__.py             # 包入口：只在"导入前"设置必需的 SDK 开关
│   │
│   ├── prompts/                # ① 提示词（独立成文件，改它不用动代码）
│   │   └── repo_agent.md
│   │
│   ├── definitions/            # ② Agent 定义：只做组装，零逻辑
│   │   └── repo_agent.py
│   │
│   ├── tools/                  # ③ 模型能"做"什么 —— 每个函数只做薄壳
│   │   ├── guards.py           #    护栏 + FsContext（被所有工具复用）
│   │   └── repo.py             #    仓库域的三个工具 + REPO_TOOLS 清单
│   │
│   ├── services/               # ④ 真正的业务逻辑（查库/调 RPC/算规则），当前仅占位
│   │
│   └── runtime/                # ⑤ 跟框架/厂商打交道，全在这一层
│       ├── config.py           #    全项目唯一读配置的地方
│       ├── llm.py              #    全项目唯一知道"用哪家模型"的地方
│       └── cli.py              #    命令行入口
│
└── tests/
    └── test_repo_tools.py      # 不联网、不花 token 的单测
```

### 依赖方向（唯一需要守住的硬规矩）

```
runtime/  ──▶  definitions/  ──▶  tools/  ──▶  services/
（外层）                                        （内层）

箭头只能单向：tools 不许反过来 import runtime / definitions
```

内层完全不知道 SDK 的 `Agent`、`Runner` 存在，所以**能脱离 LLM 单独测试**——`tests/` 那 9 个用例就是这么来的。

### 每层职责（对照你熟悉的后端分层）

| 这层 | 干什么 | 对应后端 |
|---|---|---|
| `prompts/` | 提示词文本 | 模板/配置 |
| `definitions/` | 把 prompt + tools + model 组装成一个 Agent | 装配（wire） |
| `tools/` | 参数校验 + 护栏 + 调 services + 裁剪返回 | **handler** |
| `services/` | 真正的业务逻辑 | **service / logic** |
| `runtime/` | 配置、模型接线、入口 | `main` + 基础设施 |

### 两条关键设计

1. **工具不读全局变量**，依赖从 SDK 的 `RunContextWrapper` 注入
   ```python
   @function_tool
   def list_dir(ctx: RunContextWrapper[FsContext], path: str = ".") -> str:
       root = ctx.context.root      # ← 每个请求带进来的，不是全局变量
   ```
   好处：同一进程能同时跑多个不同根目录的 Agent（多项目/多租户），测试时传个假 root 即可。
   `ctx` 参数**不会**出现在给模型的 Schema 里（有单测守着这一点）。

2. **护栏在工具层拦死，不指望模型自觉**
   - `safe_resolve`：拒绝绝对路径、拒绝 `../` 越界
   - `reject_sensitive`：`config.env` / `*.env` / `id_rsa` / `*.pem` 一律不给读
   - 理由：Agent 读到的东西 = 会流进 prompt 发给远端 + 落进 trace

---

## 二、启动方式

### 前置条件

| 需要 | 说明 |
|---|---|
| [uv](https://docs.astral.sh/uv/) | 本机已装（`uv --version`）。它也会自动准备 Python ≥ 3.10 |
| 一个支持 Function Calling 的模型 Key | 默认 DeepSeek；换厂商见"四、扩展" |

### 三步跑起来

```bash
cd /Users/wangying/apps/sakelei/ai/my-agent

# 1) 填 Key：打开 config.env，把 Key 贴在 OPENAI_API_KEY= 右边，保存
#    （只需要改这一行，其它三行有默认值）

# 2) 装依赖（第一次要几十秒）
uv sync

# 3) 跑
uv run myagent -q "这个项目有哪些文件？护栏函数在哪个文件？"
```

### 配置项（`config.env`）

| 键 | 说明 | 默认值 |
|---|---|---|
| `OPENAI_API_KEY` | **必填**，厂商的 Key | 无 |
| `OPENAI_BASE_URL` | 厂商的 OpenAI 兼容地址 | `https://api.deepseek.com/v1` |
| `MODEL` | 模型名 | `deepseek-chat` |
| `AGENT_ROOT` | 工具能访问的工作根目录 | `.`（即本项目） |

**优先级**：临时 `export` 的环境变量 > `config.env`。想试一次别的 Key，直接 `OPENAI_API_KEY=sk-xxx uv run myagent ...`，不用改文件。

---

## 三、使用方式

### 1) 单次提问

```bash
uv run myagent -q "read_file 工具是怎么做的？给我行号"
uv run myagent -q "统计 src 下有多少个 py 文件" --no-trace     # 关掉 trace 噪音
uv run myagent -q "分析一下这个项目" --max-turns 20           # 放宽轮数上限
uv run myagent -q "看看 xx 项目" --root ~/apps/some-repo       # 换个分析目标
```

### 2) 交互模式（推荐，能看到上下文怎么涨）

```bash
uv run myagent
> 这个项目分几层？
> 护栏是怎么实现的？
> exit
```

每轮都会打印**每次请求的 token 明细**，你会发现 `prompt` 一路累加、累计值远超最后一次——那就是上下文 O(n²) 的账单。

### 3) 跑官方示例（218 个，在隔壁目录）

```bash
cd ../openai-agents-python
uv run run_example.py --list                                  # 看推荐清单
uv run run_example.py examples/basic/hello_world.py           # 最小示例
uv run run_example.py examples/handoffs/message_filter.py     # 多 Agent 交接
```

### 4) 跑单测

```bash
cd /Users/wangying/apps/sakelei/ai/my-agent
uv run pytest -q          # 9 个用例，不联网、不花 token、1 秒内
```

### 命令行参数

| 参数 | 作用 | 默认 |
|---|---|---|
| `-q, --question` | 问一句就走；不传则进交互模式 | 空（交互） |
| `--root` | 工作根目录，覆盖 `AGENT_ROOT` | `config.env` 的值 |
| `--max-turns` | 单次提问最多几轮（熔断） | `12` |
| `--no-trace` | 关掉控制台的 trace 输出 | 关闭（即默认打印） |

---

## 四、扩展方式

### 加一个工具

1. 在 `src/myagent/tools/` 下**按业务域**新建文件（如 `order.py`），写函数：

```python
from agents import RunContextWrapper, function_tool
from .guards import FsContext

@function_tool
def query_order(ctx: RunContextWrapper[FsContext], order_id: str) -> str:
    """按订单号查订单状态。

    当用户问"我的订单到哪了"时用它。
    想查用户信息请改用 query_user。

    Args:
        order_id: 订单号，形如 202609220001。
    """
    data = order_service.get(order_id)     # ← 真逻辑放 services/
    return f"订单 {order_id} 状态：{data['status']}"
```

2. 在 `tools/__init__.py` 里导出（或直接建 `ORDER_TOOLS = [...]`），再在 `definitions/*.py` 的 `tools=[...]` 里加上。

**docstring 就是给模型的接口文档**，要写清"什么时候用 / 什么时候别用"。参数描述走 `Args:`（Google 风格），SDK 会自动解析成 JSON Schema。

### 加一个 Agent

在 `definitions/` 下新建文件，只做组装：

```python
def build_order_agent(model: str) -> Agent[FsContext]:
    return Agent[FsContext](
        name="订单助手",
        instructions=(PROMPT_DIR / "order_agent.md").read_text(encoding="utf-8"),
        model=model,
        tools=list(ORDER_TOOLS),
    )
```

提示词放 `prompts/order_agent.md`，别写死在代码里。

### 改提示词

直接改 `src/myagent/prompts/repo_agent.md`，不用动任何代码。

### 换模型厂商

只改两处：

1. `config.env` 的 `OPENAI_BASE_URL` 和 `MODEL`
2. 如果新厂商不支持 Chat Completions 之外的差异，看 `runtime/llm.py`（就那 3 行接线）

```env
# 通义：https://dashscope.aliyuncs.com/compatible-mode/v1  /  qwen-plus
# Kimi：https://api.moonshot.cn/v1                         /  moonshot-v1-32k
# 智谱：https://open.bigmodel.cn/api/paas/v4               /  glm-4-flash
```

⚠️ **易踩的坑**：SDK 默认走 OpenAI 的 `Responses API`，多数厂商只支持 `Chat Completions`，必须在 `runtime/llm.py` 里调 `set_default_openai_api("chat_completions")`。这一行已经加好了，别删。

---

## 五、分层纪律（防止越写越乱）

| # | 纪律 | 为什么 |
|---|---|---|
| 1 | 工具不读全局变量，依赖走 `RunContextWrapper` | 支持多根目录/多租户；工具保持纯函数可测 |
| 2 | 一个业务域一个工具文件，> 8 个工具就拆 | 单文件超 300 行说明域没拆开 |
| 3 | 提示词不进代码，统一放 `prompts/` | 便于迭代、审计、A/B |
| 4 | 只有 `runtime/config.py` 读配置，只有 `runtime/llm.py` 知道厂商 | 换模型改 1 个文件；别处不许出现 `os.getenv` |
| 5 | 工具保持薄壳，业务逻辑放 `services/` | 业务能脱离 LLM 单测和复用 |
| 6 | `definitions/` 里只组装，不写逻辑（文件 > 100 行就该拆） | 逻辑放错层是最常见的腐化 |

### 出现这些信号，说明该重构了

| 信号 | 怎么改 |
|---|---|
| 某个 `tools/*.py` > 300 行 或 > 8 个工具 | 按业务域拆文件 |
| 提示词在代码里出现 ≥ 2 次 | 抽到 `prompts/` |
| 工具函数里同时有 RPC + 业务规则 + 字符串拼接 | 抽到 `services/`，工具只留薄壳 |
| 别处出现 `os.getenv` | 收敛到 `runtime/config.py` |
| 两个 `definitions/*.py` 互相 import | 改用 `handoffs`，或抽公共层 |

---

## 六、两个目录的分工

| 目录 | 定位 | 往里写东西吗 |
|---|---|---|
| **`my-agent/`**（本目录） | 我们的项目 | ✅ 只写这里 |
| `../openai-agents-python/` | 参考资料：框架源码（25k 行）+ 218 个官方示例 + `run_example.py` 工具 | ❌ 只读、只跑 |

想看框架内部怎么实现，去读 `../openai-agents-python/src/agents/run.py`（主循环）和 `run_internal/`（真正干活的地方，18.9k 行）。

---

## 七、常见问题

| 现象 | 原因 / 怎么办 |
|---|---|
| `HTTP 401 Authentication Fails` | Key 和 `OPENAI_BASE_URL` 不是同一家厂商。DeepSeek 的 Key 以 `sk-` 开头，火山方舟（豆包）的是 UUID 形状且 `MODEL` 要填 `ep-` 开头的接入点 ID |
| `API key format is incorrect` | 同上，Key 的格式跟厂商对不上 |
| `UnicodeEncodeError: 'ascii' codec ...` | Key 里混了非 ASCII 字符（占位符没换成真的） |
| `还没填 Key` | `config.env` 里 `OPENAI_API_KEY` 还是占位符 |
| 工具返回"拒绝访问 config.env" | **这是正常的**，敏感文件护栏在生效 |
| `[Exporter]` 输出刷屏 | 那是 trace，加 `--no-trace` 关掉；或 `uv run myagent -q "..." \| less` |
| 模型一直调工具不收敛 | 已被 `--max-turns` 熔断；把问题问得更具体，或提高 `--max-turns` |
| 上下文太长导致变慢/变贵 | 交互模式注意用 `exit` 重开；单次提问比连续追问省 token |

---

## 八、参考

- **学习路线与面试考点：`../doc/agent-course.md`**（分 8 个 Stage 的实训课程，含 Go 类比）
- **官方示例导读：`../doc/official-examples-guide.md`**（218 个示例怎么读，含 DeepSeek 兼容边界）
- 中文文档：https://openai.github.io/openai-agents-python/zh/
- 本地中文文档（离线、可搜索）：`../openai-agents-python/docs/zh/`
- 官方示例：`../openai-agents-python/examples/`
- 本项目结构依据：PyPA 的 [src 布局](https://packaging.python.org/en/latest/discussions/src-layout-vs-flat-layout/) + [12-Factor 的配置原则](https://12factor.net/config) + Clean Architecture 的依赖方向

---

## 附：一次请求发生了什么

```
你提问
  └─ runtime/cli.py          组装历史 + 注入 FsContext + 设 max_turns
       └─ Runner.run()       = 那个 for 循环（Reason → Act → Observe）
            ├─ 发给模型       messages + tools 清单
            ├─ 模型说要调工具   tools/repo.py 里的函数被执行
            │    └─ guards.py 先过护栏（路径/敏感文件）
            ├─ 结果塞回 messages
            └─ 模型不再要工具 → 返回最终回答 + 用量
```
