"""运行摘要、清晰的 span 名称，以及保留终端行为的 print 采集。"""
from __future__ import annotations

from contextlib import contextmanager, redirect_stderr, redirect_stdout
from contextvars import ContextVar
from dataclasses import dataclass, field
import asyncio
import sys
import logging
import threading

from opentelemetry import context, trace
from opentelemetry.sdk.trace import SpanProcessor
from .request_causality import RequestGraph


@dataclass
class RunSummary:
    file: str
    model: str = ""
    base_url: str = ""
    llm_calls: int = 0
    tool_calls: int = 0
    input_tokens: int = 0
    output_tokens: int = 0
    errors: int = 0
    warnings: int = 0
    graph: RequestGraph = field(default_factory=RequestGraph)
    http_calls: int = 0
    def write(self, span):
        for key in ("llm_calls", "tool_calls", "http_calls", "input_tokens", "output_tokens"):
            span.set_attribute(f"app.run.{key}", getattr(self, key))
        span.set_attribute("app.run.total_tokens", self.input_tokens + self.output_tokens)
        span.set_attribute("app.run.error_span_count", self.errors)
        span.set_attribute("app.run.warning_log_count", self.warnings)


current_run: ContextVar[RunSummary | None] = ContextVar("app_run", default=None)


class DemoSpanProcessor(SpanProcessor):
    """只补语义和摘要，不改动 SDK 的父子关系，不重复导出。"""
    def on_start(self, span, parent_context=None):
        run = current_run.get()
        if run is None:
            return
        span.set_attribute("app.run.entrypoint", run.file)
        kind = span.attributes.get("openinference.span.kind", "")
        if kind == "LLM":
            run.llm_calls += 1
            span.set_attribute("app.call.sequence", run.llm_calls)
        elif kind == "TOOL":
            run.tool_calls += 1
            span.set_attribute("app.call.sequence", run.tool_calls)
        elif span.kind.name == "CLIENT":
            run.http_calls += 1
            span.set_attribute("app.call.sequence", run.http_calls)
        if kind == "LLM":
            # 自动埋点结束时会补充实际模型；这里补全手写循环的同名字段。
            if run.model:
                span.set_attribute("llm.model_name", run.model)
            if run.base_url:
                span.set_attribute("app.llm.base_url", run.base_url)
        run.graph.start(span)

    def on_end(self, span):
        run = current_run.get()
        if run is None or span.attributes.get("app.run.root"):
            return
        attrs = span.attributes
        run.graph.end(span)
        kind = attrs.get("openinference.span.kind")
        if kind == "LLM":
            run.input_tokens += int(attrs.get("llm.token_count.prompt", 0))
            run.output_tokens += int(attrs.get("llm.token_count.completion", 0))
        if span.status.status_code.name == "ERROR":
            run.errors += 1


class DemoLogFilter(logging.Filter):
    def filter(self, record):
        if record.name.startswith("opentelemetry"):
            return False
        if record.name.startswith(("httpx", "httpcore", "openai", "openinference")) and record.levelno < logging.WARNING:
            return False
        record.__dict__.setdefault("event.name", "app.log")
        run = current_run.get()
        if run:
            record.__dict__["app.run.entrypoint"] = run.file
            if record.levelno == logging.WARNING:
                run.warnings += 1
        return True


class PrintStream:
    """按行采集 stdout/stderr，防止日志 handler 写回 stderr 时递归。"""
    def __init__(self, original, name):
        self.original = original
        self.name = name
        self.pending = {}
        self.local = threading.local()
        self.lock = threading.RLock()

    def __getattr__(self, name):
        return getattr(self.original, name)

    def _emit(self, line, ctx):
        if not line:
            return
        self.local.busy = True
        token = context.attach(ctx)
        try:
            logging.getLogger(f"app.console.{self.name}").info(line, extra={"app.console.stream": self.name, "event.name": "console.output"})
        finally:
            context.detach(token)
            self.local.busy = False

    def write(self, value):
        result = self.original.write(value)
        if getattr(self.local, "busy", False) or sys._getframe(1).f_globals.get("__name__") == "logging":
            return result
        with self.lock:
            try:
                task = asyncio.current_task()
            except RuntimeError:
                task = None
            sc = trace.get_current_span().get_span_context()
            key = (threading.get_ident(), task, sc.trace_id, sc.span_id)
            pending, ctx = self.pending.pop(key, ("", context.get_current()))
            parts = (pending + value).split("\n")
            for line in parts[:-1]:
                line = line.rstrip("\r")
                for offset in range(0, len(line), 16000):
                    self._emit(line[offset:offset + 16000], ctx)
            # 限制单条日志大小，长输出分块保留，不悄悄丢掉。
            tail = parts[-1]
            while len(tail) > 16000:
                self._emit(tail[:16000], ctx)
                tail = tail[16000:]
            if tail:
                self.pending[key] = (tail, ctx)
        return result

    def flush(self):
        self.original.flush()
        if getattr(self.local, "busy", False):
            return
        with self.lock:
            for pending, ctx in self.pending.values():
                self._emit(pending, ctx)
            self.pending.clear()


@contextmanager
def capture_console():
    import sys
    stdout, stderr = PrintStream(sys.stdout, "stdout"), PrintStream(sys.stderr, "stderr")
    try:
        with redirect_stdout(stdout), redirect_stderr(stderr):
            yield
    finally:
        stdout.flush()
        stderr.flush()
