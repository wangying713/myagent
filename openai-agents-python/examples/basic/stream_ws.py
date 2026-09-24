"""Responses WebSocket 流式示例：包含函数工具、Agent 工具化，以及人工审批。

本示例展示一个面向用户的 WebSocket 工作流，使用
`responses_websocket_session(...)`：
- 流式输出（有 reasoning 摘要增量时也会一并输出）
- 普通函数工具
- 一个通过 `Agent.as_tool(...)` 包装的专家 Agent
- 针对敏感工具调用的 HITL 人工审批
- 在同一条 trace 上用 `previous_response_id` 追问一轮

必需的环境变量：
- `OPENAI_API_KEY`

可选的环境变量：
- `OPENAI_MODEL`（默认 `gpt-5.6-sol`）
- `OPENAI_BASE_URL`
- `OPENAI_WEBSOCKET_BASE_URL`
- `EXAMPLES_INTERACTIVE_MODE=auto`（为脚本化运行自动批准 HITL 提示）
"""

import asyncio
import os
from typing import Any

from openai.types.shared import Reasoning

from agents import (
    Agent,
    ModelSettings,
    ResponsesWebSocketSession,
    responses_websocket_session,
    trace,
)
from agents.decorators import tool
from examples.auto_mode import confirm_with_fallback


@tool
def lookup_order(order_id: str) -> dict[str, Any]:
    """返回演示用的确定性订单数据。"""
    orders = {
        "ORD-1001": {
            "order_id": "ORD-1001",
            "status": "delivered",
            "delivered_days_ago": 3,
            "amount": 49.99,
            "currency": "USD",
            "item": "Wireless Mouse",
        },
        "ORD-2002": {
            "order_id": "ORD-2002",
            "status": "delivered",
            "delivered_days_ago": 12,
            "amount": 129.0,
            "currency": "USD",
            "item": "Keyboard",
        },
    }
    return orders.get(
        order_id,
        {
            "order_id": order_id,
            "status": "unknown",
            "delivered_days_ago": 999,
            "amount": 0.0,
            "currency": "USD",
            "item": "unknown",
        },
    )


@tool(needs_approval=True)
def submit_refund(order_id: str, amount: float, reason: str) -> dict[str, Any]:
    """创建退款申请。该工具需要人工审批。"""
    ticket = "RF-1001" if order_id == "ORD-1001" else f"RF-{order_id[-4:]}"
    return {
        "refund_ticket": ticket,
        "order_id": order_id,
        "amount": amount,
        "reason": reason,
        "status": "approved_pending_processing",
    }


def ask_approval(question: str) -> bool:
    """请求人工审批（在示例的自动模式下会自动批准）。"""
    return confirm_with_fallback(f"[审批] {question} [y/N]: ", default=True)


async def run_streamed_turn(
    ws: ResponsesWebSocketSession,
    agent: Agent[Any],
    prompt: str,
    *,
    previous_response_id: str | None = None,
) -> tuple[str, str]:
    """跑一轮流式对话，需要时处理 HITL 人工审批。"""
    print(f"\n用户：{prompt}\n")

    result = ws.run_streamed(
        agent,
        prompt,
        previous_response_id=previous_response_id,
    )
    printed_reasoning = False
    printed_output = False

    while True:
        async for event in result.stream_events():
            if event.type == "raw_response_event":
                raw = event.data
                if raw.type == "response.reasoning_summary_text.delta":
                    if not printed_reasoning:
                        print("推理过程：")
                        printed_reasoning = True
                    print(raw.delta, end="", flush=True)
                elif raw.type == "response.output_text.delta":
                    if printed_reasoning and not printed_output:
                        print("\n")
                    if not printed_output:
                        print("助手：")
                        printed_output = True
                    print(raw.delta, end="", flush=True)
                continue

            if event.type != "run_item_stream_event":
                continue

            item = event.item
            if item.type == "tool_call_item":
                tool_name = getattr(item.raw_item, "name", "unknown")
                tool_args = getattr(item.raw_item, "arguments", "")
                print(f"\n[工具调用] {tool_name}({tool_args})")
            elif item.type == "tool_call_output_item":
                print(f"[工具结果] {item.output}")

        if printed_reasoning or printed_output:
            print("\n")

        if not result.interruptions:
            break

        state = result.to_state()
        for interruption in result.interruptions:
            question = f"批准 {interruption.name}（参数 {interruption.arguments}）吗？"
            if ask_approval(question):
                state.approve(interruption)
            else:
                state.reject(interruption)

        result = ws.run_streamed(agent, state)

    if result.last_response_id is None:
        raise RuntimeError("这次流式运行结束时没有拿到 response_id。")

    final_output = str(result.final_output)
    print(f"response_id: {result.last_response_id}")
    print(f"final_output: {final_output}\n")
    return result.last_response_id, final_output


async def main() -> None:
    model_name = os.getenv("OPENAI_MODEL", "gpt-5.6-sol")
    policy_agent = Agent(
        name="RefundPolicySpecialist",
        instructions=(
            "你是退款政策专家。政策很简单：7 天内送达的订单"
            "可以全额退款，送达时间更早的订单则不行。"
            "请简短回答：是否符合资格，并用一句话说明理由。"
        ),
        model=model_name,
        model_settings=ModelSettings(max_tokens=120),
    )

    support_agent = Agent(
        name="SupportAgent",
        instructions=(
            "你是一个客服 Agent。处理退款请求时，按这个顺序做："
            "1) 调用 lookup_order，2) 调用 refund_policy_specialist，"
            "3) 如果用户要求继续、且订单符合资格，调用 submit_refund。"
            "当用户只要退款单号时，只返回单号本身"
            "（例如 RF-1001）。"
        ),
        tools=[
            lookup_order,
            policy_agent.as_tool(
                tool_name="refund_policy_specialist",
                tool_description="检查退款资格并说明政策判定依据。",
            ),
            submit_refund,
        ],
        model=model_name,
        model_settings=ModelSettings(
            max_tokens=200,
            reasoning=Reasoning(effort="medium", summary="detailed"),
        ),
    )

    try:
        # 你可以跳过这个 helper，直接调 Runner.run_streamed(...)。
        # 那样也能跑，只是每次运行都会重新创建/建立连接，除非你手动
        # 复用同一个 RunConfig/provider。这个 helper 就是为了让这种复用
        # 在跨轮次（以及嵌套 agent-as-tool 运行）时更简单，让 WebSocket 连接保持热着。
        async with responses_websocket_session() as ws:
            with trace("Responses WebSocket 客服示例") as current_trace:
                print(f"使用模型 model={model_name}")
                print(f"trace_id={current_trace.trace_id}")

                first_response_id, _ = await run_streamed_turn(
                    ws,
                    support_agent,
                    (
                        "客户要求为订单 ORD-1001 退款，因为鼠标到货时已损坏。"
                        "请先查订单，再询问退款政策专家，"
                        "如果符合资格就提交退款。只需回复退款单号。"
                    ),
                )

                await run_streamed_turn(
                    ws,
                    support_agent,
                    "你刚创建的是哪个退款单号？只需回复单号。",
                    previous_response_id=first_response_id,
                )
    except RuntimeError as exc:
        if "closed before any response events" in str(exc):
            print(
                "\nWebSocket 模式在发送事件之前就关闭了。这通常意味着"
                "该功能尚未对当前账号/模型开放。"
            )
            return
        raise


if __name__ == "__main__":
    asyncio.run(main())
