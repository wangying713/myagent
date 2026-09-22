"""模型接入层：全项目**只有这里**知道"我们用的是哪家模型"。

换厂商（DeepSeek → 通义 → 豆包）只改 config.env 的两行 + 这个文件，
业务代码一行不用动。这就是"把外部依赖收口"的价值。
"""

from agents import (
    set_default_openai_api,
    set_default_openai_client,
    set_trace_processors,
)
from agents.tracing.processors import BatchTraceProcessor, ConsoleSpanExporter
from openai import AsyncOpenAI

from .config import Settings


def configure(settings: Settings, *, console_trace: bool = True) -> None:
    """把 SDK 指向我们的模型。

    三件事，一件都不能少：
      1. 建 OpenAI 兼容客户端，指到厂商的 base_url
      2. **切到 Chat Completions** —— SDK 默认走 Responses API，多数厂商不支持
      3. trace 不上传到 OpenAI 平台，改成打到控制台（生产上换成观测平台）
    """
    client = AsyncOpenAI(base_url=settings.base_url, api_key=settings.api_key)
    set_default_openai_client(client, use_for_tracing=False)
    set_default_openai_api("chat_completions")

    if console_trace:
        set_trace_processors([BatchTraceProcessor(ConsoleSpanExporter())])
    else:
        set_trace_processors([])
