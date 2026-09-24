"""只导出脱敏后的快照，目标为本地控制台，不发起任何 API 调用。"""

from __future__ import annotations

import json
import logging
from collections.abc import Callable
from copy import deepcopy
from typing import Any

from agents import custom_span, set_trace_processors, trace
from agents.tracing import Span, Trace
from agents.tracing.processor_interface import TracingExporter
from agents.tracing.processors import BatchTraceProcessor

logger = logging.getLogger(__name__)


class RedactingExporter(TracingExporter):
    """把「脱敏」和「投递」都收在应用自己拥有的同一个 exporter 里。

    两个回调都是可信的应用代码。脱敏函数拿到的是 payload 的私有副本；
    投递目标只会收到脱敏成功的字典。
    应当「替换掉」默认 processor，而不是在 exporter 旁边再挂一个脱敏器。
    本示例不配置向 OpenAI 后端上报。
    """

    def __init__(
        self,
        redact: Callable[[dict[str, Any]], dict[str, Any]],
        send: Callable[[list[dict[str, Any]]], None],
    ) -> None:
        self._redact = redact
        self._send = send

    def export(self, items: list[Trace | Span[Any]]) -> None:
        try:
            redacted = []
            for item in items:
                payload = item.export()
                if payload is not None:
                    redacted.append(self._redact(deepcopy(payload)))
        except Exception:
            pass
        else:
            # 整批全部脱敏完成后，才调用投递目标。
            if redacted:
                self._send(redacted)
            return

        # 打日志前先脱离敏感的异常上下文：否则格式化器一旦失败，
        # 会通过异常链把原始 payload 打出来。
        logger.warning("链路脱敏失败；已丢弃该批次。")


def redact_payload(payload: dict[str, Any]) -> dict[str, Any]:
    """一个用于本地诊断的示例白名单，不是后端的入库 schema。

    只保留关联事件所需的事件类别与各类 ID。所有名称、
    元数据、错误和 span 数据一律省略。应用必须保证
    调用方提供的 trace/span/parent ID 不含敏感信息，
    或者在使用投递目标前换成自己的 ID 映射方案。
    """
    return {
        key: payload[key] for key in ("object", "id", "trace_id", "parent_id") if key in payload
    }


def main() -> None:
    exporter = RedactingExporter(redact_payload, lambda batch: print(json.dumps(batch)))
    processor = BatchTraceProcessor(exporter)
    # 必须用「替换」：add_trace_processor 会把默认 exporter 一起留下来。
    set_trace_processors([processor])
    try:
        with trace("示例私有工作流", metadata={"customer": "synthetic-customer"}):
            with custom_span("示例私有操作", data={"message": "synthetic-secret"}):
                pass
    finally:
        # shutdown 会把队列里的数据经由同一条脱敏边界排干。
        processor.shutdown()


if __name__ == "__main__":
    main()
