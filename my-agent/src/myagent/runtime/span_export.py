"""Display model calls as their actual HTTP attempts, preserving causal links."""
from threading import RLock

from opentelemetry.sdk.trace import ReadableSpan
from opentelemetry.sdk.trace.export import SpanExporter, SpanExportResult
from .preview import preview_attributes, unavailable_attributes


def copy_span(span, **changes):
    fields = dict(
        name=span.name, context=span.context, parent=span.parent,
        resource=span.resource, attributes=span.attributes,
        events=span.events, links=span.links, kind=span.kind,
        status=span.status, start_time=span.start_time, end_time=span.end_time,
        instrumentation_scope=span.instrumentation_scope,
    )
    fields.update(changes)
    return ReadableSpan(**fields)


class DisplaySpanExporter(SpanExporter):
    def __init__(self, delegate):
        self.delegate = delegate
        self._requests = {}
        self._lock = RLock()

    def export(self, spans):
        # HTTP ends before the SDK generation span. Buffer only marked HTTP
        # attempts across export batches until model usage/output is available.
        with self._lock:
            ready = []
            for span in spans:
                attrs = span.attributes or {}
                model_id = attrs.get("app.llm.span_id")
                if model_id:
                    key = (span.context.trace_id, model_id)
                    self._requests.setdefault(key, []).append(span)
                    continue
                key = (span.context.trace_id, f"{span.context.span_id:016x}")
                requests = self._requests.pop(key, []) if attrs.get("openinference.span.kind") == "LLM" else []
                if requests:
                    requests.sort(key=lambda item: item.start_time)
                    for index, request in enumerate(requests):
                        ready.append(self.merge_request(span, request, index, len(requests)))
                else:
                    ready.append(self.for_display(span))
                if attrs.get("app.run.root"):
                    # Incomplete/cancelled instrumentation: never discard an HTTP span.
                    ready.extend(self._drain(span.context.trace_id))
            return self.delegate.export(ready) if ready else SpanExportResult.SUCCESS

    @staticmethod
    def merge_request(model, request, index, count):
        model_attrs = model.attributes or {}
        attrs = dict(request.attributes or {})
        if index == count - 1:
            # Usage belongs to the logical call, not every retry. HTTP I/O remains
            # the wire payload; SDK-normalized I/O is separately inspectable.
            for key, value in model_attrs.items():
                if key.startswith("llm."):
                    attrs[key] = value
            if "llm.token_count.total" not in attrs and "llm.token_count.prompt" in attrs:
                attrs["llm.token_count.total"] = attrs["llm.token_count.prompt"] + attrs.get("llm.token_count.completion", 0)
            for direction in ("input", "output"):
                if (value := model_attrs.get(direction + ".value")) is not None:
                    attrs[f"app.llm.{direction}"] = value
                    # For streams, the SDK has the assembled result after the
                    # HTTP span ends. Label it explicitly as SDK output.
                    if direction + ".value" not in attrs:
                        attrs[direction + ".value"] = value
                        attrs[direction + ".mime_type"] = model_attrs.get(direction + ".mime_type", "text/plain")
                        attrs[f"app.preview.{direction}.source"] = "sdk.assembled"
        attrs.update({
            "app.llm.original_name": model.name,
            "app.llm.sequence": model_attrs.get("app.call.sequence", 0),
            "app.llm.duration_ms": (model.end_time - model.start_time) / 1_000_000,
            "app.http.attempt": index + 1,
            "app.http.attempt_count": count,
        })
        if model_attrs.get("llm.model_name"):
            attrs["llm.model_name"] = model_attrs["llm.model_name"]
        if index == count - 1 and model.status.status_code.name == "ERROR":
            attrs["app.llm.error"] = model.status.description or "model processing failed"
        result = copy_span(
            request, parent=model.parent, attributes=attrs,
            events=tuple(request.events) + (tuple(model.events) if index == count - 1 else ()),
            links=tuple(request.links) + (tuple(model.links) if index == count - 1 else ()),
            status=model.status if index == count - 1 and model.status.status_code.name == "ERROR" else request.status,
        )
        return DisplaySpanExporter.for_display(result)

    @staticmethod
    def for_display(span):
        attrs = dict(span.attributes or {})
        kind = attrs.get("openinference.span.kind")
        sequence = attrs.get("app.call.sequence", "")
        name = span.name
        if "http.request.method" in attrs:
            name = span.name
        elif kind == "LLM":
            name = f"模型调用 {sequence} [{span.name}] · {attrs.get('llm.model_name', span.name)}（无可合并 HTTP）"
        elif kind == "TOOL":
            name = f"工具 {sequence} [{span.name}]"
        if name != span.name:
            attrs["app.span.original_name"] = span.name
        if kind in ("AGENT", "TOOL", "LLM") or "http.request.method" in attrs or attrs.get("app.run.root"):
            DisplaySpanExporter.complete_preview(span, attrs)
        return copy_span(span, name=name, attributes=attrs)

    @staticmethod
    def complete_preview(span, attrs):
        import json
        for direction in ("input", "output"):
            key = direction + ".value"
            if key in attrs:
                value = attrs[key]
                if isinstance(value, str) and attrs.get(direction + ".mime_type") != "text/plain":
                    try:
                        value = json.loads(value)
                    except (ValueError, TypeError):
                        pass
                original_truncated = attrs.get(f"app.preview.{direction}.truncated", False)
                http_direction = "request" if direction == "input" else "response"
                original_truncated |= attrs.get(f"app.http.{http_direction}.body.truncated", False)
                attrs.update(preview_attributes(direction, value, source=attrs.get(
                    f"app.preview.{direction}.source", "recorded")))
                if original_truncated:
                    attrs[f"app.preview.{direction}.truncated"] = True
                continue
            http_direction = "request" if direction == "input" else "response"
            status = attrs.get(f"app.http.{http_direction}.body.capture_status")
            reasons = {
                "empty": "HTTP 正文为空",
                "skipped_streaming": "流式正文未直接采集，且没有可用的 SDK 汇总内容",
                "not_buffered": "正文尚未缓冲，采集器未读取数据流",
                "skipped_too_large": "正文超过采集大小限制",
                "skipped_non_json": "正文不是 JSON，本次未采集",
                "invalid_json": "正文无法解析为 JSON",
            }
            if direction == "output" and span.status.status_code.name == "ERROR":
                reason = "执行失败，未记录返回内容：" + (span.status.description or attrs.get("error.type", "未知错误"))
            else:
                reason = reasons.get(status, "本次执行未记录" + ("输入" if direction == "input" else "最终输出"))
            attrs.update(unavailable_attributes(direction, reason))

    def _drain(self, trace_id=None):
        ready = []
        for key in list(self._requests):
            if trace_id is None or key[0] == trace_id:
                ready.extend(self.for_display(span) for span in self._requests.pop(key))
        return ready

    def shutdown(self):
        self.force_flush()
        self.delegate.shutdown()

    def force_flush(self, timeout_millis=30000):
        with self._lock:
            pending = self._drain()
            if pending:
                self.delegate.export(pending)
            return self.delegate.force_flush(timeout_millis)
