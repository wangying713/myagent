# HTTP 共用数据定义

HTTP API 方法的请求、响应及共用数据类型统一放在本目录。协议源文件放在 `http/proto`，请求响应类型由生成脚本输出到 `entity`，HTTP JSON 表现类型手写在 `entity`。业务处理方法与数据转换放在 `http` 下对应业务包，通用解析、超时和响应写出由 `infra/httpx` 提供。

```text
entity/
  agent.pb.go               # 自动生成的请求、响应、步骤和用量
  agent_json.go             # 手写的 HTTP JSON 结果、步骤和用量
```

后续新增接口遵循以下规则：

- API 方法统一使用 `entity` 中的请求和响应类型。消息字段由 `.proto` 定义，通过脚本生成 `.pb.go`；不手写重复的请求响应定义，也不用类型别名转出另一包的生成类型。
- Proto 源文件直接放在 `http/proto` 下，按业务分文件，`go_package` 指向 `entity`；修改后通过项目生成脚本更新 `entity` 中的生成文件，同时维护生成一致性检查。
- 手写 JSON 表现类型按业务分文件，并使用清楚的名称，例如 `AgentExecutionResult`；它们用于保留当前 HTTP JSON 契约，多个接口复用同一类型。
- 数据定义只承载接口字段与序列化约定；参数转换放在对应业务适配包的 `mapping.go`，字段准备和错误映射也放在该业务包，例如 `http/agent`。
- `entity` 不依赖业务处理包、领域服务、Gin 或基础设施。领域层和 `infra/httpx` 也不依赖 `entity`；`httpx` 的公共响应外层与传输错误仍由该包定义。
- JSON 字段名、数字、零值、null、空数组和可选字段都属于接口契约，新增或修改字段时同步转换代码及契约测试。

Agent 的生成命令和 JSON 兼容规则见 [接口说明](../agent/README.md)。
