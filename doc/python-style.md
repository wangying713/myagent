# Python 代码与注释规范

本文适用于本仓库正在维护的 Python 代码，主要是 agent-learning/。归档目录和第三方 vendored 代码保留原样。

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
def retry_chat(model, messages, *, max_attempts=3):
    """对临时 HTTP 错误进行有限重试。

    Args:
        model: 提供 chat 方法的模型客户端。
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

## 验证

从 agent-learning/ 目录运行：

~~~
uv run --with ruff ruff format --check .
uv run --with ruff ruff check --select I .
uv run python -m unittest discover -s tests -v
~~~

本项目在 pyproject.toml 中将 Ruff 行宽设为 80，并检查导入排序。格式化只调整排版；逻辑变更仍需用离线测试验证。

## 规范依据

本规范以 [Google Python Style Guide 的 Comments and Docstrings 部分](https://google.github.io/styleguide/pyguide.html#38-comments-and-docstrings)、[PEP 257](https://peps.python.org/pep-0257/) 和 [PEP 8](https://peps.python.org/pep-0008/) 为基础，并为初学者课程补充了中文说明约定。
