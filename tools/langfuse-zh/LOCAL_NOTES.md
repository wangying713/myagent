# 本地改动说明

上游：<https://github.com/leon30083/langfuse-zh>（MIT，2 stars / 4 commits，基本无人维护）

本目录是它的副本，做了本地适配 + 补词典。**`.git` 已移除**，改动直接进主仓库版本管理。

---

## 改了什么

### 1. `manifest.json`

| 项 | 原版 | 现在 |
|---|---|---|
| 扩展名 / 描述 | 乱码（编码问题） | 正确的 UTF-8：`Langfuse 中文翻译` |
| `content_scripts.matches` | 只有作者的 `192.168.101.16:3000` | **加了 `localhost:3000` 和 `127.0.0.1:3000`** |
| `permissions` | `["storage"]` | **不变**（没加任何网络权限） |

> 原版其实靠 `*://*/project/*` 这条通用规则也能在 localhost 生效，但写死作者的 IP 不直观，显式加上更稳。

### 2. `translations/zh.js`

补充了**调用链（Traces）/ 观测页**与通用界面术语：

| 类别 | 词条 |
|---|---|
| 导航 | Traces / Trace Detail / Trace Overview / Observations / Metadata / Details |
| 表头 | Timestamp / Latency / Total Tokens / Input·Output Tokens / Total Cost / Input·Output Cost / Trace ID / Observation ID / User ID / Session ID / Status Message / Release |
| 类型徽标 | `GENERATION`→生成 / `SPAN`→步骤 / `EVENT`→事件 / `TOOL`→工具 / `TRACE`→调用链 |
| 筛选操作 | Filters / Time Range / Apply / Clear / Reset / Refresh / Export / Delete / Copy / Share / Download / Add Filter / Select a date range |
| 空状态 | No data / No results / No results found / No traces found |
| 分页 | `Showing 1-10 of 42` → `显示第 1-10 条，共 42 条` |

**规模**：exact 114 条 + regex 74 条（原版 52 + 44）

**⚠️ 安全约定**：新增的 regex **全部用 `^...$` 锚定**，只匹配"整段就是这个词"的文本节点，
避免误改 prompt 正文、模型输出等**数据内容**。以后补词条请沿用这个约定。

---

## 安全性（已审源码）

- `permissions` 只有 `storage` —— 无网络、无全网访问
- 离线词典替换 DOM 文本，**不上传任何数据**
- 会跳过 `input / textarea / code / pre / script / style`，不翻输入框和代码块
- 源码里 2 处 `fetch` 属于**可选的"在线翻译"**功能，需自己填密钥，默认关闭

---

## 怎么装

1. Chrome 打开 `chrome://extensions`
2. 右上角开「**开发者模式**」
3. 点「**加载已解压的扩展程序**」→ 选这个目录
4. 打开 <http://localhost:3000> 并刷新

不生效时：扩展「详情」→「扩展选项」确认「启用翻译」已勾选；「域名白名单」留空 = 所有域名生效。

---

## 还有没翻到的怎么办

页面上按 F12 → Console 执行：

```js
copy(window.langfuseZhDump())
```

会复制一批页面上的英文片段。把它们加进 `translations/zh.js` 的 `exact` 或 `regex`，
回 `chrome://extensions` 点扩展卡片上的刷新图标，再刷新页面即可。
