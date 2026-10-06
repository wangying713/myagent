# OpenAI Agents SDK 官方示例：学习顺序与 OpenObserve 操作手册

> 目标：按由浅入深的顺序过一遍本机 `../openai-agents-python/examples/` 中的官方示例，并让每次本地 Agent SDK 执行都通过项目统一入口送到 OpenObserve。此清单根据当前 checkout 中有 `__main__` 入口的示例生成；官方仓库更新后请重新核对。

## 0. 先跑本项目的两个观测锚点

先从我们自己的示例熟悉 OpenObserve 的树形结构和工具参数展示：

```bash
cd /Users/wangying/apps/sakelei/ai/my-agent
uv run python -m myagent.runtime.lab experiments/hello_world_traced.py
uv run python -m myagent.runtime.lab experiments/image_tool_traced.py
```

第二个示例中检查工具 span：

- `input.value`：模型实际传给函数的 JSON 参数，例如 `{"subject":"伦敦"}`。
- `output.value`：函数实际返回的结构化结果。
- 前一个模型 HTTP 响应中的 tool call 是模型发起调用的原始内容；后一个模型 HTTP 请求可确认工具结果如何回传。

先确认这两条 trace 和日志在 OpenObserve 可查，再进入官方示例。

## 1. 官方示例怎么接入 OpenObserve

不要直接运行官方文件，也不要改官方示例源码。通过仓库内的适配入口运行它们：

```bash
uv run python -m myagent.runtime.lab experiments/run_official.py examples.basic.hello_world
```

适配器会以 Python module 方式运行官方示例，保留包内相对导入；外层 `lab` 负责初始化 OpenTelemetry、OpenInference、HTTP 和日志采集，并在结束时刷新。每个示例单独运行一次，避免不同练习挤在一条 trace。

命令格式：

```bash
uv run python -m myagent.runtime.lab experiments/run_official.py <官方模块名> [官方示例参数…]
```

### 从清单运行一项

1. 打开终端，进入项目目录：

   ```bash
   cd /Users/wangying/apps/sakelei/ai/my-agent
   ```

2. 在下面的清单里选一项，把反引号里的模块名复制到命令末尾。例如，`examples.basic.hello_world` 这一项运行：

   ```bash
   uv run python -m myagent.runtime.lab experiments/run_official.py examples.basic.hello_world
   ```

3. 等程序结束，复制终端打印的 `trace_id`，到 OpenObserve 的 Traces 里按服务 `my-agent`、运行时间和 trace ID 查找。根 span 的 `app.run.target` 会显示模块名。
4. 在 OpenObserve 展开 Agent、模型、工具和 HTTP 节点，再通过 `View Logs` 查看关联日志。完成后勾选该项，再运行下一项。

清单里的文件路径直接把 `/` 换成 `.`，去掉 `.py` 就是模块名：

| 文件路径 | 命令中的模块名 |
|---|---|
| `examples/basic/hello_world.py` | `examples.basic.hello_world` |
| `examples/agent_patterns/routing.py` | `examples.agent_patterns.routing` |
| `examples/mcp/filesystem_example/main.py` | `examples.mcp.filesystem_example.main` |

官方示例需要命令行参数时，先查看该文件的 `argparse` 或 README，再把参数放在模块名后面。下面这条可以查看 `prompt_template` 的参数说明，不会实际调用模型：

```bash
uv run python -m myagent.runtime.lab experiments/run_official.py examples.basic.prompt_template --help
```

`--help` 会由官方示例自己的参数解析器处理。若示例不支持 `--help`，就从源码确认参数后再传。无额外参数的示例不需要添加任何内容。每次只运行一个模块；不要把多个模块名放在同一条命令中。

### 配置与观测

沿用 `my-agent/config.env` 中的模型和 OpenObserve 配置：

- 模型：`OPENAI_API_KEY`、`OPENAI_BASE_URL`、`MODEL`。
- Traces：`OPENOBSERVE_ENDPOINT`、`OPENOBSERVE_USER`、`OPENOBSERVE_PASSWORD`。
- Logs 可用 `OPENOBSERVE_LOGS_ENDPOINT` 单独指定；默认由 traces endpoint 推导。
- OpenObserve 中选 `my-agent` 服务和本次时间范围。适配器根 span 名会是 `run_official.py`；在根 span 的 `app.run.target` 或 `app.run.arguments` 查正在练习的官方模块。展开看 Agent、模型、工具、HTTP；通过 `trace_id` / `span_id` 看关联日志。

大多数 Agent SDK 调用会自动出现。需要外部服务、厂商 API、MCP server、数据库、sandbox 或额外 Python 包的示例，还要先按对应目录的 README 配置依赖和服务。跨进程/远程服务自己的内部执行不会因为客户端 trace 自动变成同一条 OpenObserve trace；这里能保证的是本地运行入口和本地 SDK/HTTP 调用被观测。

## 2. 每个示例的练习方法

每次勾选前完成四件事：

1. 阅读该文件及所在目录的 README，记下它在演示的 SDK 概念和前置条件。
2. 用上面的统一入口单独运行；若依赖尚未配置，先记录阻塞原因，不要把缺少服务误判成 trace 故障。
3. 用终端打印的 trace ID 在 OpenObserve 找到根 span，确认 `app.run.target` 对应当前模块。
4. 看调用树、输入输出、HTTP 请求和关联日志，记下一个“预期结构”和一个实际观察到的差异。

建议的学习记录：

| 日期 | 官方模块 | 概念 | OpenObserve 中看到的 span | 结论/问题 |
|---|---|---|---|---|
|  |  |  |  |  |

## 3. 学习清单（按顺序）

### 1. 基础与输入输出

- [ ] `examples.basic.hello_world`
- [ ] `examples.basic.tools`
- [ ] `examples.basic.image_tool_output`
- [ ] `examples.basic.agent_lifecycle_example`
- [ ] `examples.basic.lifecycle_example`
- [ ] `examples.basic.dynamic_system_prompt`
- [ ] `examples.basic.prompt_template`
- [ ] `examples.basic.non_strict_output_type`
- [ ] `examples.basic.previous_response_id`
- [ ] `examples.basic.local_file`
- [ ] `examples.basic.local_image`
- [ ] `examples.basic.remote_image`
- [ ] `examples.basic.remote_pdf`
- [ ] `examples.basic.tool_guardrails`
- [ ] `examples.basic.stream_text`
- [ ] `examples.basic.stream_items`
- [ ] `examples.basic.stream_function_call_args`
- [ ] `examples.basic.stream_ws`
- [ ] `examples.basic.retry`
- [ ] `examples.basic.retry_litellm`
- [ ] `examples.basic.usage_tracking`
- [ ] `examples.basic.trace_redaction`
- [ ] `examples.basic.hello_world_gpt_5`
- [ ] `examples.basic.hello_world_gpt_oss`

### 2. 工具调用、路由与 Agent 编排

- [ ] `examples.agent_patterns.agents_as_tools`
- [ ] `examples.agent_patterns.agents_as_tools_conditional`
- [ ] `examples.agent_patterns.agents_as_tools_streaming`
- [ ] `examples.agent_patterns.agents_as_tools_structured`
- [ ] `examples.agent_patterns.deterministic`
- [ ] `examples.agent_patterns.forcing_tool_use`
- [ ] `examples.agent_patterns.hosted_multi_agent_beta`
- [ ] `examples.agent_patterns.human_in_the_loop`
- [ ] `examples.agent_patterns.human_in_the_loop_custom_rejection`
- [ ] `examples.agent_patterns.human_in_the_loop_server`
- [ ] `examples.agent_patterns.human_in_the_loop_stream`
- [ ] `examples.agent_patterns.input_guardrails`
- [ ] `examples.agent_patterns.llm_as_a_judge`
- [ ] `examples.agent_patterns.output_guardrails`
- [ ] `examples.agent_patterns.parallelization`
- [ ] `examples.agent_patterns.routing`
- [ ] `examples.agent_patterns.streaming_guardrails`
- [ ] `examples.handoffs.message_filter`
- [ ] `examples.handoffs.message_filter_streaming`

### 3. 内建工具与远程工具

- [ ] `examples.hosted_mcp.connectors`
- [ ] `examples.hosted_mcp.human_in_the_loop`
- [ ] `examples.hosted_mcp.on_approval`
- [ ] `examples.hosted_mcp.simple`
- [ ] `examples.mcp.filesystem_example.main`
- [ ] `examples.mcp.get_all_mcp_tools_example.main`
- [ ] `examples.mcp.git_example.main`
- [ ] `examples.mcp.manager_example.app`
- [ ] `examples.mcp.manager_example.mcp_server`
- [ ] `examples.mcp.manager_example.smoke_test`
- [ ] `examples.mcp.prompt_server.main`
- [ ] `examples.mcp.prompt_server.server` — 服务端/准备入口，先看目录 README；通常不产生 Agent 调用树
- [ ] `examples.mcp.sse_example.main`
- [ ] `examples.mcp.sse_example.server` — 服务端/准备入口，先看目录 README；通常不产生 Agent 调用树
- [ ] `examples.mcp.sse_remote_example.main`
- [ ] `examples.mcp.streamable_http_remote_example.main`
- [ ] `examples.mcp.streamablehttp_custom_client_example.main`
- [ ] `examples.mcp.streamablehttp_custom_client_example.server` — 服务端/准备入口，先看目录 README；通常不产生 Agent 调用树
- [ ] `examples.mcp.streamablehttp_example.main`
- [ ] `examples.mcp.streamablehttp_example.server` — 服务端/准备入口，先看目录 README；通常不产生 Agent 调用树
- [ ] `examples.mcp.tool_filter_example.main`
- [ ] `examples.tools.apply_patch`
- [ ] `examples.tools.code_interpreter`
- [ ] `examples.tools.codex`
- [ ] `examples.tools.codex_same_thread`
- [ ] `examples.tools.computer_use`
- [ ] `examples.tools.container_shell_inline_skill`
- [ ] `examples.tools.container_shell_skill_reference`
- [ ] `examples.tools.file_search`
- [ ] `examples.tools.image_generator`
- [ ] `examples.tools.local_shell_skill`
- [ ] `examples.tools.programmatic_tool_calling`
- [ ] `examples.tools.shell`
- [ ] `examples.tools.shell_human_in_the_loop`
- [ ] `examples.tools.tool_search`
- [ ] `examples.tools.web_search`
- [ ] `examples.tools.web_search_filters`

### 4. 会话、记忆与人工审批

- [ ] `examples.memory.advanced_sqlite_session_example`
- [ ] `examples.memory.compaction_session_example`
- [ ] `examples.memory.compaction_session_stateless_example`
- [ ] `examples.memory.dapr_session_example`
- [ ] `examples.memory.encrypted_session_example`
- [ ] `examples.memory.file_hitl_example`
- [ ] `examples.memory.hitl_session_scenario`
- [ ] `examples.memory.memory_session_hitl_example`
- [ ] `examples.memory.mongodb_session_example`
- [ ] `examples.memory.openai_session_example`
- [ ] `examples.memory.openai_session_hitl_example`
- [ ] `examples.memory.redis_session_example`
- [ ] `examples.memory.sqlalchemy_session_example`
- [ ] `examples.memory.sqlite_session_example`

### 5. 模型提供方与推理内容

- [ ] `examples.model_providers.any_llm_auto`
- [ ] `examples.model_providers.any_llm_provider`
- [ ] `examples.model_providers.custom_example_agent`
- [ ] `examples.model_providers.custom_example_global`
- [ ] `examples.model_providers.custom_example_provider`
- [ ] `examples.model_providers.litellm_auto`
- [ ] `examples.model_providers.litellm_provider`
- [ ] `examples.reasoning_content.gpt_oss_stream`
- [ ] `examples.reasoning_content.main`
- [ ] `examples.reasoning_content.runner_example`

### 6. 完整应用示例

- [ ] `examples.customer_service.main`
- [ ] `examples.financial_research_agent.main`
- [ ] `examples.research_bot.main`

### 7. Sandbox 与执行环境

- [ ] `examples.sandbox.basic`
- [ ] `examples.sandbox.docker.docker_runner`
- [ ] `examples.sandbox.docker.mounts.azure_mount_read_write`
- [ ] `examples.sandbox.docker.mounts.gcs_mount_read_write`
- [ ] `examples.sandbox.docker.mounts.s3_mount_read_write`
- [ ] `examples.sandbox.docs.coding_task`
- [ ] `examples.sandbox.extensions.blaxel_runner`
- [ ] `examples.sandbox.extensions.cloudflare_runner`
- [ ] `examples.sandbox.extensions.daytona.daytona_runner`
- [ ] `examples.sandbox.extensions.daytona.usaspending_text2sql.agent`
- [ ] `examples.sandbox.extensions.daytona.usaspending_text2sql.setup_db` — 服务端/准备入口，先看目录 README；通常不产生 Agent 调用树
- [ ] `examples.sandbox.extensions.e2b_runner`
- [ ] `examples.sandbox.extensions.modal_runner`
- [ ] `examples.sandbox.extensions.runloop.capabilities`
- [ ] `examples.sandbox.extensions.runloop.runner`
- [ ] `examples.sandbox.extensions.temporal.local_hello_workflow`
- [ ] `examples.sandbox.extensions.temporal.temporal_sandbox_agent`
- [ ] `examples.sandbox.extensions.vercel_runner`
- [ ] `examples.sandbox.handoffs`
- [ ] `examples.sandbox.healthcare_support.main`
- [ ] `examples.sandbox.memory`
- [ ] `examples.sandbox.memory_multi_agent_multiturn`
- [ ] `examples.sandbox.memory_s3`
- [ ] `examples.sandbox.misc.reference_policy_mcp_server`
- [ ] `examples.sandbox.sandbox_agent_capabilities`
- [ ] `examples.sandbox.sandbox_agent_with_remote_snapshot`
- [ ] `examples.sandbox.sandbox_agent_with_tools`
- [ ] `examples.sandbox.sandbox_agents_as_tools`
- [ ] `examples.sandbox.shared_session_workdirs`
- [ ] `examples.sandbox.tax_prep`
- [ ] `examples.sandbox.tutorials.data.dataroom.setup`
- [ ] `examples.sandbox.tutorials.dataroom_metric_extract.evals`
- [ ] `examples.sandbox.tutorials.dataroom_metric_extract.main`
- [ ] `examples.sandbox.tutorials.dataroom_qa.main`
- [ ] `examples.sandbox.tutorials.repo_code_review.evals`
- [ ] `examples.sandbox.tutorials.repo_code_review.main`
- [ ] `examples.sandbox.tutorials.sandbox_resume.main`
- [ ] `examples.sandbox.tutorials.vision_website_clone.main`
- [ ] `examples.sandbox.unix_local_pty`
- [ ] `examples.sandbox.unix_local_runner`

### 8. Realtime 与语音

- [ ] `examples.realtime.app.server` — 服务端/准备入口，先看目录 README；通常不产生 Agent 调用树
- [ ] `examples.realtime.cli.demo`
- [ ] `examples.realtime.twilio.server` — 服务端/准备入口，先看目录 README；通常不产生 Agent 调用树
- [ ] `examples.voice.static.main`
- [ ] `examples.voice.streamed.main`

## 4. 特殊情况与边界

- `examples/run_examples.py` 是官方批量运行器，不放进上面的单例清单：它启动子进程，子进程不会自然继承父进程的 OpenTelemetry span。逐个模块走 `run_official.py` 才能清楚地把每次练习和一条 trace 对上。
- `tools/shell.py`、`local_shell_skill.py`、sandbox、patch、computer-use 等示例可能执行命令、改动本地文件或访问外部环境。先读源码和 README，确认工作目录和目标服务，再决定运行。
- MCP、Realtime、Voice、Hosted tools、远端 sandbox 等示例有些 span 只表示本地发起请求；远端系统需要单独启用 OpenTelemetry 才能查看服务端内部过程。
- 自定义模型提供方或直接创建独立 OpenAI/httpx 客户端的示例，按实际调用链确认自动埋点覆盖范围。OpenObserve 有 trace 并不代表每个远端组件都已经接入。
- 请求/响应正文继续沿用项目现有脱敏和截断规则；模型自然语言和工具输出可能包含敏感内容，练习时使用合成数据。
- 官方仓库新增或改名示例后，以当前 checkout 的目录和 README 为准，更新本手册的模块列表。

## 5. 结束时自查

- [ ] 我能从 trace ID 找到一次运行，并确认根 span 的 `app.run.target`。
- [ ] 我能区分 Agent、模型调用、工具调用和 HTTP span。
- [ ] 我能从工具 span 的 `input.value` / `output.value` 解释函数收到的参数和返回值。
- [ ] 我知道流式/远端/外部服务场景哪些内容不会自动出现在本地 trace。
- [ ] 所有需要额外服务或凭据的示例都记下了运行条件和结果。
