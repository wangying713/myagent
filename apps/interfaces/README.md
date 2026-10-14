# 对外交互与入口适配

此目录按交互方式组织入口。`console` 是通用管理台模板；`cli/learning` 运行 Agent 学习课程，相关协议和行为位于 `domain/agent`，模型与观测接入位于 `infra`。

| 目录 | 职责 | 当前状态 |
| --- | --- | --- |
| `http` | 后台服务 API 的路由、请求校验与响应转换 | 预留 |
| `console` | 通用管理台前端，后续通过 HTTP API 使用后台能力 | Ant Design Pro 官方完整版演示，使用本机 Mock |
| `cli/learning` | 学习命令行入口、课程示例与依赖组装 | 第一阶段已实现 |

Go 应用启动配置由 `apps/config` 加载，入口将各组件所需配置显式传入。前端自己的构建和运行配置放在 `console` 内。
