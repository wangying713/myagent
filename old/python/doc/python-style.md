# Python 代码与注释规范

本文适用于本仓库正在维护的 Python 代码，包括 agent_toolkit/、agent-learning/ 和 python-learning/。命名形式采用 Google Python Style Guide，中文说明与教学注释遵循下文的项目约定。归档目录和第三方 vendored 代码保留原样。

## 文档字符串放在哪里

Python 使用 docstring（文档字符串）描述模块、类、函数和方法。它是函数或类体内的第一条字符串语句，可以通过 help() 和 __doc__ 查看。

~~~
def load_session(session_id):
    """读取指定会话及其版本号。"""
    ...
~~~

类和方法分别说明自己的用途：

~~~
class SessionStore:
    """按会话 ID 保存消息，并检查并发覆盖。"""

    def load(self, session_id):
        """读取消息和版本号；新会话返回空列表和版本 0。"""
        ...
~~~

井号注释可以放在定义前面或代码旁边，但不能替代 docstring。要解释函数或类的用途时，把 docstring 放在定义之后、函数体或类体的第一行。

## Docstring 内容

- 使用三引号和中文简体，第一句简洁说明用途，并以句号结束。
- 对外或较复杂的函数说明调用方需要知道的信息，而不是复述函数内部每一行。
- 参数、返回值或可能抛出的异常不明显时，使用 Args:、Returns:、Raises: 小节。
- 参数名应与代码签名一致；如果有副作用、资源生命周期或调用限制，要明确写出。
- 类 docstring 说明类的职责；有重要的公开属性时，用 Attributes: 列出。
- 小型、命名清楚的测试函数通常不需要重复 docstring；测试名称本身应说明验证行为。

示例：

~~~
def retry_chat(llm_client, messages, *, max_attempts=3):
    """对临时 HTTP 错误进行有限重试。

    Args:
        llm_client: 提供 chat 方法的模型客户端。
        messages: 发给模型的消息列表。
        max_attempts: 包括首次请求在内的最大尝试次数。

    Returns:
        成功请求的响应字典。

    Raises:
        ModelError: 错误不可重试或重试次数已用尽时。
    """
    ...
~~~

## 行内注释

- 解释原因、约束、安全边界或不明显的控制流程。
- 不要把代码换句话再写一遍；每行都加注释通常会让阅读更慢。
- 注释应紧挨需要解释的代码，使用完整、清楚的句子。
- 面向初学者的课程 demo 可以解释语法概念；底层工具代码优先解释设计原因和副作用。

## 排版与命名

- 使用 4 个空格缩进，不使用 Tab。
- 每行最多 80 个字符；长调用、条件和字典按逻辑分行，不用难读的单行压缩代码。
- 导入顺序为标准库、第三方库、本项目模块；每组之间空一行。
- 变量和函数使用 snake_case，类使用 PascalCase，常量使用 UPPER_SNAKE_CASE。
- 重要的公开接口优先补充类型注解；类型注解补充说明，不替代 docstring。
- 函数应保持一个清晰职责；复杂表达式拆成有意义的中间变量。

## 统一命名形式

本项目采用 [Google Python Style Guide 3.16 Naming](https://google.github.io/styleguide/pyguide.html#316-naming) 的命名形式。它按代码元素区分写法：变量、参数、函数和方法使用 snake_case；类使用大驼峰；常量使用大写下划线。

| 代码元素 | 统一写法 | 示例 |
|---|---|---|
| Python 文件 / 模块 | `snake_case.py` | `agent_loop.py` |
| Python 包 | `snake_case` | `demos`、`intermediate` |
| 变量、参数、实例属性 | `snake_case` | `llm_client`、`max_tool_calls`、`self.total_tokens` |
| 函数、方法 | `snake_case` | `run_agent()`、`grade_case()` |
| 类 | `PascalCase`（Google 称 CapWords） | `DeepSeekClient`、`SessionStore` |
| 异常类 | `PascalCase`，以 `Error` 结尾 | `ModelError`、`BudgetExceededError`、`SessionConflictError` |
| 常量 | `UPPER_SNAKE_CASE` | `SYSTEM_PROMPT`、`DATA_PATH` |
| 内部函数、方法、属性 | `_snake_case` | `_build_request_body()`、`self._active` |
| Python 协议方法 | 保留 Python 规定名称 | `__init__()`、`__enter__()` |

因此，本项目使用 `llm_client` 和 `run_agent()`。`llmClient`、`runAgent()` 属于小驼峰，采用它们意味着另行制定项目规范，不能称为 Google Python 命名规范。

异常类的 `Error` 后缀依据 [Google Python Style Guide 2.4 Exceptions](https://google.github.io/styleguide/pyguide.html#24-exceptions)。

第三方 API、继承接口和协议规定的名字保留原样。后面的名词、动词、复数和布尔值建议属于本项目的可读性补充，不是 Google 强制的统一前缀。

## 命名表达职责

符合 snake_case 只是命名的形式要求。名字还应帮助读者理解值的含义或函数的职责，尤其是面向初学者的课程代码。

- 变量用名词或名词短语：`question`、`messages`、`response_data`。
- 函数和方法优先用动词或动词短语：`run_agent`、`search_documents`、`grade_case`。在类的上下文已经明确时，`SessionStore.load()`、`save()` 等简短名字足够清楚。
- 布尔值应读起来像判断：`is_answer_correct`、`is_tool_correct`、`within_budget`。不强制所有布尔值加同一个前缀。
- 集合用复数，元素用单数：`evaluation_results` / `evaluation_result`、`tool_calls` / `call`。
- 次数、时间和单位应明确：`tool_call_count`、`passed_count`、`start_time`、`elapsed_ms`。不需要给所有变量添加类型后缀。
- 跨多行使用的业务变量避免只有 `data`、`rows` 或单个字母。乘法的 `a`、`b`、表示未使用值的 `_`，以及约定俗成的 `self`、`cls`、`args`、`kwargs`、`exc` 可以保留。
- 区分相近概念：`llm_client` 是提供模型请求方法的客户端；`Settings.model` 和请求 JSON 中的 `model` 是模型名称。API 字段名遵循协议，不随本地变量重命名。
- 类的内部辅助方法使用单下划线开头，例如 `_build_request_body()`。Python 协议方法如 `__enter__()`、`__exit__()` 保持规定名称。
- 重命名函数或参数时，同步检查调用方、测试、docstring 和课程文档。

当前课程约定：

| 含义 | 命名 |
|---|---|
| DeepSeek 客户端 | `llm_client` |
| 模型返回的完整响应字典 | `response_data` |
| 解码后的工具参数 | `arguments` |
| 全部评估结果 / 单条结果 | `evaluation_results` / `evaluation_result` |
| 单个评估用例的评分 | `case_score` |
| 检查一个用例的答案与工具证据 | `grade_case()` |

例如，评估流程可以直接从名字读出各对象的职责：

~~~python
llm_client = DeepSeekClient("advanced_eval_" + case["id"])
with llm_client:
    answer = run_agent(
        llm_client=llm_client,
        question=question,
        system_prompt=system_prompt,
        tools=[multiply],
    )
case_score = grade_case(case, answer, llm_client.tool_results)
~~~

## 验证

从仓库根目录运行格式与命名检查：

~~~
uv run --with ruff ruff format --check .
uv run --with ruff ruff check .
~~~

从 agent-learning/ 目录运行离线测试：

~~~
uv run python -m unittest discover -s tests -v
~~~

仓库根目录的 ruff.toml 是唯一的 Ruff 配置，行宽为 80，启用导入排序（I）和命名检查（N），并排除 old-workspace/。从 Agent 课程目录运行 Ruff 也会读取该配置。工具检查不能判断业务含义是否清楚，仍需人工阅读。格式化只调整排版；逻辑变更仍需用离线测试验证。

## 规范依据

命名形式依据 [Google Python Style Guide 3.16 Naming](https://google.github.io/styleguide/pyguide.html#316-naming)；文档字符串参考其 [3.8 Comments and Docstrings](https://google.github.io/styleguide/pyguide.html#38-comments-and-docstrings)、[PEP 257](https://peps.python.org/pep-0257/) 和 [PEP 8](https://peps.python.org/pep-0008/)。中文说明和业务名称建议是本项目的补充约定；本项目没有声称逐条实现完整的 Google Python Style Guide。
