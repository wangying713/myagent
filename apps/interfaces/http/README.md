# 后台服务 API

`main.go` 组装配置、DeepSeek、OpenObserve 和三课执行器；`api` 包负责路由、请求校验、总超时和 JSON 响应。

三课原理、操作练习与通过标准见 [中文学习指南](../../../docs/learning/README.md)。

在项目根目录运行：

```sh
./scripts/go run ./apps/interfaces/http -env-file .env
```

默认监听 `127.0.0.1:8080`。服务不会自动读取任何 `.env`；环境变量优先于显式文件。启动时缺少模型或已启用观测的鉴权会明确报错。

| 方法与接口 | 正文 | 含义 |
| --- | --- | --- |
| `GET /api/demos/health` | 无 | 服务存活检查，不调用模型、不检查外部服务 |
| `POST /api/demos/chat` | `{}` 或 `{"question":"问题"}` | 普通聊天 |
| `POST /api/demos/messages` | `{}` 或 `{"questions":["第一轮问题","第二轮问题"]}` | 单次执行两轮，第二轮携带第一轮实际历史 |
| `POST /api/demos/agent` | `{}` 或 `{"question":"乘法问题"}` | 只允许乘法工具的有限循环 |

POST 使用 `Content-Type: application/json`。空对象使用原 Python demo 默认问题；拒绝未知或跨课程字段、重复字段、null、非法类型、空白问题及多段 JSON。每个问题最多 4096 字符，正文最多 32 KiB。

响应的 `data` 包含 `completed`、`answer`、`steps`、耗时、调用数、已知 token、用量完整性和 `trace_id`。步骤按真实执行顺序保存脱敏后的请求与响应；工具步骤的请求中保留调用 ID 与 JSON 参数。

失败仍保留已执行记录，并返回 `error.code` 和中文 `error.message`：

| HTTP 状态 | 错误码 | 含义 |
| --- | --- | --- |
| 400 | `invalid_input` | 请求结构或课程输入无效 |
| 413 | `request_too_large` | 正文超限 |
| 415 | `invalid_content_type` | 正文不是 JSON |
| 409 | `budget_exceeded` | 达到预算，任务未完成 |
| 408 | `canceled` | 调用方取消；客户端已经断开时无法再收到响应 |
| 504 | `timeout` | 总运行或单次模型请求超时 |
| 502 | `invalid_model_response` / `model_request_failed` | 模型协议或服务错误 |

默认最多 5 次模型请求、8 次工具分发、8000 个已知累计 token 的调用间软预算，不自动重试。用量缺失时不伪装为零；预算不能证明未知用量没有超限。单次模型输出上限为 512 token，思考模式关闭。

每次运行独立保存历史和统计；取消信号传到模型 HTTP 请求，后续工具和模型调用停止。每次点击开始新运行，不共享或持久化会话。

一个根 span 对应一次运行，模型和工具各有子 span。OpenObserve 上报异步进行，失败提示但不覆盖模型结果；只有服务退出时关闭 provider，退出刷新有界。`TELEMETRY_CAPTURE_CONTENT` 控制观测正文采集；页面仍展示用于学习的脱敏执行记录。展示正文默认最多 32768 字符，截断有标记，不改变实际模型消息。

当前是本机开发演示：管理台登录仍属于模板 Mock，不是 Go 服务的真实鉴权。监听地址默认为本机；本次不增加生产部署、数据库、流式、会话存储或其他 Python 课程。
