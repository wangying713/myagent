"""验证入库，不调用模型。也可按示例打印的 trace_id 查询完整调用链。"""

import argparse
import json
import re
import time

import httpx

from settings import Settings
from telemetry import Telemetry


def query_trace(settings, trace_id):
    """按 trace ID 查询 OpenObserve 中的 span。

    Args:
        settings: 含查询端点和鉴权 header 的 Settings 对象。
        trace_id: 32 位十六进制 ID，或用于查最新数据的 latest。

    Returns:
        OpenObserve 返回的 span 字典列表。

    Raises:
        ValueError: trace ID、stream 名或端点格式无效时。
        httpx.HTTPStatusError: OpenObserve 返回 HTTP 错误时。
        httpx.RequestError: 查询请求无法完成时。
    """
    if trace_id != "latest" and not re.fullmatch(r"[0-9a-f]{32}", trace_id):
        raise ValueError("trace_id 必须是 32 位小写十六进制")
    stream = next(
        (
            v
            for k, v in settings.otel_headers.items()
            if k.lower() == "stream-name"
        ),
        "default",
    )
    if not re.fullmatch(r"[A-Za-z0-9_-]+", stream):
        raise ValueError("不支持的 stream 名")
    if not settings.traces_endpoint.endswith("/v1/traces"):
        raise ValueError("查询脚本需要以 /v1/traces 结尾的 OpenObserve 端点")
    url = (
        settings.traces_endpoint.removesuffix("/v1/traces")
        + "/_search?type=traces"
    )
    now = int(time.time() * 1_000_000)
    service = settings.service_name.replace("'", "''")
    where = (
        f"service_name = '{service}'"
        if trace_id == "latest"
        else f"trace_id = '{trace_id}'"
    )
    with httpx.Client(timeout=10, trust_env=False) as client:
        response = client.post(
            url,
            headers=settings.otel_headers,
            json={
                "query": {
                    "sql": (
                        f'SELECT * FROM "{stream}" WHERE {where} '
                        "ORDER BY start_time DESC"
                    ),
                    "start_time": now - 86_400_000_000,
                    "end_time": now + 1_000_000,
                    "from": 0,
                    "size": 100,
                },
            },
        )
        response.raise_for_status()
        result = response.json()
        if result.get("error"):
            raise RuntimeError("OpenObserve 查询返回错误")
        return result.get("hits", [])


def main():
    """执行观测连通性检查，或按参数查询已有 trace。"""
    parser = argparse.ArgumentParser(description=__doc__)
    source = parser.add_mutually_exclusive_group()
    source.add_argument("--trace-id")
    source.add_argument(
        "--latest",
        action="store_true",
        help="只查询当前服务最近的记录，不创建 span",
    )
    parser.add_argument(
        "--fields",
        action="store_true",
        help="显示入库字段名以排查版本差异，不打印正文",
    )
    parser.add_argument(
        "--expect", type=int, default=1, help="至少应查到的 span 数"
    )
    args = parser.parse_args()
    if args.expect < 1:
        parser.error("--expect 必须大于 0")
    settings = Settings.from_env()
    if not settings.telemetry_enabled:
        raise SystemExit("TELEMETRY_ENABLED=false，请先启用上报")
    trace_id = "latest" if args.latest else args.trace_id
    if trace_id is None:
        telemetry = Telemetry(settings)
        with telemetry.span(
            "observability.check", attributes={"app.lesson": "00_observe"}
        ) as span:
            trace_id = format(span.get_span_context().trace_id, "032x")
        telemetry.close()
        if telemetry.exporter.failed_spans:
            raise SystemExit("上报未成功；请检查服务、鉴权和端点")
    deadline = time.monotonic() + 20
    hits = []
    while time.monotonic() < deadline:
        try:
            hits = query_trace(settings, trace_id)
        except httpx.HTTPStatusError as exc:
            raise SystemExit(
                f"查询失败：HTTP {exc.response.status_code}"
            ) from None
        except httpx.RequestError as exc:
            raise SystemExit(
                f"无法连接 OpenObserve：{type(exc).__name__}"
            ) from None
        if len(hits) >= args.expect:
            break
        time.sleep(1)
    if len(hits) < args.expect:
        raise SystemExit(
            f"未查到预期记录：trace_id={trace_id}，实际={len(hits)}，预期至少={args.expect}"
        )
    print(
        json.dumps(
            {
                "trace_id": trace_id,
                "span_count": len(hits),
                "spans": [
                    {
                        key: hit.get(key)
                        for key in (
                            "operation_name",
                            "trace_id",
                            "span_id",
                            "reference_parent_span_id",
                            "service_name",
                            "span_status",
                        )
                    }
                    for hit in hits
                ],
            },
            ensure_ascii=False,
            indent=2,
        )
    )
    if args.fields:
        print("fields:", sorted({key for hit in hits for key in hit}))


if __name__ == "__main__":
    main()
