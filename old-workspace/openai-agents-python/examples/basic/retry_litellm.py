import asyncio
import inspect

from agents import (
    Agent,
    ModelRetrySettings,
    ModelSettings,
    RetryDecision,
    RunConfig,
    Runner,
    retry_policies,
)


def format_error(error: object) -> str:
    if not isinstance(error, BaseException):
        return "Unknown error"
    return str(error) or error.__class__.__name__


async def main() -> None:
    apply_policies = retry_policies.any(
        # 对 OpenAI 系的模型，provider_suggested() 会遵循厂商给出的重试建议，
        # 包括缺少 x-should-retry 头时默认视为可重试的状态码
        # （例如 408/409/429/5xx）。
        retry_policies.provider_suggested(),
        retry_policies.retry_after(),
        retry_policies.network_error(),
        retry_policies.http_status([408, 409, 429, 500, 502, 503, 504]),
    )

    async def policy(context) -> bool | RetryDecision:
        raw_decision = apply_policies(context)
        decision: bool | RetryDecision
        if inspect.isawaitable(raw_decision):
            decision = await raw_decision
        else:
            decision = raw_decision
        if isinstance(decision, RetryDecision):
            if not decision.retry:
                print(
                    f"[retry] 第 {context.attempt}/{context.max_retries + 1} 次尝试后停止："
                    f"{format_error(context.error)}"
                )
                return False

            print(
                " | ".join(
                    part
                    for part in [
                        f"[retry] 第 {context.attempt}/{context.max_retries + 1} 次尝试，准备重试",
                        (
                            f"等待 {decision.delay:.2f}s"
                            if decision.delay is not None
                            else "使用默认退避"
                        ),
                        f"原因：{decision.reason}" if decision.reason else None,
                        f"错误：{format_error(context.error)}",
                    ]
                    if part is not None
                )
            )
            return decision

        if not decision:
            print(
                f"[retry] 第 {context.attempt}/{context.max_retries + 1} 次尝试后停止："
                f"{format_error(context.error)}"
            )
        return decision

    retry = ModelRetrySettings(
        max_retries=4,
        backoff={
            "initial_delay": 0.5,
            "max_delay": 5.0,
            "multiplier": 2.0,
            "jitter": True,
        },
        policy=policy,
    )

    # RunConfig 级 model_settings 是整次运行的共享默认值。
    # 如果 Agent 自己也定义了 model_settings，重叠的键以 Agent 为准，
    # 而 retry/backoff 这类嵌套对象则是合并。
    run_config = RunConfig(model_settings=ModelSettings(retry=retry))

    agent = Agent(
        name="Assistant",
        instructions="你是一个简洁的助手。回答最多用 3 个短句要点。",
        # 加上 litellm/ 前缀，让这次请求走 LiteLLM 适配器。
        model="litellm/openai/gpt-4o-mini",
        # 这里为了直观，Agent 重复了一遍同样的重试配置。实际代码里，
        # 共享默认值放 RunConfig 即可，只有当某个 Agent 需要不同的重试行为时，
        # 才在这里写 per-agent 覆盖。
        model_settings=ModelSettings(retry=retry),
    )

    print(
        "重试机制已配置好。只有真的发生瞬时故障时，你才会看到 [retry] 日志。"
    )

    result = await Runner.run(
        agent,
        "用大白话解释一下 API 重试里的指数退避。",
        run_config=run_config,
    )

    print("\n最终输出：\n")
    print(result.final_output)


if __name__ == "__main__":
    asyncio.run(main())
