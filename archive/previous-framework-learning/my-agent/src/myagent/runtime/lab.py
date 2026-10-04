"""统一运行 demo：python -m myagent.runtime.lab experiments/xxx.py [参数]。"""
from __future__ import annotations

import argparse
import json
import logging
import os
from pathlib import Path
import runpy
import sys

from opentelemetry import trace
from opentelemetry.trace import Status, StatusCode

from .config import load_settings
from .llm import configure
from .observability import flush, setup
from .telemetry import RunSummary, capture_console, current_run
from .preview import set_preview


def run_demo(script: Path, args: list[str], *, debug_http: bool = False) -> None:
    """保留脚本自己的入口、参数与退出码，统一管理观测生命周期。"""
    setup(debug_http=debug_http)
    old_argv, old_path = sys.argv, sys.path[:]
    old_model = os.environ.get("OPENAI_DEFAULT_MODEL")
    summary = RunSummary(script.name)
    summary_token = current_run.set(summary)
    try:
        with trace.get_tracer("myagent.lab").start_as_current_span(
            script.name, record_exception=False, set_status_on_exception=False
        ) as span:
            span.set_attribute("app.run.entrypoint", script.name)
            span.set_attribute("app.run.root", True)
            span.set_attribute("app.run.arguments", json.dumps(args, ensure_ascii=False))
            set_preview(span, "input", {"script": script.name, "arguments": args}, source="run.arguments")
            print(f"[lab] {script.name} | trace_id={span.get_span_context().trace_id:032x}")
            try:
                # 纯 Python demo 不需要模型密钥；配置失败也记入本次运行。
                settings = load_settings(allow_missing_key=True)
                if settings.api_key:
                    summary.model = settings.model
                    from .http_telemetry import safe_url
                    summary.base_url = safe_url(settings.base_url)
                    # configure 会重置 processors；先配置模型再重新接入是不安全的。
                    # 这里只设置客户端，保留已安装的观测 processor。
                    configure(settings, console_trace=False, reset_tracing=False)
                    os.environ.setdefault("OPENAI_DEFAULT_MODEL", settings.model)
                sys.argv = [str(script), *args]
                sys.path.insert(0, str(script.parent))
                with capture_console():
                    runpy.run_path(str(script), run_name="__main__")
            except SystemExit as exc:
                if exc.code not in (None, 0):
                    span.set_status(Status(StatusCode.ERROR, str(exc)))
                    span.record_exception(exc)
                    span.set_attribute("app.run.status", "error")
                else:
                    span.set_status(Status(StatusCode.OK))
                    span.set_attribute("app.run.status", "ok")
                raise
            except BaseException as exc:
                span.set_attribute("app.run.status", "error")
                span.set_status(Status(StatusCode.ERROR, str(exc)))
                span.record_exception(exc)
                logging.getLogger("lab").exception("执行失败：%s", script.name)
                raise
            else:
                span.set_attribute("app.run.status", "ok")
                span.set_status(Status(StatusCode.OK))
            finally:
                summary.write(span)
                set_preview(span, "output", {
                    "status": span.attributes.get("app.run.status", "unknown"),
                    "model_calls": summary.llm_calls, "tool_calls": summary.tool_calls,
                    "http_calls": summary.http_calls,
                    "input_tokens": summary.input_tokens, "output_tokens": summary.output_tokens,
                    "errors": summary.errors,
                }, source="run.summary")
                logging.getLogger("lab").info(
                    "执行 %s：%s；模型 %d 次，工具 %d 次，HTTP %d 次",
                    script.name, span.attributes.get("app.run.status", "unknown"),
                    summary.llm_calls, summary.tool_calls, summary.http_calls,
                    extra={"event.name": "run.completed", "app.run.status": span.attributes.get("app.run.status", "unknown")},
                )
    finally:
        current_run.reset(summary_token)
        sys.argv, sys.path[:] = old_argv, old_path
        if old_model is None:
            os.environ.pop("OPENAI_DEFAULT_MODEL", None)
        else:
            os.environ["OPENAI_DEFAULT_MODEL"] = old_model
        flush()


def main() -> None:
    parser = argparse.ArgumentParser(description="运行 demo 并上报 OpenObserve")
    parser.add_argument("--debug-http", action="store_true", help="额外记录 HTTP 响应请求 ID（JSON 正文默认采集）")
    parser.add_argument("script", type=Path)
    parser.add_argument("args", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    script = args.script.resolve()
    if not script.is_file() or script.suffix != ".py":
        parser.error(f"不是 Python 文件：{script}")
    run_demo(script, args.args, debug_http=args.debug_http)


if __name__ == "__main__":
    main()
