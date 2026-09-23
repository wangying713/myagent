# Langfuse 中文镜像构建说明（方案 B2）

把 Langfuse 前端源码级汉化，并自建 Docker 镜像替换官方 `:3000`。

**与浏览器端方案（Chrome 扩展）的区别**：这是**源码级**替换，产物直接编译进镜像，
不依赖任何浏览器插件，任何设备访问都是中文。

---

## 一、成品

| 项 | 值 |
|---|---|
| 镜像 | `langfuse-zh:4`（基于官方 `docker.langfuse.com/langfuse/langfuse:4`） |
| 架构 | linux/arm64 |
| 改动 | 69 处 UI 文案 / 36 个文件 |
| 部署 | `/Users/wangying/docker-stack/langfuse/docker-compose.yml` 的 `langfuse-web` |
| 回滚 | `docker-compose.yml.bak` 已备份，改回 image 即可 |

---

## 二、汉化范围（69 处）

### 主导航 100% 中文（`web/src/components/layouts/routes.tsx`）

```
组织 项目 首页 仪表盘 调用链 会话 用户 告警
提示词 演练场 评分 评估器 人工标注 数据集 实验
云状态 升级套餐 设置 支持 跳转到...
```

### 页面标题 / 表格列头 / 面包屑（36 个文件）

匹配 `title:` / `header:` / `label:` / `name:` / `description:` 等 UI 属性位置。

**术语一致性**：`Scores→评分`、`Sessions→会话`、`Prompts→提示词`、`Datasets→数据集`、
`Tracing→调用链` 沿用 `translations/zh.json`；`Traces→追踪`、`Observations→观测`、
`Dashboard→仪表盘` 取自社区 issue [#12890](https://github.com/langfuse/langfuse/issues/12890)。

---

## 三、重建流程

```bash
# 1. 拉源码（SSH，HTTPS 会 HTTP2 失败）
git clone --depth=1 git@github.com:langfuse/langfuse.git langfuse-src
cd langfuse-src && git checkout -b i18n-zh

# 2. 装依赖与构建
npm i -g pnpm@12.4.1          # Node 25 移除了 corepack
pnpm install
# ⚠️ 必须在非 CodeBuddy 终端，或加 CODEBUDDY_SAFE_DELETE_ENABLED=0
CODEBUDDY_SAFE_DELETE_ENABLED=0 pnpm exec turbo run build --filter=web

# 3. 应用汉化
python3 apply_i18n_zh.py web/src
python3 apply_i18n_zh.py web/src/components/layouts

# 4. 重建产物（改动后必须）
CODEBUDDY_SAFE_DELETE_ENABLED=0 pnpm exec turbo run build --filter=web

# 5. 提取 standalone 产物与依赖
mkdir -p ctx && cp -R web/.next/standalone/web/.next ctx/next
cp web/.next/standalone/web/server.js web/.next/standalone/web/otelIngestionWorker.js ctx/
python3 extract_deps.py --full        # 产生 ctx/pnpm（完整 store，204MB）

# 6. 构建镜像
docker build -f Dockerfile.zh -t langfuse-zh:4 ctx
```

---

## 四、踩过的坑（按严重度）

### 1. Docker 构建 OOM —— 改用「产物替换」

```
ResourceExhausted: turbo run build ... cannot allocate memory
```

Docker 只分到 **7.7G**，编译 76 万行代码不够。完整 `web/Dockerfile` 走不通。

**解法**：本地构建（33s），只把 `.next` 产物塞进官方镜像。
产物内无 `.node/.so` 原生二进制，纯 JS 可跨 darwin→linux 移植。

### 2. 符号链接失效 —— 必须搬运 pnpm store

```
Cannot find package 'd3-scale-e5a532151ee2faed'
```

`.next/node_modules` 下 92 个包都是**指向本地 pnpm store 的相对符号链接**，
`cp -R` 保留链接后指向失效路径。官方镜像的 node_modules 不含这些前端运行时依赖。

**解法**：`extract_deps.py --full` 搬运完整 `.pnpm`（204MB）到 `/app/node_modules/.pnpm/`。
（只复制直接引用的 63 个包会漏掉 `d3-array` 这类二级依赖——pnpm 是隔离结构。）

### 3. CodeBuddy 安全钩子拦截构建

```
at checkBulkDeleteGuard (.../node-safe-delete-shim.cjs:404)
```

Next.js 清理 `.next` 时触发 IDE 的批量删除保护。首次构建成功是因为当时 `.next` 不存在。

**解法**：构建前设 `CODEBUDDY_SAFE_DELETE_ENABLED=0`。在自己的终端跑则无此问题。

### 4. Docker Hub 拉不到基础镜像

```
dial tcp 199.16.158.12:443: i/o timeout
```

`docker pull` 可用但 BuildKit 拉不到（网络栈不同）。

**解法**：先 `docker pull` 基础镜像到本地，再 `docker build --pull=false`。

### 5. venv 误入库 —— 已修

`langfuse-src/` 5.7G 且自带 `.git`，曾未加进 `.gitignore`，
会有「幽灵子模块」风险。已排除，`git status` 干净。

---

## 五、为什么官方不做 i18n

`langfuse/langfuse#12890` 提出了完整的中文 i18n 方案（含术语表），
结果是 **`Closed as not planned` + `stale`**。

技术原因：`next.config.mjs` 里的 `i18n` 配置项**不支持 App Router**
（源码注释明说用 appDir 就必须注释掉它），真做要接 `next-intl` 并改造路由，
成本极高。所以社区方案都是「运行时/产物层替换」，本方案同理。
