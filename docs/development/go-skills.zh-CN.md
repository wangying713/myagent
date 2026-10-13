# Go 技能候选：中文说明与比较

这里的技能指 AI 编码助手使用的 `SKILL.md` 指令资料，帮助助手在特定任务中遵循稳定的做法。

## 一、找到的来源

| 来源 | 提供什么 | 与本次需求的关系 |
| --- | --- | --- |
| [Google 官方技能仓库](https://github.com/google/skills) | Google 产品和技术的技能，主要覆盖云服务、开发工具等 | 查阅其技能目录，没有看到通用 Google Go 风格规范技能；其中的 Genkit Go 属于特定框架 |
| [Samuel Berthe 的 Go 技能集](https://github.com/samber/cc-skills-golang) | 按风格、命名、错误、测试、上下文、并发等主题拆分 | 适合按需挑选，但其中一些规则是社区作者的选择 |
| [Jeff Allan 的 Go 综合技能](https://github.com/Jeffallan/claude-skills/tree/main/skills/golang-pro) | 综合处理并发、接口、泛型、测试和项目组织 | 一份技能覆盖面较广，部分约束较强，需要结合项目筛选 |

Google 官方目录中的 [Genkit Go 技能](https://github.com/google/skills/tree/main/skills/cloud/developing-genkit-go)是框架专用资料。如果项目没有使用 Genkit，不必为一般 Go 开发引入它。

## 二、值得查看的社区技能

以下是候选的**中文用途概要**，没有把上游整套英文技能原样搬入项目：

| 原技能标识 | 中文用途 | 适合的任务 |
| --- | --- | --- |
| [golang-code-style](https://github.com/samber/cc-skills-golang/tree/8e899e20ff0cd4dc524af3993e4c62d8ee8c5717/skills/golang-code-style) | 代码风格 | 改善控制流、声明和函数的可读性 |
| [golang-naming](https://github.com/samber/cc-skills-golang/tree/8e899e20ff0cd4dc524af3993e4c62d8ee8c5717/skills/golang-naming) | 命名 | 为包、变量、类型、接口和测试选择名称 |
| [golang-error-handling](https://github.com/samber/cc-skills-golang/tree/8e899e20ff0cd4dc524af3993e4c62d8ee8c5717/skills/golang-error-handling) | 错误处理 | 错误包装、错误链、日志及恢复策略 |
| [golang-context](https://github.com/samber/cc-skills-golang/tree/8e899e20ff0cd4dc524af3993e4c62d8ee8c5717/skills/golang-context) | 请求上下文 | 取消、超时、跨层传递和请求相关信息 |
| [golang-concurrency](https://github.com/samber/cc-skills-golang/tree/8e899e20ff0cd4dc524af3993e4c62d8ee8c5717/skills/golang-concurrency) | 并发 | 协程退出、共享状态、通道和并发上限 |
| [golang-testing](https://github.com/samber/cc-skills-golang/tree/8e899e20ff0cd4dc524af3993e4c62d8ee8c5717/skills/golang-testing) | 测试 | 表驱动测试、隔离、竞态检查和覆盖缺口 |
| [golang-pro](https://github.com/Jeffallan/claude-skills/tree/1be15d8064f88fc25216442406d40add8fd23b53/skills/golang-pro) | Go 综合开发 | 需要同时处理接口、并发、性能和测试的功能 |

## 三、不能直接视为 Google 规范的地方

下表对照本次下载的上游版本，说明筛选中文草案时的重要差异：

| 社区技能中的表述 | Google 规范或实际适用边界 | 中文草案如何处理 |
| --- | --- | --- |
| 风格技能要求超过约 120 字符必须拆行，四个及以上实参必须分行 | Google 不设固定代码行宽，先考虑清晰度和重构 | 不设置数字门槛 |
| 风格技能要求切片和映射都不能是 nil | Google 对空切片优先使用 nil；nil 映射写入才需要初始化 | 区分切片与映射，按接口契约决定空值序列化 |
| 命名技能要求错误文本连缩写也全部小写 | Google 的首字母小写规则允许专有名称和缩写 | 保留例外；中文说明不套用英文大小写 |
| 命名技能强制布尔字段加前缀、原地操作加 `In` 后缀 | 这些不能当成 Google 的普遍硬性规则 | 以名称能否清楚表达含义为准 |
| 测试技能建议使用 testify | Google 倾向用 Go 条件判断和明确失败信息，避免断言框架 | 默认采用 `testing`；不新增断言库 |
| 综合技能要求至少 80% 覆盖率 | 覆盖率不能保证测试质量，Google 核心规范没有这个统一门槛 | 检查风险路径和行为契约，不设固定百分比 |
| 错误技能要求内部一律 `%w`、边界一律 `%v`，还推荐特定错误库 | 是否包装取决于公开错误链的契约；库选择也属于项目决策 | 明确错误链和日志责任，不强制额外库 |
| 并发技能要求通道不传指针、所有选择语句都监听取消 | 是否安全取决于所有权、阻塞和生命周期，不能仅看语法形式 | 检查数据是否被并发修改、阻塞是否能解除 |
| 多份技能要求按多个代理并行审查 | 这是上游的工作方式，不是 Go 语言规则 | 草案不要求创建多个代理 |
| 社区文件包含更新 Go 版本的 API 指南 | API 是否可用必须与项目工具链匹配 | 草案避免引入未经项目核对的新 API |

## 四、本地中文适配草案

为了方便阅读和选择，整理了四份中文技能。它们以 Google 文档为主要依据，结合候选技能的主题分类，重新组织为短指令；**属于中文适配草案，不是社区技能的完整逐句翻译，也不是官方技能发布物**。

1. [Google Go 风格评审](skills/google-go-style/SKILL.md)：格式、命名、控制流、接口和规范层级。
2. [Go 错误处理](skills/go-error-handling/SKILL.md)：错误链、错误信息、日志责任和恢复策略。
3. [Go 测试评审](skills/go-testing/SKILL.md)：有效断言、可读失败信息、隔离、并发和适度验证。
4. [Go 并发与上下文](skills/go-concurrency/SKILL.md)：退出、取消、等待、通道所有权和共享状态。

这些草案目前仅保存在 `docs/development/skills/` 供审阅。未安装技能，未修改项目级助手规则，未添加开发依赖。
