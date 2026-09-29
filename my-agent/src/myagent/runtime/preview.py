"""Bounded, redacted previews with explicit provenance and empty-value handling."""
import json

_LIMIT = 16_384


def preview_attributes(direction, value, *, source):
    from .http_telemetry import _redact

    def encode(item):
        if hasattr(item, "model_dump"):
            return item.model_dump(mode="json")
        return str(item)

    # Normalize Pydantic models/tuples before applying the shared redaction policy.
    normalized = json.loads(json.dumps(value, ensure_ascii=False, default=encode))
    normalized = _redact(normalized)
    plain_text = isinstance(normalized, str) and bool(normalized)
    text = normalized if plain_text else json.dumps(normalized, ensure_ascii=False)
    truncated = len(text) > _LIMIT
    if truncated:
        suffix = "\n…（内容已截断）"
        text = text[:_LIMIT - len(suffix)] + suffix
    return {
        direction + ".value": text,
        direction + ".mime_type": "text/plain" if plain_text or truncated else "application/json",
        f"app.preview.{direction}.source": source,
        f"app.preview.{direction}.truncated": truncated,
    }


def set_preview(span, direction, value, *, source):
    try:
        attrs = preview_attributes(direction, value, source=source)
    except (TypeError, ValueError, RecursionError):
        attrs = unavailable_attributes(direction, "内容无法序列化，未生成预览")
    span.set_attributes(attrs)


def unavailable_attributes(direction, reason):
    return preview_attributes(direction, {"说明": reason}, source="capture_status")
