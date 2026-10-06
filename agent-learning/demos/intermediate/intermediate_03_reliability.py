"""中级：用可重现的 503/401 故障学习有限重试；完全离线。"""

import time

import httpx

from llm import ModelError, final_text
from offline_lab import completion, offline_client


def retry_chat(model, messages, *, max_attempts=3, sleep=time.sleep):
    """只对指定的临时 HTTP 错误进行有限重试。

    Args:
        model: 提供 chat 方法的模型客户端。
        messages: 传给模型的消息列表。
        max_attempts: 包含首次请求在内的最大尝试次数。
        sleep: 等待函数；测试时可替换为不等待的函数。

    Returns:
        成功请求返回的模型响应字典。

    Raises:
        ValueError: 重试次数不在 1 到 5 之间时。
        ModelError: 请求失败且错误类型不可重试，或重试次数已用尽时。
    """
    if type(max_attempts) is not int or not 1 <= max_attempts <= 5:
        raise ValueError("max_attempts 必须为 1–5")
    for attempt in range(max_attempts):
        try:
            return model.chat(messages)
        except ModelError as exc:
            # 示例仅重试明确的临时 HTTP 故障，不重试工具或所有异常。
            if (
                exc.status_code not in {429, 500, 502, 503, 504}
                or attempt == max_attempts - 1
            ):
                raise
            sleep(min(0.1 * 2**attempt, 1.0))


def main():
    """用模拟的 503 和 401 响应演示重试与不重试的区别。"""
    attempts = 0

    def transient(request):
        """模拟第一次返回 503、之后恢复成功的 HTTP handler。"""
        nonlocal attempts
        attempts += 1
        return (
            httpx.Response(503, json={"error": "temporary"})
            if attempts == 1
            else httpx.Response(200, json=completion("恢复成功"))
        )

    client, exporter = offline_client("intermediate_retry", transient)
    with client as model:
        print(
            final_text(retry_chat(model, [{"role": "user", "content": "你好"}]))
        )
    print(
        "503 场景：请求次数=",
        client.calls,
        "内存 span 数=",
        len(exporter.get_finished_spans()),
    )
    client, exporter = offline_client(
        "intermediate_no_retry",
        lambda req: httpx.Response(401, json={"error": "unauthorized"}),
    )
    try:
        with client as model:
            retry_chat(model, [])
    except ModelError as exc:
        print(
            "401 场景：明确失败，HTTP=",
            exc.status_code,
            "请求次数=",
            client.calls,
        )
    print(
        "本实验为模拟响应；没有调用 DeepSeek，没有向 OpenObserve 写测试噪音。"
    )


if __name__ == "__main__":
    main()
