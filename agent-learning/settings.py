"""仅从环境/.env 读取配置；从任何目录启动都读取同一份 .env。"""

import math
import os
from dataclasses import dataclass, field
from pathlib import Path
from urllib.parse import urlsplit

from dotenv import load_dotenv

from telemetry import parse_headers


def env_bool(name, default="true"):
    """读取严格的 true/false 环境变量，避免错误值静默改变配置。

    Args:
        name: 环境变量名。
        default: 环境变量缺失时使用的文本默认值。

    Returns:
        对应的 Python 布尔值。

    Raises:
        ValueError: 值不是 true 或 false 时。
    """
    value = os.getenv(name, default).strip().lower()
    if value not in {"true", "false"}:
        raise ValueError(f"{name} 必须是 true 或 false，不能静默关闭观测")
    return value == "true"


@dataclass
class Settings:
    """保存 DeepSeek 请求、观测端点和正文采集配置。"""

    api_key: str = field(repr=False)
    base_url: str = "https://api.deepseek.com/v1"
    model: str = "deepseek-flash"
    timeout: float = 60
    telemetry_enabled: bool = True
    service_name: str = "agent-learning"
    traces_endpoint: str = "http://localhost:5080/api/default/v1/traces"
    otel_headers: dict = field(default_factory=dict, repr=False)
    capture_content: bool = True
    content_max_chars: int = 32768
    secrets: tuple = field(default_factory=tuple, repr=False)

    @classmethod
    def from_env(cls):
        """从环境变量和本目录的 .env 文件构造配置对象。

        Returns:
            经过基础校验的 Settings 实例。

        Raises:
            ValueError: 布尔开关、观测鉴权或 URL 配置无效时。
        """
        load_dotenv(Path(__file__).with_name(".env"), override=False)
        key = os.getenv("DEEPSEEK_API_KEY", "")
        headers = parse_headers(
            os.getenv("OTEL_EXPORTER_OTLP_TRACES_HEADERS")
            or os.getenv("OTEL_EXPORTER_OTLP_HEADERS", "")
        )
        endpoint = os.getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") or (
            os.getenv(
                "OTEL_EXPORTER_OTLP_ENDPOINT",
                "http://localhost:5080/api/default",
            ).rstrip("/")
            + "/v1/traces"
        )
        enabled = env_bool("TELEMETRY_ENABLED")
        if enabled and not any(
            k.lower() == "authorization" and v for k, v in headers.items()
        ):
            raise ValueError(
                "请在 .env 配置 OpenObserve 的 OTEL_EXPORTER_OTLP_HEADERS"
            )
        return cls(
            api_key=key,
            base_url=os.getenv(
                "DEEPSEEK_BASE_URL", "https://api.deepseek.com/v1"
            ).rstrip("/"),
            model=os.getenv("DEEPSEEK_MODEL", "deepseek-flash"),
            timeout=float(os.getenv("DEEPSEEK_TIMEOUT", "60")),
            telemetry_enabled=enabled,
            service_name=os.getenv("OTEL_SERVICE_NAME", "agent-learning"),
            traces_endpoint=endpoint,
            otel_headers=headers,
            capture_content=env_bool("TELEMETRY_CAPTURE_CONTENT"),
            content_max_chars=int(
                os.getenv("TELEMETRY_CONTENT_MAX_CHARS", "32768")
            ),
            secrets=tuple(
                [key]
                + list(headers.values())
                + [v.split(" ", 1)[-1] for v in headers.values()]
            ),
        )

    def __post_init__(self):
        """验证服务 URL、超时和正文长度等配置约束。"""
        for url in (self.base_url, self.traces_endpoint):
            parsed = urlsplit(url)
            if (
                parsed.scheme not in {"http", "https"}
                or not parsed.hostname
                or parsed.username
                or parsed.password
                or parsed.query
                or parsed.fragment
            ):
                raise ValueError("服务地址应为无凭据、无 query 的 HTTP(S) URL")
        if (
            not math.isfinite(self.timeout)
            or self.timeout <= 0
            or self.content_max_chars < 256
        ):
            raise ValueError("timeout 必须大于 0，内容上限至少为 256 字符")
