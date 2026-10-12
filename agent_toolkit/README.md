# Agent 通用工具包

这里存放可复用的 Agent 基础组件。课程示例通过 `agent_toolkit` 包导入，提示词、具体工具和业务资料由调用方提供。

## 模块职责

| 模块 | 用途 |
|---|---|
| [agent_loop.py](agent_loop.py) | 有限 Agent 循环、工具结果回填、轮次与预算控制 |
| [tools.py](tools.py) | 将 LangChain 工具转换为模型描述，校验并分发工具调用 |
| [llm.py](llm.py) | DeepSeek HTTP 客户端、普通与流式响应解析、工具执行观测 |
| [settings.py](settings.py) | 配置读取与校验，支持指定 `.env` 路径 |
| [telemetry.py](telemetry.py) | trace 与 span、正文脱敏、限长、OpenObserve 上报 |
| [session_store.py](session_store.py) | SQLite 会话隔离与版本冲突检查 |

乘法工具、虚构产品检索和评估用例属于课程业务，分别保留在 `agent-learning/toolbox.py`、`retrieval.py` 和 `data/` 中。

## 安装与导入

课程项目已将本工具包声明为本地可编辑依赖。在课程目录执行一次同步即可使用：

```bash
cd agent-learning
uv sync --locked
```

可编辑安装直接使用本目录的源码，修改工具包后无需反复安装。其他 Python 项目可以在自己的虚拟环境中安装仓库根目录的项目。

```python
from agent_toolkit.agent_loop import BudgetExceededError, run_agent
from agent_toolkit.llm import DeepSeekClient, ModelError, final_text
from agent_toolkit.session_store import SessionConflictError, SessionStore
from agent_toolkit.settings import Settings
from agent_toolkit.tools import ToolSet
```

`run_agent()` 接收客户端、问题，以及调用方提供的 `system_prompt`、`tools` 和 `dispatch`，返回最终文本回答。具体用法见 [初级 Agent demo](../agent-learning/demos/beginner/beginner_03_agent_loop.py)。

工具可以通过 `langchain_core.tools.tool` 装饰器声明。`ToolSet([tool_a, tool_b])` 从同一份列表生成 `.schemas` 和 `.dispatch`，分别传给循环的 `tools` 和 `dispatch` 参数。业务函数使用普通类型注解与默认值；`ToolSet` 统一配置严格类型校验、拒绝多余参数及非有限数值。`float` 参数接受整数和浮点数，并按浮点数运算。业务有额外限制时再显式声明参数约束。这里只使用 `langchain-core` 的工具组件，模型客户端与 Agent 循环仍由本项目实现。

## 配置与生命周期

在本仓库中默认读取 `agent-learning/.env`，不受启动目录影响；环境变量优先。其他项目可指定配置文件，再将配置传给客户端：

```python
settings = Settings.from_env(env_path="/path/to/project/.env")
with DeepSeekClient("my_experiment", settings=settings) as llm_client:
    response_data = llm_client.chat(
        [{"role": "user", "content": "用一句话介绍 Agent。"}]
    )
    print(final_text(response_data))
```

上面的示例会调用真实模型并消耗 token。客户端必须在 `with` 块内使用，退出时关闭 HTTP 连接并结束观测记录。

测试集中在课程项目的 `tests/`，使用假 HTTP 和内存 exporter：

```bash
cd agent-learning
uv run python -m unittest discover -s tests -v
```
