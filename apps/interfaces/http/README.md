# 后台服务 API

此目录预留给后台 HTTP 服务 API，负责路由、请求校验、调用业务逻辑和响应转换。

Agent 协议和行为位于 `domain/agent`。Go 应用启动配置由 `apps/config` 加载，入口负责模型与观测依赖的组装。
