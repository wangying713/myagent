# Ant Design Pro 官方完整版演示

本目录安装 Ant Design Pro 官方 `v6.0.0`，保留官方页面、布局、登录和权限示例。欢迎、个人页和 AI 助手保留独立入口，其他示例统一归档到“示例页面（Demos）”。其中“Go Agent 示例”调用本项目 Go 后台，其他官方示例继续使用 Mock。

## 运行

需要 Node.js 20 或以上版本。在项目根目录执行：

```sh
cd apps/interfaces/console
npm ci
npm start
```

本机地址：`http://127.0.0.1:8000`。默认显示中文，使用模板自带的本机 Mock 数据。按 Ctrl+C 停止服务。

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
| 示例页面（Demos） | `src/pages/demos` | `/demos/...` |

登录页 `src/pages/user/login` 和全站兜底页 `src/pages/404.tsx` 保留在外面，不显示在侧边菜单。注册与注册结果作为示例放在 `demos/user`；异常页中的 404 示例放在 `demos/exception`。

示例页面包括：

- 仪表盘：分析页、监控页、工作台。
- 表单页：基础表单、分步表单、高级表单。
- 列表页：查询表格、标准列表、卡片列表、搜索列表。
- 详情页、结果页、异常页。
- 管理员权限示例、注册示例。权限示例仅管理员可见，其他示例普通用户也能查看。
- 官方 AI 助手示例也保留，发送时返回本机固定演示响应。
- Go Agent 示例：普通聊天、消息历史、乘法工具循环，调用本机 Go 服务，展示实际请求、响应、工具和 trace ID。

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

## Go demo 联调

先在项目根目录准备 `.env` 并启动 Go 服务：

```sh
./scripts/go run ./apps/interfaces/http -env-file .env
```

再在本目录执行 `npm start`，打开 `http://127.0.0.1:8000/demos/go-agent`。登录后选择一课，保留或修改默认问题，再点击“执行演示”。第二课一次完成两轮，每次点击都重新开始。

`config/proxy.ts` 只将 `/api/demos/*` 转发到 `http://127.0.0.1:8080`，保留其他官方 Mock。若改了 Go 监听端口，同步修改该代理目标。模型密钥和观测鉴权留在 Go 配置，前端不保存凭据。

“取消”中止本次请求；切换标签和重复提交在执行期间禁用。接口返回业务错误时仍展示已经完成的步骤，网络错误和取消分别显示提示。

模板的完整中文介绍见 [README.zh-CN.md](README.zh-CN.md)，来源与适配记录见 [UPSTREAM.md](UPSTREAM.md)。
