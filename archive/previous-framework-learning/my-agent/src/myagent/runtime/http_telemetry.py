"""Record one client span per send, with bounded JSON payloads, without consuming response streams."""
from contextlib import contextmanager
from functools import wraps
from importlib import import_module
import json
import re
from urllib.parse import parse_qs, urlsplit, urlunsplit

from opentelemetry import trace
from opentelemetry.instrumentation.utils import is_http_instrumentation_enabled
from opentelemetry.trace import SpanKind, StatusCode

from .telemetry import current_run


def safe_url(value: str) -> str:
    """Drop credentials, query strings and fragments from telemetry URLs."""
    parts = urlsplit(value)
    host = parts.hostname or ""
    if ":" in host:
        host = f"[{host}]"
    if parts.port:
        host += f":{parts.port}"
    return urlunsplit((parts.scheme, host, parts.path, "", ""))


_BODY_LIMIT = 16_384
_PARSE_LIMIT = 1_048_576
_SECRET_KEYS = {"authorization", "proxyauthorization", "apikey", "key", "token", "accesstoken", "refreshtoken", "password", "secret", "clientsecret", "cookie", "setcookie"}


def _redact(value):
    if isinstance(value, dict):
        return {
            key: "[REDACTED]" if re.sub(r"[^a-z0-9]", "", key.lower()) in _SECRET_KEYS else _redact(item)
            for key, item in value.items()
        }
    if isinstance(value, list):
        return [_redact(item) for item in value]
    if isinstance(value, str):
        if value.startswith("data:"):
            return "[data URI omitted]"
        if value.startswith(("https://", "http://")):
            return safe_url(value)
        return re.sub(r"\bBearer\s+[^\s\"]+|\bsk-[A-Za-z0-9_-]+", "[REDACTED]", value, flags=re.IGNORECASE)
    return value


def _json_attribute(span, key, value):
    text = json.dumps(_redact(value), ensure_ascii=False)
    span.set_attribute(key, text[:_BODY_LIMIT])
    span.set_attribute(key + ".truncated", len(text) > _BODY_LIMIT)
    return text[:_BODY_LIMIT], len(text) > _BODY_LIMIT


def record_body(span, message, direction, *, streaming=False):
    """Read only already-buffered JSON; never read a request or response stream."""
    prefix = f"app.http.{direction}"
    span.set_attribute(prefix + ".content_type", message.headers.get("content-type", ""))
    if streaming:
        span.set_attribute(prefix + ".body.capture_status", "skipped_streaming")
        return
    try:
        body = message.content
    except Exception:
        span.set_attribute(prefix + ".body.capture_status", "not_buffered")
        return
    span.set_attribute(prefix + ".body.size", len(body))
    if not body:
        span.set_attribute(prefix + ".body.capture_status", "empty")
        return
    if len(body) > _PARSE_LIMIT:
        span.set_attribute(prefix + ".body.capture_status", "skipped_too_large")
        return
    if "json" not in message.headers.get("content-type", "").lower():
        span.set_attribute(prefix + ".body.capture_status", "skipped_non_json")
        return
    try:
        value = json.loads(body)
        text, truncated = _json_attribute(span, prefix + ".body", value)
    except (ValueError, UnicodeError, RecursionError):
        span.set_attribute(prefix + ".body.capture_status", "invalid_json")
        return
    span.set_attribute(prefix + ".body.capture_status", "captured_json")
    # Reuse the bounded, redacted payload for the trace UI's generic I/O preview.
    # A truncated JSON document must be displayed as plain text.
    preview = "input" if direction == "request" else "output"
    span.set_attribute(preview + ".value", text)
    span.set_attribute(preview + ".mime_type", "text/plain" if truncated else "application/json")
    if direction == "request" and (run := current_run.get()) is not None:
        run.graph.bind_input(span, value)


@contextmanager
def request_span(request, *, stream: bool, debug_http: bool):
    if current_run.get() is None or not is_http_instrumentation_enabled():
        yield None
        return
    url = safe_url(str(request.url))
    parts = urlsplit(url)
    attrs = {
        "http.request.method": request.method,
        "url.full": url,
        "server.address": parts.hostname or "",
        "app.http.response.streaming": stream,
        "app.http.duration_scope": "response_headers" if stream else "response_body",
    }
    with trace.get_tracer("myagent.http").start_as_current_span(
        f"HTTP {request.method} {parts.path or '/'}", kind=SpanKind.CLIENT,
        attributes=attrs, record_exception=False, set_status_on_exception=False,
    ) as span:
        record_body(span, request, "request")
        query = parse_qs(urlsplit(str(request.url)).query, keep_blank_values=True)
        if query:
            _json_attribute(span, "app.http.request.query", query)
        try:
            yield span
        except BaseException as exc:
            span.set_attribute("error.type", type(exc).__name__)
            span.set_status(StatusCode.ERROR, type(exc).__name__)
            raise


def finish_response(span, response, *, debug_http, streaming=False):
    if span is None:
        return
    span.set_attribute("http.response.status_code", response.status_code)
    record_body(span, response, "response", streaming=streaming)
    if response.status_code >= 400:
        span.set_status(StatusCode.ERROR, f"HTTP {response.status_code}")
    if debug_http:
        for name in ("x-request-id", "request-id"):
            if value := response.headers.get(name):
                span.set_attribute("app.http.response.request_id", value)
                break


def instrument_http(*, debug_http: bool = False) -> None:
    """Support both SDK transports, including the synchronous hand-written demo."""
    for module_name in ("httpx", "httpx2"):
        try:
            module = import_module(module_name)
        except ModuleNotFoundError:
            continue
        for cls, asynchronous in ((module.Client, False), (module.AsyncClient, True)):
            original = cls.send
            if getattr(original, "_myagent_patched", False):
                continue

            def make_wrapper(send, asynchronous):
                if asynchronous:
                    @wraps(send)
                    async def wrapped(self, request, *args, **kwargs):
                        with request_span(request, stream=kwargs.get("stream", False), debug_http=debug_http) as span:
                            response = await send(self, request, *args, **kwargs)
                            finish_response(span, response, debug_http=debug_http, streaming=kwargs.get("stream", False))
                            return response
                else:
                    @wraps(send)
                    def wrapped(self, request, *args, **kwargs):
                        with request_span(request, stream=kwargs.get("stream", False), debug_http=debug_http) as span:
                            response = send(self, request, *args, **kwargs)
                            finish_response(span, response, debug_http=debug_http, streaming=kwargs.get("stream", False))
                            return response
                wrapped._myagent_patched = True
                return wrapped

            cls.send = make_wrapper(original, asynchronous)
