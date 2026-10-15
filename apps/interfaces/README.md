# 对外交互与入口适配

此目录按交互方式组织入口。`console` 是通用管理台模板，Agent 开发页面通过 HTTP 调用 Go 后台；业务流程位于 `domain/agent`。

| 目录 | 职责 | 当前状态 |
| --- | --- | --- |
| `http` | 通过 Gin 组织后台 API、请求校验与响应转换 | 三课接口与健康检查已接入 Gin |
| `console` | 通用管理台前端，通过 HTTP API 使用后台能力 | 官方示例继续 Mock；独立的 Agent 开发页面调用真实 Go 服务 |
| `cli` | 命令行入口与依赖组装 | 预留 |

`http/router.go` 显式注册完整路由，`http/proto` 用 Proto 定义业务消息并生成 Go 类型，`http/agent` 实现具名处理方法与 JSON 适配；通用 HTTP 解析和响应由 `infra/httpx` 提供，不依赖业务。生成与使用说明见 [HTTP API](http/agent/README.md)。

`apps/config` 加载 Go 启动配置，由入口将各组件所需配置显式传入。前端自己的构建和运行配置放在 `console` 内。
