"""Compact SDK wrapper spans before export so logs keep a visible current span.

This adapter relies on OpenInference's processor maps. Keep that dependency here
and verify it with real SDK integration tests when upgrading OpenInference.
"""
from agents.tracing.span_data import TaskSpanData, TurnSpanData
from openinference.instrumentation.openai_agents._processor import OpenInferenceTracingProcessor
from opentelemetry import trace
from opentelemetry.trace import StatusCode

from .telemetry import current_run


class CompactTracingProcessor(OpenInferenceTracingProcessor):
    def __init__(self, tracer):
        super().__init__(tracer)
        self._compact_traces = set()
        self._wrappers = set()

    def on_trace_start(self, sdk_trace):
        parent = trace.get_current_span()
        # Only flatten within the lab's existing run; standalone SDK traces keep a root.
        if current_run.get() is not None and parent.get_span_context().is_valid:
            self._compact_traces.add(sdk_trace.trace_id)
            self._root_spans[sdk_trace.trace_id] = parent
        else:
            super().on_trace_start(sdk_trace)

    def on_trace_end(self, sdk_trace):
        if sdk_trace.trace_id in self._compact_traces:
            self._compact_traces.remove(sdk_trace.trace_id)
            self._root_spans.pop(sdk_trace.trace_id, None)
        else:
            super().on_trace_end(sdk_trace)

    def on_span_start(self, span):
        if span.trace_id in self._compact_traces and isinstance(span.span_data, (TaskSpanData, TurnSpanData)):
            parent = self._otel_spans.get(span.parent_id) or self._root_spans[span.trace_id]
            self._otel_spans[span.span_id] = parent
            self._wrappers.add(span.span_id)
            # Do not attach a new context: logs stay on the nearest visible operation.
            return
        super().on_span_start(span)

    def on_span_end(self, span):
        if span.span_id in self._wrappers:
            self._wrappers.remove(span.span_id)
            parent = self._otel_spans.pop(span.span_id)
            if span.error:
                parent.add_event("sdk.wrapper.error", {"app.sdk.wrapper.type": span.span_data.type,
                                                     "error.message": str(span.error.get("message", "SDK error"))})
                parent.set_status(StatusCode.ERROR)
            return
        super().on_span_end(span)
