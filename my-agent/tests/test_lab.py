"""统一入口保留脚本语义，错误也要有完整 trace。"""
import sys
from types import SimpleNamespace

import pytest
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import SimpleSpanProcessor
from opentelemetry.sdk.trace.export.in_memory_span_exporter import InMemorySpanExporter

from myagent.runtime import lab


@pytest.fixture
def recording(monkeypatch):
    provider = TracerProvider()
    exporter = InMemorySpanExporter()
    provider.add_span_processor(SimpleSpanProcessor(exporter))
    monkeypatch.setattr(lab, 'setup', lambda **kwargs: None)
    monkeypatch.setattr(lab, "load_settings", lambda **kwargs: SimpleNamespace(api_key=""))
    monkeypatch.setattr(lab, 'configure', lambda *args, **kwargs: None)
    monkeypatch.setattr(trace, 'get_tracer', provider.get_tracer)
    flushed = []
    monkeypatch.setattr(lab, 'flush', lambda: flushed.append(True))
    return exporter, flushed


@pytest.mark.parametrize('ending,status,exception', [
    ('', 'ok', None),
    ('raise SystemExit(0)', 'ok', SystemExit),
    ('raise SystemExit(3)', 'error', SystemExit),
    ('raise RuntimeError("demo failure")', 'error', RuntimeError),
])
def test_lifecycle(tmp_path, recording, ending, status, exception):
    script = tmp_path / 'example.py'
    script.write_text('import sys\nassert __name__ == "__main__"\nassert sys.argv[1:] == ["--step", "3"]\n'+ending)
    before_argv, before_path = sys.argv[:], sys.path[:]
    if exception:
        with pytest.raises(exception):
            lab.run_demo(script, ['--step', '3'])
    else:
        lab.run_demo(script, ['--step', '3'])
    exporter, flushed = recording
    span, = exporter.get_finished_spans()
    assert span.name == 'example.py'
    assert span.attributes['app.run.status'] == status
    assert span.status.status_code.name == ('ERROR' if status == 'error' else 'OK')
    import json
    assert json.loads(span.attributes['input.value']) == {'script': 'example.py', 'arguments': ['--step', '3']}
    assert json.loads(span.attributes['output.value'])['status'] == status
    assert flushed == [True]
    assert sys.argv == before_argv
    assert sys.path == before_path
