# 第一阶段：看懂并手写 Go Agent

目标是独立解释并实现：构造消息 → 请求模型 → 校验工具调用 → 执行本地工具 → 回填结果 → 正常结束或达到预算。

前置知识是 Go 基本语法、结构体、函数和错误返回。课程在需要时解释接口、上下文和资源关闭，不先引入 Agent 框架。

## 学习顺序

| 课程 | 重点 | 通过标准 |
| --- | --- | --- |
| [第一课：HTTP 与回答](01-chat.md) | 请求、响应、结束原因、用量 | 区分 HTTP 成功与完整回答，能查到自己的 trace |
| [第二课：消息历史](02-messages.md) | 应用保存历史并传回模型 | 解释第二轮为何需要第一轮 user 和 assistant |
| [第三课：工具与循环](03-agent.md) | 参数模式、受控执行、回填、预算 | 能解释调用 ID 配对和循环停止，不执行任意生成代码 |

先按[项目入口](../../README.md)准备配置，运行 `observe` 检查，再按顺序学习。每次只改一个条件，对照终端与 OpenObserve 中的实际正文。

## 代码怎么读

从 `apps/interfaces/learning` 的课程函数开始，追踪到 `infra/deepseek` 的 HTTP 请求。第三课再看 `domain/agent/service` 的循环和注册表。

`entity.ChatRequest` 与 `entity.ChatResponse` 是类型明确的协议；`service.ChatClient` 由使用方定义，因此测试可以用假模型替换真实 HTTP 客户端。领域层不读取环境变量。

Go 的 `context.Context` 负责把取消和截止时间沿调用链传递。`defer` 用于关闭响应体、空闲连接和观测资源。取消业务之后，已有观测仍有独立的五秒刷新时间。

## 第一阶段验收

先运行 `go test ./...` 与 `go test -race ./...`，再对每课运行 `-verify`。真实验收需要可用模型与 OpenObserve；HTTP 接收成功不等于记录已入库。

默认多轮课程检查是否记住“豆包”；默认工具课程检查程序确实算出 391，且最终回答包含结果。传入自定义 `-question` 时不使用固定题目的答案判断，但仍检查实际 trace。

观测详细说明见[OpenObserve](openobserve.md)。后续阶段才加入结构化输出、流式、RAG、持久化会话和评估。

本次实际结果见[第一阶段验收记录](verification.md)。
