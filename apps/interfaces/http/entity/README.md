# HTTP 共用数据定义

HTTP 接口手写的共用数据结构放在本目录，协议定义与生成文件放在 `http/proto`。业务处理方法与数据转换放在 `http` 下对应业务包，通用解析、超时和响应写出由 `infra/httpx` 提供。

```text
entity/
  agent.go                  # Agent 共用的 HTTP JSON 结果、步骤和用量
```

后续新增接口遵循以下规则：

- HTTP 数据结构放入 `http/entity`，手写类型按业务分文件，并使用清楚的名称，例如 `AgentExecutionResult`；多个接口复用同一类型，不重复声明。
- Proto 消息直接放在 `http/proto` 下，按业务分文件，修改 `.proto` 后通过项目生成脚本更新生成文件，同时维护生成一致性检查。
- 数据定义只承载接口字段与序列化约定；参数转换、字段准备和错误映射放在对应业务适配包，例如 `http/agent`。
- `entity` 不依赖业务处理包、领域服务、Gin 或基础设施。领域层和 `infra/httpx` 也不依赖 `entity`；`httpx` 的公共响应外层与传输错误仍由该包定义。
- JSON 字段名、数字、零值、null、空数组和可选字段都属于接口契约，新增或修改字段时同步转换代码及契约测试。

Agent 的生成命令和 JSON 兼容规则见 [接口说明](../agent/README.md)。
