# 本地可观测平台（OpenObserve）接入说明

> 用途：把 Agent / 服务的 **trace 和日志**集中到本地一个平台里看，按 traceID 串起一次请求的全过程。
> 本地学习/开发用，服务跑在 `~/docker-stack/openobserve`。
> ⚠️ 本文含**明文凭据**，仅本机自用。别提交到 git、别同步到云盘或外发。

---

## 一、服务信息

| 项 | 值 |
|---|---|
| UI 地址 | http://localhost:5080/web/ |
| 登录邮箱 | `wangying713@163.com` |
| 登录密码 | `LIyuan1234` |
| 组织 / stream | `default`（trace 和日志默认都进这个 stream） |
| OTLP traces 端点 | `http://localhost:5080/api/default/v1/traces` |
| OTLP logs 端点 | `http://localhost:5080/api/default/v1/logs` |
| 查询 API | `POST http://localhost:5080/api/default/_search?type=traces` |
| gRPC | 5081（本项目用不到） |

**界面语言**：登录后右上角用户菜单 → `Language` → 简体中文（原生中文，非机翻）。

### 运维命令

```bash
cd ~/docker-stack/openobserve
docker compose ps            # 看状态
docker compose logs -f       # 看日志（排查"数据没进来"必看）
docker compose stop          # 停（保留数据）
docker compose down          # 停并删容器（数据卷还在）
docker compose down -v       # ⚠️ 连数据一起删
```

---

## 二、怎么接入（三步）

### 1. 鉴权说明

所有 API 和 OTLP 端点都用 **HTTP Basic**：

```
用户名 = 登录邮箱     wangying713@163.com
密码   = 登录密码     LIyuan1234
Header = Authorization: Basic base64(邮箱:密码)
```

算一下（macOS）：

```bash
printf 'wangying713@163.com:LIyuan1234' | base64
# 得到 d2FuZ3lpbmc3MTNAMTYzLmNvbTpMSXl1YW4xMjM0
```

### 2. 先跑通连通性（复制即用）

```bash
# 注意 --noproxy '*'：本机 shell 配了 http_proxy，不加会被代理拦掉
curl -s -o /dev/null -w "HTTP %{http_code}\n" --noproxy '*' \
  -u "wangying713@163.com:LIyuan1234" \
  http://localhost:5080/api/default/users
# 期望 HTTP 200

# 推一条空 trace（验证端点+鉴权）
curl -s -w "HTTP %{http_code}\n" --noproxy '*' \
  -X POST -H "Content-Type: application/x-protobuf" \
  -u "wangying713@163.com:LIyuan1234" \
  --data-binary "" \
  http://localhost:5080/api/default/v1/traces
# 期望 HTTP 200
```

### 3. 接自己的服务

**方式 A：已有 OpenTelemetry SDK（推荐）**

标准 OTLP，用环境变量就能指过来：

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:5080/api/default
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Basic d2FuZ3lpbmc3MTNAMTYzLmNvbTpMSXl1YW4xMjM0"
export OTEL_SERVICE_NAME=my-service
```

> 注意：`OTEL_EXPORTER_OTLP_ENDPOINT` 只写到 `/api/default`，SDK 会自动补 `/v1/traces`。
> 但 Go 的 `otlptracehttp.WithEndpointURL()` **不会**自动补，得自己拆 host + path（坑 3.1）。

**方式 B：参照本仓库 mini-agent 的实现**

`mini-agent/otel.go` 是一份最小可用的 Go 接入示例（约 200 行、零框架）：

```
一次提问        → trace（traceID 打印在控制台）
├─ chat 轮次    → span，attrs: gen_ai.request.model / usage tokens / finish_reason
│                 events: 完整请求体 + 响应体
└─ execute_tool → 子 span，attrs: tool.name / read_only / seq / elapsed_ms
```

切换上报目标只改 `mini-agent/config.env` 三行：

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:5080/api/default
OTEL_AUTH_USER=wangying713@163.com
OTEL_AUTH_PASSWORD=LIyuan1234
```

**属性命名建议**：想让 UI 正确识别出"LLM 调用"和"工具调用"，按 OTel GenAI 语义约定打属性：

| 场景 | 关键属性 |
|---|---|
| LLM 调用 | `gen_ai.operation.name=chat`、`gen_ai.request.model`、`gen_ai.usage.input_tokens`、`gen_ai.usage.output_tokens`、`gen_ai.response.finish_reasons` |
| 工具调用 | `gen_ai.operation.name=execute_tool`、`gen_ai.tool.name`、`gen_ai.tool.call.id` |
| 输入输出原文 | `langfuse.observation.input` / `langfuse.observation.output`（Langfuse 会优先读它；OpenObserve 里也能看到） |

---

## 三、怎么看数据

### UI

左侧 **Traces** → 选 stream `default` → 能看到 trace 列表、每条 trace 的调用树和瀑布图；点某个 span 看属性与事件（我们的请求/响应原文就挂在 events 里）。

### 日志

trace 和日志**都已接入**（同一份代码、同一个 endpoint，只是路径不同）：

| 信号 | 端点 | 代码位置 |
|---|---|---|
| trace | `.../v1/traces` | `mini-agent/otel.go` |
| 日志 | `.../v1/logs` | `mini-agent/otel_log.go` |

左侧 **Logs** → stream `default`。每条日志都自动带 `trace_id` / `span_id`，所以：

- 在 trace 里看到可疑的一轮 → 复制它的 trace_id → 到 Logs 里按 `trace_id` 过滤，就能看到这一轮前后的全部日志
- 日志里也带了结构化字段，可以直接当列用（**查询时把属性名里的点换成下划线**）：

| 我们代码里的属性名 | 查询时用的列名 | 含义 |
|---|---|---|
| `gen_ai.usage.input_tokens` | `gen_ai_usage_input_tokens` | 本轮输入 token |
| `gen_ai.usage.output_tokens` | `gen_ai_usage_output_tokens` | 本轮输出 token |
| `gen_ai.response.finish_reasons` | `gen_ai_response_finish_reasons` | stop / tool_calls |
| `agent.round` | `agent_round` | 第几轮 |
| `agent.messages` | `agent_messages` | 这轮的 messages 条数 |
| `agent.cache_hit_tokens` | `agent_cache_hit_tokens` | 命中缓存 |
| `llm.elapsed_ms` | `llm_elapsed_ms` | 等模型耗时 |
| `gen_ai.tool.name` | `gen_ai_tool_name` | 工具名 |
| `agent.tool.elapsed_ms` | `agent_tool_elapsed_ms` | 工具耗时 |
| ——（日志级别，由 OTLP 自带） | `severity` | `INFO` / `WARN` / `ERROR`，**不是** `severity_text` |

> 日志级别：异常状态（`finish_reason=length`、token 预算触顶）用 `WARN` / `ERROR` 上报，正常流程是 `INFO`，所以可以直接 `where severity != 'INFO'` 捞出所有异常。

**设计取舍**：日志在**终端照原样打印**（不加 `time= level=` 前缀，不破坏可读性），结构化字段只走上报通道。所以同一行日志，人看到的是原来那句，平台拿到的是带字段的记录。

### SQL 查询（脚本化排查用）

```bash
NOW=$(python3 -c 'import time;print(int(time.time()*1000000))')
START=$((NOW - 3600000000))   # 往前 1 小时

curl -s --noproxy '*' -u "wangying713@163.com:LIyuan1234" \
  -H "Content-Type: application/json" \
  -X POST "http://localhost:5080/api/default/_search?type=traces" \
  -d "{\"query\":{\"sql\":\"select trace_id, service_name, operation_name, duration from \\\"default\\\" order by start_time desc limit 20\",\"start_time\":$START,\"end_time\":$NOW}}"
```

> `start_time` / `end_time` 是**微秒时间戳**，且**必须给**，给 0 会报 `Error# [file_list] invalid time range`。

---

## 四、踩过的坑（能省你半天）

### 4.1 OTLP 路径不会自动补

Go 的 `otlptracehttp.WithEndpointURL("http://host:5080/api/default")` 会**原样使用路径**，请求打到 `/api/default` → 404。要拆成：

```go
otlptracehttp.WithEndpoint("localhost:5080"),
otlptracehttp.WithURLPath("/api/default/v1/traces"),
otlptracehttp.WithInsecure(), // http 明文必须加
```

### 4.2 上报失败是"静默"的

OpenTelemetry SDK 默认把导出错误吞掉，程序看起来一切正常。**接入第一步先把错误暴露出来**：

```go
otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
    fmt.Printf("[otel] 上报失败：%v\n", err)
}))
```

我们就是靠这行才发现 404 / 401 的。

### 4.3 密码强度：启动校验 vs 运行期不校验

- **启动时** `ZO_ROOT_USER_PASSWORD` **强校验**：8-128 位，必须同时含大写、小写、数字、**特殊字符**；不合规进程直接 panic 起不来
- **运行期**通过 API 改密码**不校验**：

```bash
curl -X PUT --noproxy '*' -u "wangying713@163.com:<当前密码>" \
  -H "Content-Type: application/json" \
  -d '{"old_password":"<当前密码>","new_password":"<新密码>"}' \
  "http://localhost:5080/api/default/users/wangying713%40163.com"
```

- ⚠️ 字段名是 **`new_password`**，不是 `password`：传 `password` 会返回 `User updated successfully` 但**密码根本没变**（必须用真实登录验证，别信返回）
- 因此 `docker-compose.yml` 里保留的是合规值 `Liyuan1234!` 作为**初始化密码**。含义：正常重启不重置密码；只有 `down -v` 重建后才会回到 `Liyuan1234`

### 4.4 "查不到数据" ≠ "数据没进来"

数据先写 WAL，过几秒才刷成可查的 parquet。判断数据有没有被接收，看**接收端日志**：

```bash
cd ~/docker-stack/openobserve && docker compose logs --tail=50 | grep "v1/traces"
# 会看到类似： "POST /api/default/v1/traces HTTP/1.1" 200 - "31527" "OTel OTLP Exporter Go/1.32.0"
# 引号里的 31527 就是收到的字节数
```

### 4.5 拉镜像（国内网络）

| 源 | 结果 |
|---|---|
| `docker.io/openobserve/openobserve` | ❌ TLS handshake timeout |
| `docker.m.daocloud.io/...` | ❌ 有白名单，报 "not in the allowlist" |
| `docker.1ms.run/openobserve/openobserve` | ✅ 可用 |
| **`public.ecr.aws/zinclabs/openobserve`（官方）** | ✅ 最快，推荐 |

```bash
docker pull public.ecr.aws/zinclabs/openobserve:latest
docker tag public.ecr.aws/zinclabs/openobserve:latest openobserve/openobserve:latest
```

另外 `~/.docker/daemon.json` 里的 `registry-mirrors` **会被 Docker Desktop 重启覆盖**，别指望它长期生效。

### 4.6 端口冲突

本机原有 `local-redis` 占了 `0.0.0.0:6379`，Langfuse 的 redis 已挪到 **6380**。新装服务前先查：

```bash
lsof -nP -iTCP -sTCP:LISTEN | grep -E ':(5080|3000|6379|3306|9200)'
```

### 4.7 http_proxy 会拦 localhost

本机 shell 配了 `http_proxy`，命令行 `curl localhost:...` 会失败。加 `--noproxy '*'`（应用代码直连不受影响）。

---

## 五、同时装的另一套：Langfuse

本机还跑着一套 **Langfuse**（LLM 语义 UI 更专业：直接展示每轮 prompt / 输出 / token / 工具调用，界面是英文）。

| | OpenObserve | Langfuse |
|---|---|---|
| 地址 | http://localhost:5080/web/ | http://localhost:3000/auth/sign-in |
| 账号 | `wangying713@163.com` / `LIyuan1234` | `wangying713@163.com` / `LIyuan1234` |
| 语言 | ✅ 原生中文 | 英文 |
| OTLP endpoint | `http://localhost:5080/api/default` | `http://localhost:3000/api/public/otel` |
| 鉴权用户名/密码 | 邮箱 / 密码 | **Public Key / Secret Key** |
| 强项 | 日志 + trace 一起看、中文、通用 | LLM 语义视图（prompt/输出/token 一目了然） |

**同一份 OTLP 数据，改三行环境变量就能切换目标**，代码不用动——这就是选 OTLP 而不是某家私有 SDK 的价值。

> 如果同时维护 Langfuse：它的 MinIO 密码必须与 compose 里 `LANGFUSE_S3_*_SECRET_ACCESS_KEY`（默认 `miniosecret`）一致，否则 OTLP 上报会 500 `SignatureDoesNotMatch`。

---

## 六、完整凭据总表

本机全部服务（OpenObserve / Langfuse / MySQL / Redis / ES）的账号密码、端口速查：

```
~/docker-stack/本机服务账号密码.md
```

---

## 七、5 分钟接入 checklist

- [ ] `curl` 打通 `/api/default/users`，确认 HTTP 200（鉴权 OK）
- [ ] `curl` 打通 `/api/default/v1/traces`，确认 HTTP 200（端点 OK）
- [ ] 代码里接 OTLP，**并先加上 `otel.SetErrorHandler`**（否则失败无感）
- [ ] 跑一次业务请求，拿到 traceID
- [ ] 用 SQL 查询或 UI 确认数据已入库
- [ ] 记下 traceID，去 UI 里点开看调用树和 events（请求/响应原文）
