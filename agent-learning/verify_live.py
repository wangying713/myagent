"""分级真实验收：检查业务结果及 OpenObserve 完整链路，会消耗模型 token。"""

import argparse
import json
import time
from pathlib import Path
from tempfile import TemporaryDirectory

from agent_toolkit.agent_loop import run_agent
from agent_toolkit.llm import DeepSeekClient, final_text, message_of
from agent_toolkit.session_store import SessionStore
from agent_toolkit.settings import Settings
from demos.advanced.advanced_01_thinking import solve
from demos.advanced.advanced_03_knowledge_agent import run_knowledge_agent
from demos.intermediate.intermediate_01_json import parse_order
from demos.intermediate.intermediate_03_rag import answer_question
from demos.intermediate.intermediate_04_memory import ask
from toolbox import multiply
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
    root_spans = [
        span for span in hits if span.get("operation_name") == "agent.run"
    ]
    model_spans = [
        span
        for span in hits
        if span.get("operation_name", "").startswith("chat ")
    ]
    tool_spans = [
        span
        for span in hits
        if span.get("gen_ai_operation_name") == "execute_tool"
    ]
    require(
        len(root_spans) == 1
        and len(model_spans) == llm_calls
        and len(tool_spans) == tool_calls,
        "span 类型或数量不完整",
    )
    root = root_spans[0]
    require(
        all(
            (span.get("reference_parent_span_id") or span.get("parent_span_id"))
            == root["span_id"]
            for span in hits
            if span != root
        ),
        "父子关系错误",
    )
    require(
        all(span.get("span_status") != "ERROR" for span in hits),
        "出现错误 span",
    )
    require(settings.api_key not in json.dumps(hits), "凭据进入观测数据")
    for span in model_spans:
        require(
            span.get("app_request") and span.get("app_response"),
            "缺少请求或响应正文",
        )
        require(
            span.get("gen_ai_usage_input_tokens") is not None, "缺少输入用量"
        )
    for span in tool_spans:
        require(
            all(
                span.get(field)
                for field in (
                    "app_tool_request",
                    "app_tool_response",
                    "gen_ai_tool_call_arguments",
                    "gen_ai_tool_call_result",
                )
            ),
            "缺少工具正文或预览",
        )
        request = json.loads(span["app_tool_request"])
        require(
            json.loads(span["gen_ai_tool_call_arguments"])
            == json.loads(request["function"]["arguments"])
            and json.loads(span["gen_ai_tool_call_result"])
            == json.loads(span["app_tool_response"]),
            "工具预览与实际记录不一致",
        )
        require(
            span.get("app_tool_call_id") == request["id"]
            and span.get("app_tool_name") == request["function"]["name"],
            "工具调用元数据不一致",
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
    with DeepSeekClient("verify_" + name) as llm_client:
        print(f"{name}: trace_id={llm_client.trace_id}", flush=True)
        result = action(llm_client)
        require(check(result), name + ": 业务结果不正确")
    hits = check_trace(
        llm_client.settings,
        llm_client.trace_id,
        llm_client.calls,
        llm_client.tool_calls,
    )
    if expected_tool_result is not None:
        results = [
            json.loads(span["app_tool_response"])
            for span in hits
            if span.get("gen_ai_operation_name") == "execute_tool"
        ]
        require(
            any(
                result.get("ok") is True
                and result.get("result") == expected_tool_result
                for result in results
            ),
            "未找到正确的实际工具结果",
        )
    record = {
        "lesson": name,
        "trace_id": llm_client.trace_id,
        "spans": len(hits),
        "llm_calls": llm_client.calls,
        "tool_calls": llm_client.tool_calls,
        "tokens": llm_client.total_tokens,
        "passed": True,
    }
    print(json.dumps(record, ensure_ascii=False), flush=True)
    return record


def multi_turn(llm_client):
    """执行两轮对话，验证第二次请求确实携带第一轮历史。"""
    messages = [
        {"role": "user", "content": "请记住暗号是蓝色铅笔，回复收到即可。"}
    ]
    messages.append(message_of(llm_client.chat(messages)))
    messages.append({"role": "user", "content": "暗号是什么？只回复暗号。"})
    return final_text(llm_client.chat(messages))


def memory_roundtrip(llm_client):
    """跨存储对象重新读取会话，并检查另一个 session 没有串线。"""
    with TemporaryDirectory() as folder:
        path = Path(folder) / "sessions.sqlite3"
        ask(llm_client, path, "alice", "请记住我的项目代号是海盐。")
        result = ask(
            llm_client, path, "alice", "我的项目代号是什么？只回复代号。"
        )
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
                lambda llm_client: final_text(
                    llm_client.chat(
                        [{"role": "user", "content": "只回复：你好"}]
                    )
                ),
                lambda result: "你好" in result,
                None,
            ),
            ("messages", multi_turn, lambda result: "蓝色铅笔" in result, None),
            (
                "agent",
                lambda llm_client: run_agent(
                    llm_client,
                    "用工具计算 17 乘以 23。最终只输出数字。",
                    system_prompt="你是计算助手。乘法必须调用工具，最终只输出数字；"
                    "工具出错时不能捏造结果。",
                    tools=[multiply],
                ),
                lambda result: result.strip() == "391",
                391,
            ),
        ],
        "intermediate": [
            (
                "json",
                lambda llm_client: parse_order(
                    final_text(
                        llm_client.chat(
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
                lambda result: result == {"item": "笔记本", "quantity": 3},
                None,
            ),
            (
                "stream",
                lambda llm_client: final_text(
                    llm_client.chat_stream(
                        [{"role": "user", "content": "只回复：流式调用成功"}],
                        on_text=lambda text: None,
                    )
                ),
                lambda result: "流式调用成功" in result,
                None,
            ),
            (
                "rag",
                lambda llm_client: answer_question(
                    llm_client, "星河笔记支持导出哪些格式？"
                ),
                lambda result: (
                    result["source_ids"] == ["export"]
                    and "PDF" in result["answer"]
                    and "Markdown" in result["answer"]
                ),
                None,
            ),
            (
                "rag_unknown",
                lambda llm_client: answer_question(
                    llm_client, "是否支持脑机接口？"
                ),
                lambda result: (
                    result["insufficient_evidence"] and not result["source_ids"]
                ),
                None,
            ),
            ("memory", memory_roundtrip, lambda result: "海盐" in result, None),
        ],
        "advanced": [
            ("thinking", solve, lambda result: result.strip() == "74", None),
            (
                "knowledge",
                lambda llm_client: run_knowledge_agent(
                    llm_client, "星河笔记误删的笔记能恢复多久？请给出资料依据。"
                ),
                lambda result: (
                    result["source_ids"] == ["recovery"]
                    and "30" in result["answer"]
                    and not result["insufficient_evidence"]
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
        from demos.advanced.advanced_02_evaluation import evaluate

        for evaluation_result in evaluate():
            require(
                evaluation_result["passed"],
                "真实评估未通过：" + evaluation_result["case"],
            )
            check_trace(
                settings,
                evaluation_result["trace_id"],
                evaluation_result["llm_calls"],
                evaluation_result["tool_calls"],
            )
            report.append(evaluation_result)
            print(json.dumps(evaluation_result, ensure_ascii=False), flush=True)
    if args.report:
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.write_text(
            json.dumps(report, ensure_ascii=False, indent=2) + "\n"
        )
    print("所选级别的业务结果及完整 trace 入库检查通过。")


if __name__ == "__main__":
    main()
