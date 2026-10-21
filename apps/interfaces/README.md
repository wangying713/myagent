# 对外交互与入口适配

此目录按交互方式组织入口。`console` 是通用管理台模板，API 调试页通过 HTTP 调用 Go 后台，Agent Demos 提供文档与页内演示；业务流程位于 `domain/agent`。

| 目录 | 职责 | 当前状态 |
| --- | --- | --- |
| `http` | 通过 Gin 组织后台 API、请求校验与响应转换 | 原三课接口、健康检查与 13 章 Demo 接口已接入 Gin |
| `console` | 通用管理台前端，通过 HTTP API 使用后台能力 | 官方示例继续 Mock；独立 API 页调用真实 Go 服务；Agent Demos 按章节阅读并请求对应接口 |
| `cli` | 命令行入口与依赖组装 | 预留 |

`http/main.go` 是唯一顶层 Go 文件，加载配置后调用 `app.RunHTTP(ctx, cfg)`；服务实现与测试集中在 `http/app`，`app/http.go` 组装依赖并管理资源。`http/app/router.go` 显式注册完整路由，`http/entity` 统一保存手写请求、响应、HTTP 公共外层与共用 JSON 数据。`http/app/impl_agent.go` 实现请求准备、具名处理方法并分别组装各接口响应；`http/app/handler.go` 处理严格 JSON 解码、映射错误，并包装 `entity.Rsp[T]` 的 `code/data/traceID/msg`，通用 HTTP 解析和编码由 `infra/httpx` 提供，不依赖业务。接口与使用说明见 [HTTP API](http/README.md)。

`apps/config` 加载 Go 启动配置，由入口将各组件所需配置显式传入。前端自己的构建和运行配置放在 `console` 内。

管理台 **Agent Demos** 按学习路线提供 13 个二级章节，每章包含文档、相关原文链接及可编辑请求。Demo 接口为 `/api/agent-demos/*`，已有 API 页面和 `/api/agent/*` 继续保留。各章的实际演示范围见 `docs/learning`。
