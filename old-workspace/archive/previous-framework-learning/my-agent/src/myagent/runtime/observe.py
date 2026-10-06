"""给没有框架自动埋点的手写循环记录一次调用。"""
import json
from typing import Callable, TypeVar
from opentelemetry import trace
from opentelemetry.trace import Status, StatusCode

T = TypeVar("T")


def recorded_call(kind: str, fn: Callable[..., T], *args, **kwargs) -> T:
    name = kwargs.get("model", "chat.completions") if kind == "LLM" else fn.__name__
    with trace.get_tracer("myagent.raw").start_as_current_span(
        name, attributes={"openinference.span.kind": kind}
    ) as span:
        span.set_attribute("input.value", json.dumps(kwargs, ensure_ascii=False, default=lambda x: x.model_dump() if hasattr(x, "model_dump") else str(x)))
        if kind == "LLM":
            span.set_attribute("llm.model_name", kwargs.get("model", "unknown"))
            span.set_attribute("input.mime_type", "application/json")
        else:
            span.set_attribute("tool.name", fn.__name__)
        result = fn(*args, **kwargs)
        output = result.model_dump_json() if hasattr(result, "model_dump_json") else str(result)
        span.set_attribute("output.value", output)
        usage = getattr(result, "usage", None)
        if usage:
            span.set_attribute("llm.token_count.prompt", usage.prompt_tokens)
            span.set_attribute("llm.token_count.completion", usage.completion_tokens)
            span.set_attribute("llm.token_count.total", usage.total_tokens)
        span.set_status(Status(StatusCode.OK))
        return result
