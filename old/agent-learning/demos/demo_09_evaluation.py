"""高级：调用真实模型运行固定用例评估，并上报 OpenObserve。"""

import argparse
import json
import time
from decimal import Decimal, InvalidOperation
from pathlib import Path

from old.agent_toolkit.agent_loop import run_agent
from old.agent_toolkit.llm import DeepSeekClient, ModelError
from demos.tools.calculator import multiply


def grade_case(case, answer, tool_results):
    """分别验证最终答案和实际工具调用证据是否符合评估用例。"""
    try:
        is_answer_correct = Decimal(answer.strip()) == Decimal(
            str(case["expected"])
        )
    except (InvalidOperation, AttributeError):
        is_answer_correct = False
    is_tool_correct = False
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
        is_tool_correct |= (
            arguments
            in (
                {"a": case["a"], "b": case["b"]},
                {"a": case["b"], "b": case["a"]},
            )
            and result.get("ok") is True
            and result.get("result") == case["expected"]
        )
    return {
        "answer_correct": is_answer_correct,
        "tool_correct": is_tool_correct,
    }


def evaluate(*, cases=None):
    """调用真实模型运行固定评估用例并上报 OpenObserve。

    Args:
        cases: 可选用例列表；省略时读取项目内的默认 JSON 用例。

    Returns:
        每个用例的答案、工具、预算和运行状态结果。
    """
    data_dir = Path(__file__).resolve().parents[1] / "data"
    cases = cases or json.loads((data_dir / "eval_cases.json").read_text())
    evaluation_results = []
    for case in cases:
        llm_client = DeepSeekClient("advanced_eval_" + case["id"])
        start_time = time.perf_counter()
        answer, error_name, error_message = "", None, None
        try:
            with llm_client:
                answer = run_agent(
                    llm_client,
                    f"请用工具计算 {case['a']} 乘以 {case['b']}，最终只输出数字。",
                    system_prompt="你是计算助手。乘法必须调用工具，最终只输出数字；"
                    "工具出错时不能捏造结果。",
                    tools=[multiply],
                )
        except Exception as exc:
            error_name = type(exc).__name__
            if isinstance(exc, ModelError):
                error_message = str(llm_client.telemetry.clean(str(exc)))[:1024]
        case_score = grade_case(case, answer, llm_client.tool_results)
        within_budget = llm_client.calls <= 5 and llm_client.tool_calls <= 8
        evaluation_results.append(
            {
                "case": case["id"],
                "mode": "live",
                **case_score,
                "budget_ok": within_budget,
                "passed": all(case_score.values())
                and within_budget
                and error_name is None,
                "error": error_name,
                "error_message": error_message,
                "llm_calls": llm_client.calls,
                "tool_calls": llm_client.tool_calls,
                "tokens": llm_client.total_tokens,
                "elapsed_ms": round((time.perf_counter() - start_time) * 1000),
                "trace_id": llm_client.trace_id,
            }
        )
    return evaluation_results


def main():
    """运行真实模型评估，打印结果并以退出码报告是否全部通过。"""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.parse_args()
    evaluation_results = evaluate()
    for evaluation_result in evaluation_results:
        print(json.dumps(evaluation_result, ensure_ascii=False))
    passed_count = sum(
        evaluation_result["passed"] for evaluation_result in evaluation_results
    )
    print(f"通过 {passed_count}/{len(evaluation_results)}；真实模型评估。")
    if passed_count != len(evaluation_results):
        raise SystemExit(1)


if __name__ == "__main__":
    main()
