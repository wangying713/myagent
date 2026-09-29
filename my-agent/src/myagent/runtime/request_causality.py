"""Run-local, call-ID based edges between model requests and local tools."""
from dataclasses import dataclass, field
from functools import wraps
import json

from opentelemetry import trace


def messages(value):
    """Accept both Chat Completions and Responses SDK payload shapes."""
    if isinstance(value, list):
        return value
    if isinstance(value, dict):
        if isinstance(value.get("choices"), list):
            return [c.get("message", {}) for c in value["choices"] if isinstance(c, dict)]
        for key in ("messages", "output", "input"):
            if isinstance(value.get(key), list):
                return value[key]
    return []


def output_call_ids(value):
    for msg in messages(value):
        if not isinstance(msg, dict):
            continue
        if msg.get("type") == "function_call" and msg.get("call_id"):
            yield msg["call_id"]
        for call in msg.get("tool_calls") or []:
            if isinstance(call, dict) and call.get("id"):
                yield call["id"]


def result_call_ids(value):
    for msg in messages(value):
        if not isinstance(msg, dict):
            continue
        if msg.get("role") == "tool" and msg.get("tool_call_id"):
            yield msg["tool_call_id"]
        elif msg.get("type") == "function_call_output" and msg.get("call_id"):
            yield msg["call_id"]


@dataclass
class RequestGraph:
    models: dict = field(default_factory=dict)
    requests: dict = field(default_factory=dict)
    sources: dict = field(default_factory=dict)
    tools: dict = field(default_factory=dict)
    consumed: dict = field(default_factory=dict)

    def start(self, span):
        attrs = span.attributes or {}
        parent_id = span.parent.span_id if span.parent else 0
        if attrs.get("openinference.span.kind") == "LLM":
            self.models[span.context.span_id] = (span, parent_id)
        elif "http.request.method" in attrs and parent_id in self.models:
            self.requests.setdefault(parent_id, []).append(span)
            span.set_attribute("app.llm.span_id", f"{parent_id:016x}")

    def end(self, span):
        attrs = span.attributes or {}
        if attrs.get("openinference.span.kind") != "LLM":
            return
        model = self.models.pop(span.context.span_id, None)
        if model is None:
            return
        _, scope = model
        requests = self.requests.pop(span.context.span_id, [])
        source = requests[-1] if requests else span
        try:
            value = json.loads(attrs.get("output.value", "null"))
        except (TypeError, ValueError):
            return
        for call_id in output_call_ids(value):
            # Scope by agent span as well as run: different agents can reuse IDs.
            self.sources[(scope, call_id)] = (source.context, source.attributes.get("app.call.sequence", 0), bool(requests))

    def bind_tool(self, span, call_id):
        if not call_id or not span.is_recording():
            return
        scope = span.parent.span_id if span.parent else 0
        span.set_attribute("app.tool.call_id", call_id)
        span.set_attribute("app.tool.executor", "Runner")
        source = self.sources.get((scope, call_id))
        if source:
            context, sequence, is_http = source
            span.set_attribute("app.tool.source.span_id", f"{context.span_id:016x}")
            span.set_attribute("app.tool.source.sequence", sequence)
            span.set_attribute("app.tool.source.kind", "HTTP" if is_http else "LLM")
            span.add_link(context, {"app.link.relation": "requested_tool", "app.tool.call_id": call_id})
        else:
            span.set_attribute("app.tool.source.status", "unmatched")
        self.tools[(scope, call_id)] = (span.get_span_context(), span.attributes.get("app.call.sequence", 0), span.name)

    def bind_input(self, span, value):
        model_id = span.parent.span_id if span.parent else 0
        model = self.models.get(model_id)
        if model is None:
            return
        _, scope = model
        call_ids, sequences, names = [], [], []
        for call_id in dict.fromkeys(result_call_ids(value)):
            tool = self.tools.get((scope, call_id))
            if tool is None:
                continue
            context, sequence, name = tool
            span.add_link(context, {"app.link.relation": "tool_result", "app.tool.call_id": call_id})
            call_ids.append(call_id)
            # Historical results remain linked; only newly consumed results label the row.
            first_consumer = self.consumed.setdefault((scope, call_id), model_id)
            if first_consumer == model_id:
                sequences.append(sequence)
                names.append(name)
        if call_ids:
            span.set_attribute("app.request.tool_result_call_ids", call_ids)
        if sequences:
            span.set_attribute("app.request.new_tool_sequences", sequences)
            span.set_attribute("app.request.new_tool_names", names)


def instrument_tool_context():
    """Capture identity at SDK context creation, without replacing user hooks/tools.

    FunctionSpanData omits call_id. Keep this SDK adapter isolated and regression
    tested against real Runner execution when upgrading the SDK.
    """
    from agents.tool_context import ToolContext
    original = ToolContext.from_agent_context.__func__
    if getattr(original, "_myagent_causality", False):
        return

    @wraps(original)
    def wrapped(cls, *args, **kwargs):
        result = original(cls, *args, **kwargs)
        from .telemetry import current_run
        run = current_run.get()
        span = trace.get_current_span()
        if run and getattr(span, "attributes", {}).get("openinference.span.kind") == "TOOL":
            run.graph.bind_tool(span, result.tool_call_id)
        return result

    wrapped._myagent_causality = True
    ToolContext.from_agent_context = classmethod(wrapped)
