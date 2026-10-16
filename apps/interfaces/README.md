# 对外交互与入口适配

此目录按交互方式组织入口。`console` 是通用管理台模板，Agent 开发页面通过 HTTP 调用 Go 后台；业务流程位于 `domain/agent`。

| 目录 | 职责 | 当前状态 |
| --- | --- | --- |
| `http` | 通过 Gin 组织后台 API、请求校验与响应转换 | 三课接口与健康检查已接入 Gin |
| `console` | 通用管理台前端，通过 HTTP API 使用后台能力 | 官方示例继续 Mock；独立的 Agent 开发页面调用真实 Go 服务 |
| `cli` | 命令行入口与依赖组装 | 预留 |

`http/main.go` 是唯一顶层 Go 文件，加载配置后调用 `app.Run(ctx, cfg)`；服务实现与测试集中在 `http/app`，`app/run.go` 组装依赖并管理资源。`http/app/router.go` 显式注册完整路由，`http/proto` 同时保存协议源文件和生成的请求响应类型，`http/entity` 保存手写 HTTP 公共外层与 JSON 表现类型。`http/app/impl_agent.go` 实现请求准备、具名处理方法并分别组装各接口响应；`http/app/handler.go` 处理 JSON 适配、映射错误，并包装 `entity.Rsp[T]` 的 `code/data/traceID/msg`，通用 HTTP 解析和编码由 `infra/httpx` 提供，不依赖业务。生成与使用说明见 [HTTP API](http/README.md)。

`apps/config` 加载 Go 启动配置，由入口将各组件所需配置显式传入。前端自己的构建和运行配置放在 `console` 内。
