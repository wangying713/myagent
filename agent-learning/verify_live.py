"""分级真实验收：检查业务结果及 OpenObserve 完整链路，会消耗模型 token。"""

import argparse
import json
import time
from pathlib import Path
from tempfile import TemporaryDirectory

from demos.advanced.advanced_01_thinking import solve
from demos.advanced.advanced_04_knowledge_agent import run_knowledge_agent
from demos.beginner.beginner_03_agent_loop import run_agent
from demos.intermediate.intermediate_01_json import parse_order
from demos.intermediate.intermediate_04_rag import answer_question
from demos.intermediate.intermediate_05_memory import ask
from llm import DeepSeekClient, final_text, message_of
from session_store import SessionStore
from settings import Settings
from verify_observe import query_trace


def require(condition, message):
    """条件不成立时抛出 AssertionError，且不受 python -O 影响。"""
    if not condition:
        raise AssertionError(message)  # 不用 assert，python -O 也不会跳过验收。


def check_trace(settings, trace_id, llm_calls, tool_calls):
    """轮询 trace 入库状态，并校验 span 数、层级、正文和凭据。

    Args:
        settings: 查询 OpenObserve 所需的 Settings 对象。
        trace_id: 本次运行的 trace ID。
        llm_calls: 预期模型请求数。
        tool_calls: 预期工具执行数。

    Returns:
        查询到的全部 span 字典。

    Raises:
        AssertionError: 入库不完整或 trace 内容不符合验收要求时。
    """
    expected = llm_calls + tool_calls + 1
    deadline = time.monotonic() + 20
    while True:
        hits = query_trace(settings, trace_id)
        if len(hits) == expected:
            break
        if time.monotonic() >= deadline:
            raise AssertionError(
                f"trace 未完整入库：{trace_id}；实际={len(hits)}，预期={expected}"
            )
        time.sleep(1)
    roots = [h for h in hits if h.get("operation_name") == "agent.run"]
    models = [
        h for h in hits if h.get("operation_name", "").startswith("chat ")
    ]
    tools = [h for h in hits if h.get("operation_name") == "execute_tool"]
    require(
        len(roots) == 1
        and len(models) == llm_calls
        and len(tools) == tool_calls,
        "span 类型或数量不完整",
    )
    root = roots[0]
    require(
        all(
            (h.get("reference_parent_span_id") or h.get("parent_span_id"))
            == root["span_id"]
            for h in hits
            if h != root
        ),
        "父子关系错误",
    )
    require(all(h.get("span_status") != "ERROR" for h in hits), "出现错误 span")
    require(settings.api_key not in json.dumps(hits), "凭据进入观测数据")
    for span in models:
        require(
            span.get("app_request") and span.get("app_response"),
            "缺少请求或响应正文",
        )
        require(
            span.get("gen_ai_usage_input_tokens") is not None, "缺少输入用量"
        )
    return hits


def exercise(name, action, check, expected_tool_result=None):
    """运行一个真实模型场景，检查业务结果和对应 trace。

    Args:
        name: 场景名称，同时用作 trace 标签。
        action: 接收客户端并执行场景的函数。
        check: 判断 action 结果是否正确的函数。
        expected_tool_result: 可选的实际工具结果期望值。

    Returns:
        含 trace、调用次数、用量和通过状态的摘要字典。
    """
    with DeepSeekClient("verify_" + name) as model:
        print(f"{name}: trace_id={model.trace_id}", flush=True)
        result = action(model)
        require(check(result), name + ": 业务结果不正确")
    hits = check_trace(
        model.settings, model.trace_id, model.calls, model.tool_calls
    )
    if expected_tool_result is not None:
        results = [
            json.loads(h["app_tool_response"])
            for h in hits
            if h.get("operation_name") == "execute_tool"
        ]
        require(
            any(
                r.get("ok") is True and r.get("result") == expected_tool_result
                for r in results
            ),
            "未找到正确的实际工具结果",
        )
    record = {
        "lesson": name,
        "trace_id": model.trace_id,
        "spans": len(hits),
        "llm_calls": model.calls,
        "tool_calls": model.tool_calls,
        "tokens": model.total_tokens,
        "passed": True,
    }
    print(json.dumps(record, ensure_ascii=False), flush=True)
    return record


def multi_turn(model):
    """执行两轮对话，验证第二次请求确实携带第一轮历史。"""
    messages = [
        {"role": "user", "content": "请记住暗号是蓝色铅笔，回复收到即可。"}
    ]
    messages.append(message_of(model.chat(messages)))
    messages.append({"role": "user", "content": "暗号是什么？只回复暗号。"})
    return final_text(model.chat(messages))


def memory_roundtrip(model):
    """跨存储对象重新读取会话，并检查另一个 session 没有串线。"""
    with TemporaryDirectory() as folder:
        path = Path(folder) / "sessions.sqlite3"
        ask(model, path, "alice", "请记住我的项目代号是海盐。")
        result = ask(model, path, "alice", "我的项目代号是什么？只回复代号。")
        require(SessionStore(path).load("bob") == ([], 0), "会话发生串线")
        return result


def main():
    """按级别执行真实 DeepSeek 场景，并逐条核对 OpenObserve 入库。"""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--level",
        choices=["beginner", "intermediate", "advanced", "all"],
        default="all",
    )
    parser.add_argument(
        "--evaluate",
        action="store_true",
        help="额外运行四个真实计算评估用例（通常再调用 8 次）",
    )
    parser.add_argument(
        "--report", type=Path, help="将本次成功验收摘要写入 JSON 文件"
    )
    args = parser.parse_args()
    settings = Settings.from_env()
    require(
        settings.telemetry_enabled and settings.capture_content,
        "真实验收需要开启观测与正文采集",
    )
    scenarios = {
        "beginner": [
            (
                "chat",
                lambda m: final_text(
                    m.chat([{"role": "user", "content": "只回复：你好"}])
                ),
                lambda r: "你好" in r,
                None,
            ),
            ("messages", multi_turn, lambda r: "蓝色铅笔" in r, None),
            (
                "agent",
                lambda m: run_agent(
                    m, "用工具计算 17 乘以 23。最终只输出数字。"
                ),
                lambda r: r.strip() == "391",
                391,
            ),
        ],
        "intermediate": [
            (
                "json",
                lambda m: parse_order(
                    final_text(
                        m.chat(
                            [
                                {
                                    "role": "system",
                                    "content": (
                                        "输出 JSON，且仅有 item 和 quantity 两个字段，"
                                        '例如 {"item":"笔","quantity":2}。'
                                    ),
                                },
                                {"role": "user", "content": "买三本笔记本。"},
                            ],
                            response_format={"type": "json_object"},
                        )
                    )
                ),
                lambda r: r == {"item": "笔记本", "quantity": 3},
                None,
            ),
            (
                "stream",
                lambda m: final_text(
                    m.chat_stream(
                        [{"role": "user", "content": "只回复：流式调用成功"}],
                        on_text=lambda t: None,
                    )
                ),
                lambda r: "流式调用成功" in r,
                None,
            ),
            (
                "rag",
                lambda m: answer_question(m, "星河笔记支持导出哪些格式？"),
                lambda r: (
                    r["source_ids"] == ["export"]
                    and "PDF" in r["answer"]
                    and "Markdown" in r["answer"]
                ),
                None,
            ),
            (
                "rag_unknown",
                lambda m: answer_question(m, "是否支持脑机接口？"),
                lambda r: r["insufficient_evidence"] and not r["source_ids"],
                None,
            ),
            ("memory", memory_roundtrip, lambda r: "海盐" in r, None),
        ],
        "advanced": [
            ("thinking", solve, lambda r: r.strip() == "74", None),
            (
                "knowledge",
                lambda m: run_knowledge_agent(
                    m, "星河笔记误删的笔记能恢复多久？请给出资料依据。"
                ),
                lambda r: (
                    r["source_ids"] == ["recovery"]
                    and "30" in r["answer"]
                    and not r["insufficient_evidence"]
                ),
                None,
            ),
        ],
    }
    report = []
    for level, exercises in scenarios.items():
        if args.level in {"all", level}:
            for scenario in exercises:
                report.append(exercise(*scenario))
    if args.evaluate:
        from demos.advanced.advanced_03_evaluation import evaluate

        for row in evaluate(live=True):
            require(row["passed"], "真实评估未通过：" + row["case"])
            check_trace(
                settings, row["trace_id"], row["llm_calls"], row["tool_calls"]
            )
            report.append(row)
            print(json.dumps(row, ensure_ascii=False), flush=True)
    if args.report:
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.write_text(
            json.dumps(report, ensure_ascii=False, indent=2) + "\n"
        )
    print("所选级别的业务结果及完整 trace 入库检查通过。")


if __name__ == "__main__":
    main()
