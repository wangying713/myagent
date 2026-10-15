# Python 学习资料迁移说明

迁移日期：2026-10-14。以旧资料 `old/doc` 为主要来源；`old/python/doc` 是另一份历史副本，除工具章节与 Python 规范外内容相同。工具章节差异属于旧 Python 调用方式，本次按实际 Go 接口重写，不保留不适用的调用语法。

## 对应关系

| 旧资料 | 当前位置 | 迁移方式 |
| --- | --- | --- |
| 旧项目 README 的学习导航、启动和代码分工 | [学习入口](README.md) 与根 README | 改为当前 HTTP + 管理台启动，配置用根 `.env` |
| `levels/beginner.md` | [初级](levels/beginner.md) | 三课运行、阅读顺序、练习、通过标准 |
| `levels/intermediate.md` | [中级路线](levels/intermediate.md) | 保留结构化、流式、检索、会话目标，标注待实现 |
| `levels/advanced.md` | [高级路线](levels/advanced.md) | 保留思考、评估和资料 Agent 目标，标注待实现 |
| `lessons/01-http.md` | [01 普通聊天](01-chat.md) | Go HTTP/JSON、响应校验与页面步骤 |
| `lessons/02-messages.md` | [02 消息历史](02-messages.md) | Go 切片、完整 assistant、实际用量统计 |
| `lessons/03-agent-loop.md` | [03 工具循环](03-agent.md) | 手写工具 schema、严格分发、调用 ID、有限循环 |
| `lessons/04-json.md` | [04 结构化输出](04-json.md) | 保留三层校验，给出 Go 练习方向 |
| `lessons/05-stream-thinking.md` | [05 流式与思考](05-stream-thinking.md) | 保留重组、模式区别和实验目标，移除已实现声明 |
| `lessons/06-reliability.md` | [06 失败与预算](06-reliability.md) | 按当前取消、超时、预算、错误及测试说明 |
| `lessons/07-retrieval-memory.md` | [07 检索与状态](07-retrieval-memory.md) | 保留来源、隔离、冲突与恢复原理，标注待实现 |
| `lessons/08-evaluation.md` | [08 评估与交付](08-evaluation.md) | 保留固定用例与工具证据，分开当前契约与未来评估 |
| `openobserve.md` | [OpenObserve](openobserve.md) | 按实际 Go 字段、预览、生命周期和人工入库步骤重写 |
| `verification.md` | [验收说明](verification.md) 与 [历史验收](history/python-verification.md) | 当前 Go 命令和证据独立；旧结果归档 |
| `review.md` | [评审检查](review.md) 与 [历史评审](history/python-review.md) | 当前检查点独立；旧问题经过归档 |
| `python-style.md` | [Go 开发规范](../development/README.md) | 语言规则不机械翻译；Go 与前端遵循现有项目规范 |

## 不再依赖旧工作区

学习入口、实际源码、配置、测试和运行命令都在当前 Go 项目内。没有 `uv`、Python 包、旧虚拟环境、旧配置或旧验收脚本的运行依赖。

初级三课已实现；中高级资料迁移为后续路线和原理，不意味着对应代码已迁移。旧 Python 标准库练习、旧框架源码与旧业务代码属于历史内容，不纳入 Go Agent 当前课程。

`old` 现在作为废弃的历史归档，不再维护，也不作为新开发依据。本次没有删除旧代码和配置；后续移除目录不会影响当前学习流程。历史验收中的路径和命令只描述当时工作区，不应在当前项目执行。
