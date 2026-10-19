# Ant Design Pro 官方完整版演示

本目录安装 Ant Design Pro 官方 `v6.0.0`，保留官方页面、布局、登录和权限示例。欢迎、个人页、AI 助手、Agent Demos 和 API 保留独立入口，其他示例统一归档到“示例页面（Demos）”。“Agent Demos”和“API”调用本项目 Go 后台，其他官方示例继续使用 Mock。

## 运行

需要 Node.js 20 或以上版本。在项目根目录执行：

```sh
cd apps/interfaces/console
npm ci
cd ../../..
./scripts/server
```

`./scripts/server` 无参数时默认同时重启 Go 后台和 Node 管理台，不需要开两个终端；请先准备项目根目录的 `.env`。前端使用现有 `npm start` 配置，保留本机 Mock 和热更新。启动成功前检查管理台页面及 `/api/health` 到 Go 的代理，日志位于 `.runtime/console/server.log`，Go 日志位于 `.runtime/http/server.log`。已运行时先停止脚本管理的进程及其子进程，未运行时直接启动。

从项目根目录统一管理前后端：

```sh
./scripts/server start
./scripts/server status
./scripts/server stop
```

`start` 会保留已运行的进程并补启动缺失的服务，`stop` 同时停止前后端，`status` 同时显示两者状态。前端进程管理实现留在本目录 `scripts/server.mjs`，作为统一脚本的内部辅助，不需要单独调用。

也可以继续手动在本目录运行：

```sh
npm start
```

本机地址：`http://127.0.0.1:8000`。默认显示中文，使用模板自带的本机 Mock 数据。手动运行时按 Ctrl+C 停止服务；切换到整套脚本前先停止手动服务，以免占用端口。

## 演示账号

| 角色 | 用户名 | 密码 |
| --- | --- | --- |
| 管理员 | `admin` | `ant.design` |
| 普通用户 | `user` | `ant.design` |

账号仅用于模板本机演示。切换账号可查看管理员菜单与普通用户菜单的区别。

开发环境的登录表单默认填入 `admin` / `ant.design`，可直接点击登录，也可手动改为普通用户。生产构建不预填演示账号。

## 页面组织

默认首页为 `/welcome`，示例菜单默认收起。

| 菜单 | 页面目录 | 路由 |
| --- | --- | --- |
| 欢迎 | `src/pages/Welcome.tsx` | `/welcome` |
| 个人页 | `src/pages/account` | `/account/center`、`/account/settings` |
| AI 助手 | `src/pages/chatbot` | `/chatbot` |
| Agent Demos | `src/pages/agent-development` | `/agent-demos/overview` 及各章子路由 |
| API | `src/pages/api-explorer` | `/api-explorer` |
| 示例页面（Demos） | `src/pages/demos` | `/demos/...` |

登录页 `src/pages/user/login` 和全站兜底页 `src/pages/404.tsx` 保留在外面，不显示在侧边菜单。注册与注册结果作为示例放在 `demos/user`；异常页中的 404 示例放在 `demos/exception`。

示例页面包括：

- 仪表盘：分析页、监控页、工作台。
- 表单页：基础表单、分步表单、高级表单。
- 列表页：查询表格、标准列表、卡片列表、搜索列表。
- 详情页、结果页、异常页。
- 管理员权限示例、注册示例。权限示例仅管理员可见，其他示例普通用户也能查看。
- 官方 AI 助手示例也保留，发送时返回本机固定演示响应。

这些页面展示官方模板自身的效果。示例图片和字体可能来自官方静态资源地址；业务演示数据由本机 Mock 提供，不读取真实账号、密钥或数据库。

页面专属的组件、样式、类型、服务和 Mock 随页面目录归档。公共组件、公共服务及共享 Mock 工具仍在公共目录。后续实际业务页面在 `src/pages` 下建立独立目录，并在 `config/routes.ts` 中加入独立菜单，不依赖 `demos`。

## 验证与构建

前端代码规范见 [本项目前端具体约定](../../../docs/development/frontend-decisions.zh-CN.md)，Google 语言规范的中文整理与来源见 [开发规范目录](../../../docs/development/README.md)。

```sh
npm run tsc
npm run build
```

构建前先停止开发服务，构建完成后重新执行 `npm start`，避免打包器缓存影响开发环境。

当前完整体验使用 `npm start`，登录与列表等演示依赖本机 Mock；直接部署静态构建产物需要另外提供演示数据服务。后续接入真实后台另行处理。

## API 接口调试

点击独立“API”菜单，或打开 `/api-explorer`。页面根据实际 Go 接口提供列表，包含健康检查 GET/HEAD，以及聊天、消息历史、Agent 运行三个 POST 接口。此页面按接口组织，不依赖课程演示页。

选中接口后可编辑默认 JSON，点击“发送请求”调用同源后台代理；GET/HEAD 不发送请求体。每个接口的编辑草稿在页面内分别保留，支持恢复默认参数。4xx/5xx 仍展示实际 HTTP 状态和响应正文，Trace ID 可复制。JSON 响应默认按两空格缩进、逐层换行显示，格式化只调整字符串之外的空白，保留大整数、精确小数、字段顺序和转义内容；非 JSON 响应保持原文。复制响应仍使用收到的原始正文。请求期间可取消，切换接口或离开页面也会取消旧请求；迟到响应不覆盖当前结果。

接口定义与参数示例维护于 `src/pages/api-explorer/data.ts`，与 `apps/interfaces/http/app/router.go` 同步；后续新增后台接口时在列表中补充对应配置。此页不使用模板生成的 OpenAPI 示例或课程页面数据，也不保存模型密钥。

## Agent 学习文档与接口联调

Agent Demos 按 13 章组织二级菜单，每章有文档、相关原文和页内请求区域。正文来自项目 `docs` 中的 Markdown；修改后重启前端开发服务或重新构建，即可更新正文。旧文档链接仍可访问。

原有接口继续在独立 `/api-explorer` 页面调试。`config/proxy.ts` 将 `/api/agent/*`、`/api/agent-demos/*` 和 `/api/health` 转发到 `http://127.0.0.1:8080`，其余官方 Mock 保留。模型密钥和观测鉴权留在 Go 配置，前端不保存凭据。

模板的完整中文介绍见 [README.zh-CN.md](README.zh-CN.md)，来源与适配记录见 [UPSTREAM.md](UPSTREAM.md)。

## Agent Demos

管理台菜单已改为 **Agent Demos**，按学习路线展开 13 个章节。打开章节后可阅读文档、点击官方原文或相关参考链接，并在当前页编辑 JSON、发送请求、停止请求和查看真实执行结果。旧文档地址 `/agent-development` 仍可打开；新入口为 <http://127.0.0.1:8000/agent-demos/overview>。

Demo 请求 `/api/agent-demos/*`；原「API」页面和 `/api/agent/*` 全部保留。模型和观测继续使用现有 Go 启动配置，前端不保存密钥。检索与商品资料使用内置样本；评估使用三道固定题；流式页面实时读取模型内容，停止或切换章节会取消请求。

学习文档来自 `docs/learning`，修改后重启开发服务或重新构建以重新载入。
