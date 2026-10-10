"""高级：调用真实模型运行固定用例评估，并上报 OpenObserve。"""

import argparse
import json
import time
from decimal import Decimal, InvalidOperation
from pathlib import Path

from demos.beginner.beginner_03_agent_loop import run_agent
from llm import DeepSeekClient


def grade(case, answer, tool_results):
    """分别验证最终答案和实际工具调用证据是否符合评估用例。"""
    try:
        answer_ok = Decimal(answer.strip()) == Decimal(str(case["expected"]))
    except (InvalidOperation, AttributeError):
        answer_ok = False
    tool_ok = False
    for execution in tool_results:
        request = execution["request"]
        result = execution["response"]
        function = request.get("function")
        if not isinstance(function, dict) or function.get("name") != "multiply":
            continue
        try:
            arguments = json.loads(function.get("arguments", "{}"))
        except (ValueError, TypeError):
            continue
        tool_ok |= (
            arguments
            in (
                {"a": case["a"], "b": case["b"]},
                {"a": case["b"], "b": case["a"]},
            )
            and result.get("ok") is True
            and result.get("result") == case["expected"]
        )
    return {"answer_correct": answer_ok, "tool_correct": tool_ok}


def evaluate(*, cases=None):
    """调用真实模型运行固定评估用例并上报 OpenObserve。

    Args:
        cases: 可选用例列表；省略时读取项目内的默认 JSON 用例。

    Returns:
        每个用例的答案、工具、预算和运行状态结果。
    """
    data_dir = Path(__file__).resolve().parents[2] / "data"
    cases = cases or json.loads((data_dir / "eval_cases.json").read_text())
    rows = []
    for case in cases:
        model = DeepSeekClient("advanced_eval_" + case["id"])
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
        score = grade(case, answer, model.tool_results)
        budget_ok = model.calls <= 5 and model.tool_calls <= 8
        rows.append(
            {
                "case": case["id"],
                "mode": "live",
                **score,
                "budget_ok": budget_ok,
                "passed": all(score.values()) and budget_ok and error is None,
                "error": error,
                "llm_calls": model.calls,
                "tool_calls": model.tool_calls,
                "tokens": model.total_tokens,
                "elapsed_ms": round((time.perf_counter() - started) * 1000),
                "trace_id": model.trace_id,
            }
        )
    return rows


def main():
    """运行真实模型评估，打印结果并以退出码报告是否全部通过。"""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.parse_args()
    rows = evaluate()
    for row in rows:
        print(json.dumps(row, ensure_ascii=False))
    passed = sum(r["passed"] for r in rows)
    print(f"通过 {passed}/{len(rows)}；真实模型评估。")
    if passed != len(rows):
        raise SystemExit(1)


if __name__ == "__main__":
    main()
