"""Offline regression checks for context, duplicate logs and actual SDK integration."""
import asyncio
import io
import logging
import sys

import pytest
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import SimpleSpanProcessor
from opentelemetry.sdk.trace.export.in_memory_span_exporter import InMemorySpanExporter
from opentelemetry.sdk._logs import LoggerProvider
from opentelemetry.instrumentation.logging.handler import LoggingHandler
from opentelemetry.sdk._logs.export import SimpleLogRecordProcessor, InMemoryLogRecordExporter

from myagent.runtime.telemetry import DemoSpanProcessor, DemoLogFilter, RunSummary, capture_console, current_run
from myagent.runtime.http_telemetry import instrument_http
from myagent.runtime.span_export import DisplaySpanExporter


@pytest.fixture
def recording(monkeypatch):
    provider = TracerProvider()
    spans = InMemorySpanExporter()
    provider.add_span_processor(DemoSpanProcessor())
    provider.add_span_processor(SimpleSpanProcessor(DisplaySpanExporter(spans)))
    monkeypatch.setattr(trace, 'get_tracer', provider.get_tracer)
    logs = InMemoryLogRecordExporter()
    lp = LoggerProvider()
    lp.add_log_record_processor(SimpleLogRecordProcessor(logs))
    handler = LoggingHandler(logger_provider=lp)
    handler.addFilter(DemoLogFilter())
    root = logging.getLogger()
    monkeypatch.setattr(root, 'handlers', [handler])
    monkeypatch.setattr(root, 'level', logging.INFO)
    token = current_run.set(RunSummary('offline.py', model='test-model'))
    yield provider.get_tracer('test'), spans, logs
    current_run.reset(token)
    provider.shutdown()
    lp.shutdown()


def test_print_context_and_logging_not_recaptured(recording, monkeypatch):
    tracer, spans, logs = recording
    monkeypatch.setattr(sys, 'stdout', io.StringIO())
    with capture_console():
        # A handler writing to the intercepted terminal must not create a second record.
        console = logging.StreamHandler(sys.stdout)
        logging.getLogger().addHandler(console)
        with tracer.start_as_current_span('first') as first:
            print('first')
            logging.info('once')
        with tracer.start_as_current_span('second') as second:
            print('second')
        logging.getLogger().removeHandler(console)
    records = [x.log_record for x in logs.get_finished_logs()]
    assert [r.body for r in records] == ['first', 'once', 'second']
    assert [r.span_id for r in records] == [first.context.span_id, first.context.span_id, second.context.span_id]


def test_partial_prints_from_concurrent_tasks(recording, monkeypatch):
    tracer, spans, logs = recording
    monkeypatch.setattr(sys, 'stdout', io.StringIO())
    async def worker(name):
        with tracer.start_as_current_span(name):
            print(name, end='')
            await asyncio.sleep(0)
            print(' done')
    async def main():
        await asyncio.gather(worker('a'), worker('b'))
    with capture_console():
        asyncio.run(main())
    records = [x.log_record for x in logs.get_finished_logs()]
    ids = {s.name: s.context.span_id for s in spans.get_finished_spans()}
    assert {(r.body, r.span_id) for r in records} == {('a done', ids['a']), ('b done', ids['b'])}


@pytest.mark.parametrize('module_name', ['httpx', 'httpx2'])
@pytest.mark.parametrize('asynchronous', [False, True])
def test_http_relationship_error_and_stream(recording, monkeypatch, module_name, asynchronous):
    import importlib
    module = importlib.import_module(module_name)
    for cls in (module.Client, module.AsyncClient):
        monkeypatch.setattr(cls, 'send', cls.send)
    instrument_http(debug_http=True)
    tracer, spans, logs = recording
    def respond(request):
        logging.info('inside request')
        return module.Response(503, content=b'body', headers={'x-request-id': 'test-request'})
    async def async_call():
        async with module.AsyncClient(transport=module.MockTransport(respond)) as client:
            response = await client.send(client.build_request('GET', 'https://example.test/path?token=secret'), stream=True)
            assert await response.aread() == b'body'
            await response.aclose()
    with tracer.start_as_current_span('model') as parent:
        if asynchronous:
            asyncio.run(async_call())
        else:
            with module.Client(transport=module.MockTransport(respond)) as client:
                response = client.send(client.build_request('GET', 'https://example.test/path?token=secret'), stream=True)
                assert response.read() == b'body'
                response.close()
    http = [s for s in spans.get_finished_spans() if s.kind.name == 'CLIENT']
    assert len(http) == 1
    span = http[0]
    assert span.parent.span_id == parent.context.span_id
    assert span.status.status_code.name == 'ERROR'
    assert span.attributes['url.full'] == 'https://example.test/path'
    assert span.attributes['app.http.duration_scope'] == 'response_headers'
    assert 'app.http.request.body' not in span.attributes
    assert [r.log_record.span_id for r in logs.get_finished_logs()] == [span.context.span_id]


def test_actual_agents_tool_print_context(recording, monkeypatch):
    from agents import Agent, Runner, function_tool, set_trace_processors
    from agents.testing import ScriptedModel, assistant_message, function_call
    from openinference.instrumentation.openai_agents import OpenAIAgentsInstrumentor
    tracer, spans, logs = recording
    set_trace_processors([])
    instrumentor = OpenAIAgentsInstrumentor()
    from myagent.runtime.observability import _setup_agent_instrumentation
    _setup_agent_instrumentation()
    # Instrumentation gets the patched trace.get_tracer, so it shares our memory exporter.
    @function_tool
    def picture() -> str:
        """Return an example picture URL."""
        print('tool output')
        return 'https://example.test/image.png'
    model = ScriptedModel([
        [function_call('picture', '{}', call_id='call_test')],
        [assistant_message('done')],
    ], emit_traces=True)
    monkeypatch.setattr(sys, 'stdout', io.StringIO())
    try:
        with tracer.start_as_current_span('demo'):
            with capture_console():
                result = asyncio.run(Runner.run(Agent(name='Assistant', model=model, tools=[picture]), 'picture'))
                print(result.final_output)
        finished = spans.get_finished_spans()
        tool, = [s for s in finished if s.attributes.get('openinference.span.kind') == 'TOOL']
        llm = [s for s in finished if s.attributes.get('openinference.span.kind') == 'LLM']
        assert len(llm) == 2
        assert not any(s.name in ('Agent 工作流', '任务执行', 'Agent 轮次') for s in finished)
        agent, = [s for s in finished if s.name == 'Assistant']
        root, = [s for s in finished if s.name == 'demo']
        assert agent.parent.span_id == root.context.span_id
        assert all(s.parent.span_id == agent.context.span_id for s in [tool, *llm])
        assert all(r.log_record.span_id in {s.context.span_id for s in finished} for r in logs.get_finished_logs())
        assert tool.name.startswith('工具 1')
        assert len({s.context.trace_id for s in finished}) == 1
        record, = [r.log_record for r in logs.get_finished_logs() if r.log_record.body == 'tool output']
        assert record.span_id == tool.context.span_id
    finally:
        instrumentor.uninstrument()
        set_trace_processors([])


def test_env_only_observability_config(tmp_path, monkeypatch):
    from myagent.runtime.observability import load_observability_settings
    monkeypatch.setenv('OPENOBSERVE_USER', 'test')
    monkeypatch.setenv('OPENOBSERVE_PASSWORD', 'test-password')
    monkeypatch.setenv('OPENOBSERVE_ENDPOINT', 'http://example.test/api/test/v1/traces')
    monkeypatch.setenv('OTEL_SERVICE_NAME', 'test-service')
    settings = load_observability_settings(tmp_path / 'missing.env')
    assert settings.has_openobserve
    assert settings.service_name == 'test-service'
    assert settings.oo_logs_endpoint == 'http://example.test/api/test/v1/logs'


def test_http_cancellation_and_export_suppression(recording, monkeypatch):
    import httpx2
    from opentelemetry.instrumentation.utils import suppress_http_instrumentation
    tracer, spans, logs = recording
    for cls in (httpx2.Client, httpx2.AsyncClient):
        monkeypatch.setattr(cls, 'send', cls.send)
    instrument_http()
    async def fail(request):
        raise asyncio.CancelledError()
    async def main():
        async with httpx2.AsyncClient(transport=httpx2.MockTransport(fail)) as client:
            with pytest.raises(asyncio.CancelledError):
                await client.get('https://example.test/cancel')
    with tracer.start_as_current_span('parent'):
        asyncio.run(main())
        with suppress_http_instrumentation():
            with httpx2.Client(transport=httpx2.MockTransport(lambda r: httpx2.Response(200))) as client:
                client.get('https://example.test/export')
    requests = [s for s in spans.get_finished_spans() if s.kind.name == 'CLIENT']
    assert len(requests) == 1
    assert requests[0].status.status_code.name == 'ERROR'
    assert requests[0].attributes['error.type'] == 'CancelledError'


def test_large_print_is_bounded_without_loss(recording, monkeypatch):
    tracer, spans, logs = recording
    monkeypatch.setattr(sys, 'stdout', io.StringIO())
    body = 'a' * 40000
    with tracer.start_as_current_span('large'), capture_console():
        print(body)
    records = [x.log_record for x in logs.get_finished_logs()]
    assert ''.join(r.body for r in records) == body
    assert all(len(r.body) <= 16000 for r in records)


@pytest.mark.parametrize('module_name', ['httpx', 'httpx2'])
@pytest.mark.parametrize('asynchronous', [False, True])
def test_http_json_payloads_visible_by_default(recording, monkeypatch, module_name, asynchronous):
    import importlib
    import json
    module = importlib.import_module(module_name)
    for cls in (module.Client, module.AsyncClient):
        monkeypatch.setattr(cls, 'send', cls.send)
    instrument_http()
    tracer, spans, logs = recording
    payload = {'model': 'test', 'messages': [{'role': 'user', 'content': '你好'}], 'api_key': 'secret', 'image': 'data:image/png;base64,SECRET'}
    answer = {'choices': [{'message': {'content': '模型回答'}}], 'access_token': 'secret'}
    def respond(request):
        assert json.loads(request.content) == payload
        return module.Response(200, json=answer)
    async def call():
        async with module.AsyncClient(transport=module.MockTransport(respond)) as client:
            result = await client.post('https://example.test/chat?temperature=0.5&token=secret', json=payload)
            assert result.json() == answer
    with tracer.start_as_current_span('model'):
        if asynchronous:
            asyncio.run(call())
        else:
            with module.Client(transport=module.MockTransport(respond)) as client:
                assert client.post('https://example.test/chat?temperature=0.5&token=secret', json=payload).json() == answer
    span, = [s for s in spans.get_finished_spans() if s.kind.name == 'CLIENT']
    request = json.loads(span.attributes['app.http.request.body'])
    response = json.loads(span.attributes['app.http.response.body'])
    assert request['messages'] == payload['messages']
    assert request['api_key'] == '[REDACTED]'
    assert request['image'] == '[data URI omitted]'
    assert response['choices'] == answer['choices']
    assert response['access_token'] == '[REDACTED]'
    assert span.attributes['input.value'] == span.attributes['app.http.request.body']
    assert span.attributes['output.value'] == span.attributes['app.http.response.body']
    assert span.attributes['input.mime_type'] == 'application/json'
    assert span.attributes['output.mime_type'] == 'application/json'
    assert 'openinference.span.kind' not in span.attributes
    assert json.loads(span.attributes['app.http.request.query']) == {'temperature': ['0.5'], 'token': '[REDACTED]'}
    assert not logs.get_finished_logs()


def test_body_limits_and_unconsumed_stream(recording):
    import httpx
    from myagent.runtime.http_telemetry import record_body
    tracer, spans, logs = recording
    with tracer.start_as_current_span('bodies') as span:
        record_body(span, httpx.Response(200, json={'text': 'x' * 20000}), 'response')
        assert span.attributes['app.http.response.body.truncated'] is True
        assert len(span.attributes['app.http.response.body']) == 16384
        assert span.attributes['output.value'] == span.attributes['app.http.response.body']
        assert span.attributes['output.mime_type'] == 'text/plain'
    class UnreadStream(httpx.SyncByteStream):
        def __iter__(self):
            raise AssertionError('Telemetry must not consume streams')
    with tracer.start_as_current_span('stream') as span:
        record_body(span, httpx.Response(200, stream=UnreadStream()), 'response', streaming=True)
        assert span.attributes['app.http.response.body.capture_status'] == 'skipped_streaming'
        assert 'app.http.response.body' not in span.attributes
        assert 'output.value' not in span.attributes


def test_run_summary_namespace_and_warning_count(recording):
    tracer, spans, logs = recording
    with tracer.start_as_current_span('run') as root:
        root.set_attribute('app.run.root', True)
        with tracer.start_as_current_span('llm', attributes={'openinference.span.kind': 'LLM'}) as model:
            model.set_attribute('llm.token_count.prompt', 12)
            model.set_attribute('llm.token_count.completion', 3)
            model.set_attribute('input.value', 'input remains on model')
            model.set_attribute('output.value', 'output remains on model')
        logging.warning('warning')
        logging.error('error is not a warning')
        current_run.get().write(root)
    root_span = spans.get_finished_spans()[-1]
    assert root_span.attributes['app.run.llm_calls'] == 1
    assert root_span.attributes['app.run.total_tokens'] == 15
    assert root_span.attributes['app.run.warning_log_count'] == 1
    assert root_span.attributes['app.run.error_span_count'] == 0
    assert not any(key.startswith('demo.') for s in spans.get_finished_spans() for key in s.attributes)
    assert not any('cost' in key or 'result_status' in key for key in root_span.attributes)
    assert root_span.attributes['app.preview.input.source'] == 'capture_status'
    assert 'input remains on model' not in root_span.attributes['input.value']
    assert all(r.log_record.attributes['app.run.entrypoint'] == 'offline.py' for r in logs.get_finished_logs())


def test_compact_parallel_agents_and_custom_spans(recording):
    from agents import set_trace_processors
    from agents.tracing import trace as sdk_trace, task_span, turn_span, agent_span, custom_span
    from myagent.runtime.compact_tracing import CompactTracingProcessor
    tracer, spans, logs = recording
    processor = CompactTracingProcessor(tracer)
    set_trace_processors([processor])
    async def worker(name):
        with sdk_trace(name), task_span(name), agent_span(name), turn_span(1, name):
            # A custom operation named like a wrapper must survive.
            with custom_span('turn'):
                await asyncio.sleep(0)
                logging.info(name)
    async def main():
        await asyncio.gather(worker('Agent A'), worker('Agent B'))
    try:
        with tracer.start_as_current_span('root'):
            asyncio.run(main())
        finished = spans.get_finished_spans()
        assert len(finished) == 5
        root, = [s for s in finished if s.name == 'root']
        agents = {s.context.span_id: s for s in finished if s.name in ('Agent A', 'Agent B')}
        assert len(agents) == 2
        assert all(s.parent.span_id == root.context.span_id for s in agents.values())
        customs = {s.context.span_id: s for s in finished if s.attributes.get('openinference.span.kind') == 'CHAIN'}
        assert all(s.name == 'turn' for s in customs.values())
        for record in logs.get_finished_logs():
            custom = customs[record.log_record.span_id]
            assert agents[custom.parent.span_id].name == record.log_record.body
        assert not processor._root_spans and not processor._otel_spans
        assert not processor._wrappers and not processor._compact_traces
    finally:
        set_trace_processors([])


def test_compact_wrapper_error_is_retained(recording):
    from agents import set_trace_processors
    from agents.tracing import trace as sdk_trace, task_span
    from myagent.runtime.compact_tracing import CompactTracingProcessor
    tracer, spans, logs = recording
    set_trace_processors([CompactTracingProcessor(tracer)])
    try:
        with tracer.start_as_current_span('root'):
            with sdk_trace('workflow'), task_span('task') as task:
                logging.info('inside wrapper')
                task.set_error({'message': 'synthetic failure', 'data': {}})
        root, = spans.get_finished_spans()
        assert root.status.status_code.name == 'ERROR'
        assert root.events[0].name == 'sdk.wrapper.error'
        assert logs.get_finished_logs()[0].log_record.span_id == root.context.span_id
    finally:
        set_trace_processors([])


def test_request_body_does_not_duplicate_model_parameters(recording):
    import json
    import httpx
    from myagent.runtime.http_telemetry import record_body
    tracer, spans, logs = recording
    payload = {'model': 'example', 'stream': False, 'temperature': 0,
               'messages': [{'role': 'user', 'content': 'hello'}],
               'tools': [{'type': 'function', 'function': {'name': 'lookup'}}]}
    with tracer.start_as_current_span('request') as span:
        record_body(span, httpx.Request('POST', 'https://example.test', json=payload), 'request')
        assert json.loads(span.attributes['app.http.request.body']) == payload
        assert not any(key.endswith(('.model', '.stream', '.temperature', '.count', '.names'))
                       for key in span.attributes)
    assert not logs.get_finished_logs()


@pytest.mark.parametrize('retry', [False, True])
def test_real_sdk_http_tool_causality(recording, monkeypatch, retry):
    """Real SDK parsing/dispatch, parallel same-name tools, mocked network only."""
    import json
    import httpx2
    from openai import AsyncOpenAI
    from agents import Agent, Runner, OpenAIChatCompletionsModel, function_tool, set_trace_processors
    from openinference.instrumentation.openai_agents import OpenAIAgentsInstrumentor
    from myagent.runtime.observability import _setup_agent_instrumentation
    tracer, spans, logs = recording
    for cls in (httpx2.Client, httpx2.AsyncClient):
        monkeypatch.setattr(cls, 'send', cls.send)
    instrument_http()
    _setup_agent_instrumentation()
    requests = []

    @function_tool
    async def picture(number: int) -> str:
        """Return a picture label."""
        await asyncio.sleep(0)
        logging.info('picture %d', number)
        return f'picture-{number}'

    def respond(request):
        payload = json.loads(request.content)
        requests.append(payload)
        if retry and len(requests) == 1:
            return httpx2.Response(503, json={'error': {'message': 'retry', 'type': 'server_error'}})
        returned = [m for m in payload['messages'] if m.get('role') == 'tool']
        if returned:
            assert {m['tool_call_id'] for m in returned} == {'call_a', 'call_b'}
            message = {'role': 'assistant', 'content': 'done'}
            reason = 'stop'
        else:
            message = {'role': 'assistant', 'content': None, 'tool_calls': [
                {'id': call_id, 'type': 'function', 'function': {'name': 'picture', 'arguments': json.dumps({'number': n})}}
                for n, call_id in enumerate(['call_a', 'call_b'])
            ]}
            reason = 'tool_calls'
        return httpx2.Response(200, json={
            'id': 'chatcmpl-test', 'object': 'chat.completion', 'created': 1, 'model': 'test-model',
            'choices': [{'index': 0, 'message': message, 'finish_reason': reason}],
            'usage': {'prompt_tokens': 10, 'completion_tokens': 5, 'total_tokens': 15},
        })

    async def main():
        async with AsyncOpenAI(api_key='offline', base_url='https://example.test/v1', max_retries=1,
                               http_client=httpx2.AsyncClient(transport=httpx2.MockTransport(respond))) as client:
            result = await Runner.run(Agent(name='Assistant', model=OpenAIChatCompletionsModel(
                model='test-model', openai_client=client), tools=[picture]), 'pictures')
            assert result.final_output == 'done'
    try:
        with tracer.start_as_current_span('run') as root:
            root.set_attribute('app.run.root', True)
            asyncio.run(main())
        finished = spans.get_finished_spans()
        http = sorted((s for s in finished if s.kind.name == 'CLIENT'), key=lambda s: s.start_time)
        tools = [s for s in finished if s.attributes.get('openinference.span.kind') == 'TOOL']
        assert len(http) == 2 + retry
        assert len(tools) == 2
        assert not any(s.attributes.get('openinference.span.kind') == 'LLM' for s in finished)
        agent, = [s for s in finished if s.name == 'Assistant']
        assert all(s.parent.span_id == agent.context.span_id for s in [*http, *tools])
        assert json.loads(agent.attributes['input.value'])['messages'] == [{'role': 'user', 'content': 'pictures'}]
        assert agent.attributes['output.value'] == 'done'
        assert agent.attributes['app.preview.output.source'] == 'agent_end.final_output'
        source = http[-2]
        assert {s.attributes['app.tool.call_id'] for s in tools} == {'call_a', 'call_b'}
        assert all(s.attributes['app.tool.executor'] == 'Runner' for s in tools)
        assert all(s.links[0].context.span_id == source.context.span_id for s in tools)
        assert {s.name for s in tools} == {'工具 1 [picture]', '工具 2 [picture]'}
        assert {link.context.span_id for link in http[-1].links} == {s.context.span_id for s in tools}
        assert all(s.name == 'HTTP POST /v1/chat/completions' for s in http)
        assert json.loads(source.attributes['output.value'])['choices'][0]['message']['tool_calls']
        assert sum(s.attributes.get('llm.token_count.total', 0) for s in http) == 30
        if retry:
            assert http[0].status.status_code.name == 'ERROR'
            assert http[0].attributes['app.http.attempt'] == 1
            assert http[1].attributes['app.http.attempt'] == 2
            assert 'llm.token_count.total' not in http[0].attributes
        assert all(r.log_record.span_id in {s.context.span_id for s in finished} for r in logs.get_finished_logs())
    finally:
        OpenAIAgentsInstrumentor().uninstrument()
        set_trace_processors([])


def test_model_failure_before_http_is_retained(recording):
    tracer, spans, logs = recording
    with tracer.start_as_current_span('generation', attributes={'openinference.span.kind': 'LLM'}) as model:
        model.set_status(trace.Status(trace.StatusCode.ERROR, 'invalid request'))
    span, = spans.get_finished_spans()
    assert span.context.span_id == model.context.span_id
    assert span.status.status_code.name == 'ERROR'
    assert '无可合并 HTTP' in span.name


@pytest.mark.parametrize('mode', ['async', 'sync', 'streamed'])
def test_agent_preview_lifecycle_modes_and_user_hooks(recording, mode):
    import json
    from agents import Agent, Runner, RunHooks, set_trace_processors
    from agents.testing import ScriptedModel, assistant_message
    from openinference.instrumentation.openai_agents import OpenAIAgentsInstrumentor
    from myagent.runtime.observability import _setup_agent_instrumentation
    tracer, spans, logs = recording
    _setup_agent_instrumentation()
    events = []
    class UserHooks(RunHooks):
        async def on_agent_start(self, context, agent):
            events.append('start')
        async def on_agent_end(self, context, agent, output):
            events.append(('end', output))
        async def on_llm_start(self, *args):
            events.append('llm')
    agent = Agent(name='Preview', model=ScriptedModel([[assistant_message('真实回答')]], emit_traces=True))
    async def run():
        if mode == 'streamed':
            result = Runner.run_streamed(agent, '真实输入', hooks=UserHooks())
            async for _ in result.stream_events():
                pass
            return result
        return await Runner.run(agent, '真实输入', hooks=UserHooks())
    try:
        with tracer.start_as_current_span('root'):
            if mode == 'sync':
                # Runner.run_sync needs a usable event loop after asyncio.run tests.
                loop = asyncio.new_event_loop()
                asyncio.set_event_loop(loop)
                try:
                    result = Runner.run_sync(agent, '真实输入', hooks=UserHooks())
                finally:
                    loop.close()
                    asyncio.set_event_loop(None)
            else:
                result = asyncio.run(run())
        assert result.final_output == '真实回答'
        span, = [s for s in spans.get_finished_spans() if s.attributes.get('openinference.span.kind') == 'AGENT']
        assert json.loads(span.attributes['input.value'])['messages'] == [{'role': 'user', 'content': '真实输入'}]
        assert span.attributes['output.value'] == '真实回答'
        assert events == ['start', 'llm', ('end', '真实回答')]
    finally:
        OpenAIAgentsInstrumentor().uninstrument()
        set_trace_processors([])


def test_agent_tool_final_output_and_empty_preview(recording):
    from agents import Agent, Runner, function_tool, set_trace_processors
    from agents.testing import ScriptedModel, function_call
    from openinference.instrumentation.openai_agents import OpenAIAgentsInstrumentor
    from myagent.runtime.observability import _setup_agent_instrumentation
    tracer, spans, logs = recording
    _setup_agent_instrumentation()
    @function_tool
    def empty() -> str:
        """Return an intentionally empty result."""
        return ''
    agent = Agent(name='Empty', tools=[empty], tool_use_behavior='stop_on_first_tool',
                  model=ScriptedModel([[function_call('empty', '{}', call_id='empty_call')]], emit_traces=True))
    try:
        with tracer.start_as_current_span('root'):
            result = asyncio.run(Runner.run(agent, 'empty'))
        assert result.final_output == ''
        agent_span, = [s for s in spans.get_finished_spans() if s.attributes.get('openinference.span.kind') == 'AGENT']
        assert agent_span.attributes['output.value'] == '""'
        assert agent_span.attributes['app.preview.output.source'] == 'agent_end.final_output'
        tool, = [s for s in spans.get_finished_spans() if s.attributes.get('openinference.span.kind') == 'TOOL']
        assert tool.attributes['input.value'] == '{}'
        assert tool.attributes['output.value'] == '""'
    finally:
        OpenAIAgentsInstrumentor().uninstrument()
        set_trace_processors([])


def test_preview_redaction_structured_and_missing_values(recording):
    import json
    from pydantic import BaseModel
    from myagent.runtime.preview import preview_attributes
    tracer, spans, logs = recording
    class Answer(BaseModel):
        answer: str
        api_key: str
    attrs = preview_attributes('output', Answer(answer='正确', api_key='secret'), source='test')
    assert json.loads(attrs['output.value']) == {'answer': '正确', 'api_key': '[REDACTED]'}
    attrs = preview_attributes('output', {'large': 'x' * 20000}, source='test')
    assert len(attrs['output.value']) <= 16384
    assert attrs['output.mime_type'] == 'text/plain'
    assert attrs['app.preview.output.truncated'] is True
    with tracer.start_as_current_span('failed', attributes={'openinference.span.kind': 'AGENT'}) as span:
        span.set_status(trace.Status(trace.StatusCode.ERROR, 'failure'))
    with tracer.start_as_current_span('empty', attributes={
        'http.request.method': 'GET', 'app.http.request.body.capture_status': 'empty',
        'app.http.response.body.capture_status': 'skipped_streaming',
    }):
        pass
    failed, empty = spans.get_finished_spans()
    assert '执行失败' in failed.attributes['output.value']
    assert 'HTTP 正文为空' in empty.attributes['input.value']
    assert '流式正文未直接采集' in empty.attributes['output.value']


def test_streaming_http_uses_sdk_assembled_preview(recording):
    import json
    import httpx
    from myagent.runtime.http_telemetry import request_span, finish_response
    tracer, spans, logs = recording
    with tracer.start_as_current_span('generation', attributes={'openinference.span.kind': 'LLM'}) as model:
        with request_span(httpx.Request('POST', 'https://example.test/chat', json={'messages': []}), stream=True, debug_http=False) as http:
            finish_response(http, httpx.Response(200, headers={'content-type': 'text/event-stream'}), debug_http=False, streaming=True)
        model.set_attribute('output.value', json.dumps([{'role': 'assistant', 'content': 'assembled'}]))
        model.set_attribute('output.mime_type', 'application/json')
    span, = spans.get_finished_spans()
    assert span.attributes['app.http.response.body.capture_status'] == 'skipped_streaming'
    assert span.attributes['app.preview.output.source'] == 'sdk.assembled'
    assert json.loads(span.attributes['output.value'])[0]['content'] == 'assembled'


@pytest.mark.parametrize('capture', [False, True])
def test_agent_structured_output_and_capture_setting(recording, capture):
    import json
    from pydantic import BaseModel
    from agents import Agent, Runner, RunConfig, set_trace_processors
    from agents.testing import ScriptedModel, assistant_message
    from openinference.instrumentation.openai_agents import OpenAIAgentsInstrumentor
    from myagent.runtime.observability import _setup_agent_instrumentation
    tracer, spans, logs = recording
    class Answer(BaseModel):
        answer: str
    _setup_agent_instrumentation()
    try:
        agent = Agent(name='Structured', output_type=Answer,
                      model=ScriptedModel([[assistant_message('{"answer":"structured answer"}')]], emit_traces=True))
        with tracer.start_as_current_span('root'):
            result = asyncio.run(Runner.run(agent, 'structured input', run_config=RunConfig(trace_include_sensitive_data=capture)))
        assert result.final_output.answer == 'structured answer'
        span, = [s for s in spans.get_finished_spans() if s.attributes.get('openinference.span.kind') == 'AGENT']
        if capture:
            assert json.loads(span.attributes['output.value']) == {'answer': 'structured answer'}
            assert 'structured input' in span.attributes['input.value']
        else:
            assert 'structured input' not in span.attributes['input.value']
            assert 'structured answer' not in span.attributes['output.value']
            assert '关闭' in span.attributes['output.value']
    finally:
        OpenAIAgentsInstrumentor().uninstrument()
        set_trace_processors([])


def test_agent_handoff_preview(recording):
    import json
    from agents import Agent, Runner, set_trace_processors
    from agents.testing import ScriptedModel, assistant_message, function_call
    from openinference.instrumentation.openai_agents import OpenAIAgentsInstrumentor
    from myagent.runtime.observability import _setup_agent_instrumentation
    tracer, spans, logs = recording
    _setup_agent_instrumentation()
    target = Agent(name='Specialist', model=ScriptedModel([[assistant_message('finished')]], emit_traces=True))
    first = Agent(name='Router', handoffs=[target], model=ScriptedModel([
        [function_call('transfer_to_specialist', '{}', call_id='handoff_1')]], emit_traces=True))
    try:
        with tracer.start_as_current_span('root'):
            result = asyncio.run(Runner.run(first, 'help'))
        assert result.final_output == 'finished'
        agents = {s.name: s for s in spans.get_finished_spans() if s.attributes.get('openinference.span.kind') == 'AGENT'}
        assert json.loads(agents['Router'].attributes['output.value']) == {'handoff_to': 'Specialist'}
        assert agents['Specialist'].attributes['output.value'] == 'finished'
        assert all(s.attributes['app.preview.input.source'] == 'agent_start.configuration+first_llm_start' for s in agents.values())
    finally:
        OpenAIAgentsInstrumentor().uninstrument()
        set_trace_processors([])


def test_agent_complete_preview_uses_resolved_prompt_once(recording):
    import json
    from agents import Agent, Runner, ModelSettings, function_tool, set_trace_processors
    from agents.testing import ScriptedModel, assistant_message
    from openinference.instrumentation.openai_agents import OpenAIAgentsInstrumentor
    from myagent.runtime.observability import _setup_agent_instrumentation
    tracer, spans, logs = recording
    _setup_agent_instrumentation()
    calls = []
    async def instructions(context, agent):
        calls.append('resolved')
        return '系统提示词 Bearer synthetic-secret'
    @function_tool
    def lookup(city: str) -> str:
        """Look up the given city."""
        return city
    agent = Agent(name='Complete', instructions=instructions, tools=[lookup],
                  model_settings=ModelSettings(temperature=0),
                  model=ScriptedModel([[assistant_message('done')]], emit_traces=True))
    try:
        with tracer.start_as_current_span('root'):
            asyncio.run(Runner.run(agent, 'hello'))
        span, = [s for s in spans.get_finished_spans() if s.name == 'Complete']
        data = json.loads(span.attributes['input.value'])
        assert calls == ['resolved']
        assert data['instructions'] == '系统提示词 [REDACTED]'
        assert data['instructions_source'] == 'first_llm_start.system_prompt'
        assert data['messages'] == [{'role': 'user', 'content': 'hello'}]
        assert data['first_model_messages'] == data['messages']
        tool, = data['configured_tools']
        assert tool['name'] == 'lookup'
        assert tool['description'] == 'Look up the given city.'
        assert tool['parameters']['properties']['city']['type'] == 'string'
        assert data['configured_model_settings']['temperature'] == 0
    finally:
        OpenAIAgentsInstrumentor().uninstrument()
        set_trace_processors([])
