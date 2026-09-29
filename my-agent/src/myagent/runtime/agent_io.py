"""Capture real Agent lifecycle I/O while preserving user-provided SDK hooks."""
from functools import wraps

from agents import RunHooks
from agents.lifecycle import RunHooksBase
from opentelemetry import trace

from .preview import set_preview


class PreviewHooks(RunHooks):
    def __init__(self, delegate, *, capture):
        self.delegate = delegate
        self.capture = capture
        self.agents = {}
        self.inputs = {}
        self.resolved = set()

    async def on_agent_start(self, context, agent):
        span = trace.get_current_span()
        self.agents[id(agent)] = span
        if self.capture:
            tools = []
            for tool in agent.tools:
                item = {"name": getattr(tool, "name", type(tool).__name__), "type": type(tool).__name__}
                for attr, key in (("description", "description"), ("params_json_schema", "parameters"),
                                  ("strict_json_schema", "strict")):
                    value = getattr(tool, attr, None)
                    if value is not None:
                        item[key] = value
                enabled = getattr(tool, "is_enabled", True)
                item["enabled"] = enabled if isinstance(enabled, bool) else "dynamic; see actual HTTP tools"
                tools.append(item)
            snapshot = {
                "scope": "Agent 开始时的消息和配置；实际生效参数及每轮上下文请查看对应 HTTP 请求",
                "agent": agent.name,
                "instructions": agent.instructions if isinstance(agent.instructions, str) or agent.instructions is None else "动态提示词，等待 SDK 求值",
                "messages": context.turn_input,
                "configured_tools": tools,
                "handoffs": [getattr(h, "name", getattr(h, "tool_name", type(h).__name__)) for h in agent.handoffs],
                "configured_model": agent.model if isinstance(agent.model, str) else getattr(agent.model, "model", None),
                "configured_model_settings": agent.model_settings.to_traceable_dict(),
                "mcp_servers": [getattr(server, "name", type(server).__name__) for server in agent.mcp_servers],
            }
            self.inputs[id(agent)] = snapshot
            self.resolved.discard(id(agent))
            set_preview(span, "input", snapshot, source="agent_start.configuration")
        else:
            set_preview(span, "input", {"说明": "本次运行已关闭输入输出内容采集"}, source="capture_status")
        await self.delegate.on_agent_start(context, agent)

    async def on_llm_start(self, context, agent, system_prompt, input_items):
        # Reuse the SDK-resolved prompt; never invoke a dynamic prompt or tool
        # discovery a second time just for telemetry.
        key = id(agent)
        if self.capture and key in self.inputs and key not in self.resolved:
            self.resolved.add(key)
            snapshot = self.inputs[key]
            snapshot["instructions"] = system_prompt
            snapshot["instructions_source"] = "first_llm_start.system_prompt"
            snapshot["first_model_messages"] = input_items
            set_preview(self.agents[key], "input", snapshot, source="agent_start.configuration+first_llm_start")
        await self.delegate.on_llm_start(context, agent, system_prompt, input_items)

    async def on_agent_end(self, context, agent, output):
        span = self.agents.get(id(agent), trace.get_current_span())
        if self.capture:
            set_preview(span, "output", output, source="agent_end.final_output")
        else:
            set_preview(span, "output", {"说明": "本次运行已关闭输入输出内容采集"}, source="capture_status")
        await self.delegate.on_agent_end(context, agent, output)

    async def on_handoff(self, context, from_agent, to_agent):
        span = self.agents.get(id(from_agent))
        if span is not None:
            set_preview(span, "output", {"handoff_to": to_agent.name}, source="agent.handoff")
        await self.delegate.on_handoff(context, from_agent, to_agent)


def _forward(name):
    async def forwarded(self, *args, **kwargs):
        return await getattr(self.delegate, name)(*args, **kwargs)
    return forwarded


for _name in dir(RunHooksBase):
    if _name.startswith("on_") and _name not in PreviewHooks.__dict__:
        setattr(PreviewHooks, _name, _forward(_name))


def instrument_agent_io():
    # AgentRunner.run_sync delegates to run, and streamed execution has its own
    # entry point. Wrap these two methods once; do not mutate agent/user hooks.
    from agents.run import AgentRunner
    from .telemetry import current_run

    def with_hooks(kwargs):
        if current_run.get() is None:
            return kwargs
        hooks = kwargs.get("hooks")
        if isinstance(hooks, PreviewHooks):
            return kwargs
        # Invalid hooks still go through the SDK's normal validation/error path.
        if hooks is not None and not isinstance(hooks, RunHooksBase):
            return kwargs
        config = kwargs.get("run_config")
        capture = config.get("trace_include_sensitive_data", True) if isinstance(config, dict) else getattr(config, "trace_include_sensitive_data", True)
        return {**kwargs, "hooks": PreviewHooks(hooks if hooks is not None else RunHooks(), capture=capture)}

    def wrap(original, asynchronous):
        if asynchronous:
            @wraps(original)
            async def wrapped(self, *args, **kwargs):
                return await original(self, *args, **with_hooks(kwargs))
        else:
            @wraps(original)
            def wrapped(self, *args, **kwargs):
                return original(self, *args, **with_hooks(kwargs))
        wrapped._myagent_preview = True
        return wrapped

    for name, asynchronous in (("run", True), ("run_streamed", False)):
        original = getattr(AgentRunner, name)
        if not getattr(original, "_myagent_preview", False):
            setattr(AgentRunner, name, wrap(original, asynchronous))
