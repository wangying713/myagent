"""离线回归测试支持：假 HTTP、内存 trace；不读取密钥、不连接外部服务。"""

import httpx
from opentelemetry.sdk.trace.export.in_memory_span_exporter import (
    InMemorySpanExporter,
)

from llm import DeepSeekClient
from settings import Settings
from telemetry import Telemetry


def completion(content=None, calls=None):
    """构造测试用的 Chat Completions 响应，不访问真实模型。"""
    message = {"role": "assistant", "content": content}
    if calls:
        message["tool_calls"] = calls
    return {
        "id": "offline-fixture",
        "model": "offline-fixture",
        "choices": [
            {
                "message": message,
                "finish_reason": "tool_calls" if calls else "stop",
            }
        ],
        "usage": {
            "prompt_tokens": 12,
            "completion_tokens": 8,
            "total_tokens": 20,
        },
    }


def offline_client(lesson, handler):
    """创建使用假 HTTP handler 和内存 exporter 的客户端。

    Args:
        lesson: 写入测试 trace 的名称。
        handler: httpx MockTransport 用来生成响应的函数。

    Returns:
        DeepSeekClient 和用于检查 span 的内存 exporter。
    """
    settings = Settings(api_key="offline-placeholder", model="offline-fixture")
    exporter = InMemorySpanExporter()
    telemetry = Telemetry(settings, exporter=exporter)
    return DeepSeekClient(
        lesson,
        settings=settings,
        telemetry=telemetry,
        transport=httpx.MockTransport(handler),
    ), exporter
