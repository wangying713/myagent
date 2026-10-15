# Ant Design Pro 官方演示约定

- 前端代码遵循 [本项目前端具体约定](../../../docs/development/frontend-decisions.zh-CN.md)，并按其中的适配规则使用 Google TypeScript 与 HTML/CSS 中文规范；格式以本目录 Biome 配置为准。
- 保留 Ant Design Pro 官方完整版演示及样式；本次经用户授权，在 `demos/go-agent` 新增指定的三个 Go demo 接口页面，不改造官方 AI 助手或其他示例。
- 常用入口为欢迎、个人页和 AI 助手；其他官方示例的菜单、路由和文件统一放入 `demos`，页面专属资源随目录移动。登录页和全站 404 作为基础入口保留在外面。
- 保留官方布局、登录、权限和 Mock 示例；仅权限示例限制管理员访问。新增 Go demo 页面通过 `/api/demos/*` 请求 Go 后台，由后台调用模型；其余示例继续使用原 Mock。
- 前端配置放在本目录。运行说明使用中文，默认界面语言为中文。
- 官方自动生成的 `src/services` 不手动编辑；后续实际业务页面使用独立目录与菜单，不依赖 `demos`。
- 模板来源、固定版本和本机适配记录在 `UPSTREAM.md`；运行方法和演示账号记录在 `README.md`。
- 修改 Ant Design 组件前查询对应版本 API；验证以类型检查、构建和实际浏览器体验为准。
