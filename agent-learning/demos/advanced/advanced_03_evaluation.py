"""高级：固定用例评估。默认离线验证执行链；--live 才评估真实模型，消耗 token。"""

import argparse
import json
import re
import time
from decimal import Decimal, InvalidOperation
from pathlib import Path

import httpx
from opentelemetry.sdk.trace.export import SpanExporter
from opentelemetry.sdk.trace.export.in_memory_span_exporter import (
    InMemorySpanExporter,
)

from demos.beginner.beginner_03_agent_loop import run_agent
from llm import DeepSeekClient
from offline_lab import completion, offline_client
from settings import Settings
from telemetry import OpenObserveExporter, Telemetry


class TeeExporter(SpanExporter):
    """真实模式既发送 OpenObserve，也在内存留一份供评估检查工具结果。"""

    def __init__(self, remote, memory):
        """组合远程 exporter 和内存 exporter，供真实评估同时上报和检查。"""
        self.remote, self.memory = remote, memory

    def export(self, spans):
        """将同一批 span 同时交给两个 exporter。"""
        self.memory.export(spans)
        return self.remote.export(spans)

    def shutdown(self):
        """关闭两个 exporter 持有的资源。"""
        self.memory.shutdown()
        self.remote.shutdown()


def fixture(request):
    """根据请求消息构造确定性的假模型响应，用于完全离线评估。"""
    messages = json.loads(request.content)["messages"]
    if messages[-1]["role"] == "tool":
        result = json.loads(messages[-1]["content"])
        return httpx.Response(
            200, json=completion(str(result.get("result", "工具出错")))
        )
    question = next(
        m["content"] for m in reversed(messages) if m["role"] == "user"
    )
    a, b = map(int, re.findall(r"-?\d+", question))
    return httpx.Response(
        200,
        json=completion(
            calls=[
                {
                    "id": "eval-call",
                    "type": "function",
                    "function": {
                        "name": "multiply",
                        "arguments": json.dumps({"a": a, "b": b}),
                    },
                }
            ]
        ),
    )


def grade(case, answer, spans):
    """分别验证最终答案和实际工具调用证据是否符合评估用例。"""
    try:
        answer_ok = Decimal(answer.strip()) == Decimal(str(case["expected"]))
    except (InvalidOperation, AttributeError):
        answer_ok = False
    tool_ok = False
    for span in spans:
        if span.name != "execute_tool":
            continue
        request = json.loads(span.attributes.get("app.tool.request", "{}"))
        result = json.loads(span.attributes.get("app.tool.response", "{}"))
        arguments = json.loads(
            request.get("function", {}).get("arguments", "{}")
        )
        tool_ok |= (
            request.get("function", {}).get("name") == "multiply"
            and arguments
            in (
                {"a": case["a"], "b": case["b"]},
                {"a": case["b"], "b": case["a"]},
            )
            and result.get("ok") is True
            and result.get("result") == case["expected"]
        )
    return {"answer_correct": answer_ok, "tool_correct": tool_ok}


def evaluate(*, live=False, cases=None):
    """运行固定评估用例，可选调用真实模型并上报 OpenObserve。

    Args:
        live: 为 True 时调用 DeepSeek；否则使用离线假模型。
        cases: 可选用例列表；省略时读取项目内的默认 JSON 用例。

    Returns:
        每个用例的答案、工具、预算和运行状态结果。

    Raises:
        ValueError: 真实评估未开启所需观测配置时。
    """
    data_dir = Path(__file__).resolve().parents[2] / "data"
    cases = cases or json.loads((data_dir / "eval_cases.json").read_text())
    rows = []
    for case in cases:
        if live:
            settings = Settings.from_env()
            if not settings.telemetry_enabled or not settings.capture_content:
                raise ValueError(
                    "真实评估需要开启观测与正文采集，以验证工具行为"
                )
            memory = InMemorySpanExporter()
            exporter = TeeExporter(
                OpenObserveExporter(
                    settings.traces_endpoint, settings.otel_headers
                ),
                memory,
            )
            model = DeepSeekClient(
                "advanced_eval_" + case["id"],
                settings=settings,
                telemetry=Telemetry(settings, exporter=exporter),
            )
        else:
            model, memory = offline_client(
                "offline_eval_" + case["id"], fixture
            )
        started = time.perf_counter()
        answer, error = "", None
        try:
            with model:
                answer = run_agent(
                    model,
                    f"请用工具计算 {case['a']} 乘以 {case['b']}，最终只输出数字。",
                )
        except Exception as exc:
            error = type(exc).__name__
        score = grade(case, answer, memory.get_finished_spans())
        budget_ok = model.calls <= 5 and model.tool_calls <= 8
        rows.append(
            {
                "case": case["id"],
                "mode": "live" if live else "offline",
                **score,
                "budget_ok": budget_ok,
                "passed": all(score.values()) and budget_ok and error is None,
                "error": error,
                "llm_calls": model.calls,
                "tool_calls": model.tool_calls,
                "tokens": model.total_tokens,
                "elapsed_ms": round((time.perf_counter() - started) * 1000),
                "trace_id": model.trace_id if live else "memory-only",
            }
        )
    return rows


def main():
    """解析命令行参数并运行离线或真实模型评估。"""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--live", action="store_true")
    args = parser.parse_args()
    rows = evaluate(live=args.live)
    for row in rows:
        print(json.dumps(row, ensure_ascii=False))
    passed = sum(r["passed"] for r in rows)
    print(
        f"通过 {passed}/{len(rows)}；"
        + (
            "真实模型评估。"
            if args.live
            else "离线契约评估，不代表模型真实准确率；token 为固定模拟值。"
        )
    )
    if passed != len(rows):
        raise SystemExit(1)


if __name__ == "__main__":
    main()
