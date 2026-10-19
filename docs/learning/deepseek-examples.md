# DeepSeek 官方示例清单与课程依据

核对日期：2026-10-17。本文用于选课和查来源，可在管理台「Agent Demos」中阅读。13 章的最小演示按用户另行确认的范围实现，本文中其余官方主题仍只提供学习入口。

## 先说明课程怎么来的

之前的学习路线是结合本项目“用 Go 学 Agent”的目标整理的，没有逐项核对 DeepSeek 官方 demo。它不是 DeepSeek 官方发布的课程，也不能把所有章节都说成来自官方示例。

以后细化每章时，分清三件事：**官方演示了什么、课程补充了什么、练习由谁设计**。官方示例通常说明一种调用方式；Go 的数据校验、取消、错误处理和测试，再依据 [项目开发规范](../development/README.md) 补充。

本次“官方 demo”的清单范围是：官方 API 入门与使用指南中的示例、官方提示库，以及与 Agent 学习相关的官方仓库示例。下面按主题列，Python、curl、Node.js 等同一示例的不同语言写法合并，不把它们重复算成多门课。模型训练、部署和 GPU 推理属于另一条路线，放在后面的选读入口；本文不声称穷举官方组织所有仓库中的脚本、测试和历史版本。

## 一、API 官方示例

来源：[官方 API 文档](https://api-docs.deepseek.com/)。本表覆盖核对时使用指南导航中的全部主题，并补上首次调用；链接指向实际主题页，不使用已经跳回首页的旧路径。

| 编号 | 官方主题与链接 | 页面里的示例或演示内容 | 能学到什么 |
| --- | --- | --- | --- |
| A01 | [首次调用](https://api-docs.deepseek.com/) | curl、Python、Node.js 发起一次聊天请求 | 请求地址、消息、认证和回答分别在哪里 |
| A02 | [多轮对话](https://api-docs.deepseek.com/guides/multi_round_chat/) | 先问最高的山，再追问第二高的山；追加上一轮回答与新问题 | 每次请求怎样带上历史 |
| A03 | [工具调用](https://api-docs.deepseek.com/guides/tool_calls/) | 查询杭州天气、回传工具结果；思考模式入口；中途插入工具消息的协议差异；strict 模式工具定义与约束样例 | 模型提出调用，程序负责执行和回传 |
| A04 | [JSON 输出](https://api-docs.deepseek.com/guides/json_mode/) | 把一段问答提取成 question、answer 两个字段，再解析 JSON | 让回答变成程序能读取的数据 |
| A05 | [思考模式](https://api-docs.deepseek.com/guides/thinking_mode/) | 开关与强度设置；普通返回、流式返回的两轮问答；日期工具与天气工具配合 | 思考内容、最终回答和工具消息如何处理 |
| A06 | [对话前缀续写（Beta）](https://api-docs.deepseek.com/guides/chat_prefix_completion/) | 给定 Python 代码块开头，让模型继续生成快速排序代码 | 指定回答从哪里接着写 |
| A07 | [FIM 补全（Beta）](https://api-docs.deepseek.com/guides/fim_completion/) | 给出斐波那契函数的开头和结尾，让模型补中间；附 Continue 接入入口 | 根据前后文填补缺失内容 |
| A08 | [上下文缓存](https://api-docs.deepseek.com/guides/kv_cache/) | 多轮对话复用前缀；围绕同一份长文连续提问；查看缓存命中用量 | 重复输入怎样被复用，如何查看用量 |
| A09 | [视觉输入](https://api-docs.deepseek.com/guides/vision/) | Base64 图片、图片 URL、已上传图片的 file_id | 让请求同时带文字和图片 |
| A10 | [文件 API](https://api-docs.deepseek.com/guides/files_api/) | 上传、列出、查询和删除图片文件；聊天引用 file_id；Anthropic 兼容文件用法 | 图片文件如何上传和重复使用；不是任意文档解析示例 |
| A11 | [网页搜索与抓取](https://api-docs.deepseek.com/guides/search/) | Search 搜索请求、查询条件；Fetch 获取单页正文；curl 与 Python 写法 | 先找到网页，再取得内容；还不是完整知识库系统 |
| A12 | [Responses API](https://api-docs.deepseek.com/guides/responses_api/) | 普通调用、流式事件、图片输入、工具返回图片的消息片段；附协议兼容说明 | 同一个模型也有另一种请求和事件格式 |
| A13 | [Anthropic 兼容 API](https://api-docs.deepseek.com/guides/anthropic_api/) | 用 Anthropic Python SDK 发起消息请求；附字段兼容表和工具接入入口 | 兼容协议的地址、消息和字段怎样对应 |

A03 的 strict 部分还包含对象、字符串、数值、数组、枚举、anyOf 和引用/递归等 Schema 样例。它约束的是工具参数，不能直接当作 A04 的“所有业务字段都自动正确”。A04 的 JSON 能解析后，仍要由应用检查字段和值。

**普通流式回答也在学习范围内**：A01 说明 stream 开关，A05 提供流式处理代码，A12 展示另一种流式事件格式。三处可以对照阅读，不必为每处重复安排一课。

API Reference 还提供逐接口请求样例，可用于查完整字段，不另外算课程：

- [聊天请求](https://api-docs.deepseek.com/api/create-chat-completion)、[文本补全](https://api-docs.deepseek.com/api/create-completion)、[Responses 请求](https://api-docs.deepseek.com/api/create-response)。
- [上传文件](https://api-docs.deepseek.com/api/create-file)、[列出文件](https://api-docs.deepseek.com/api/list-files)、[文件信息](https://api-docs.deepseek.com/api/retrieve-file)、[删除文件](https://api-docs.deepseek.com/api/delete-file)。
- [抓取网页](https://api-docs.deepseek.com/api/fetch)，搜索的请求和响应入口见 A11。
- [模型列表](https://api-docs.deepseek.com/api/list-models)、[账户余额](https://api-docs.deepseek.com/api/get-user-balance)。

## 二、官方提示库：13 项样例

来源：[官方中文提示库](https://api-docs.deepseek.com/zh-cn/prompt-library/)。下面按本次官方索引检索可见的条目列全。这些是提示词样例，不是 13 个完整 Agent 项目。

访问说明：本次检索能读取以下条目，但直接抓取页面时仅返回提示库标题，未取得各条目的独立详情链接。这里保留官方索引入口，不编造详情地址，也不把所有条目说成已逐一运行验证。

| 编号 | 官方条目 | 可以怎么练习 |
| --- | --- | --- |
| P01 | 代码改写 | 提出一个明确的修改要求，再检查改动是否符合要求 |
| P02 | 代码解释 | 让模型用白话说明输入、处理过程和结果 |
| P03 | 代码生成 | 描述一个小功能，检查返回代码是否实现它 |
| P04 | 内容分类 | 先限定几个类别，再判断文本属于哪类 |
| P05 | 结构化输出 | 把文字整理成约定字段，交给程序解析 |
| P06 | 角色扮演（自定义人设） | 观察身份与说话方式的约束有什么作用 |
| P07 | 角色扮演（情景续写） | 给定情境，观察对话怎样接下去 |
| P08 | 散文写作 | 对照主题、长度和表达要求检查结果 |
| P09 | 诗歌创作 | 尝试给创作任务增加格式限制 |
| P10 | 文案大纲生成 | 先要一个提纲，再分部分展开 |
| P11 | 宣传标语生成 | 根据商品特点生成候选句，再人工选择 |
| P12 | 模型提示词生成 | 把模糊需求整理成可检查的要求 |
| P13 | 中英翻译专家 | 对照原文检查含义、专名和格式 |

这些样例适合练习“怎样把要求说清楚”。单次分类、翻译、写作本身通常只需要一次模型调用；是否需要 Agent，要看任务是否还需要查资料、调用工具或多步执行。

## 三、官方仓库里的 Agent 与协议示例

这里仅收录 deepseek-ai 官方组织的来源。仓库会调整目录，后续展开教程时应记录使用的 tag 或 commit；下面是本次能核对到的入口，没有下载、安装或执行这些 demo。

### DeepSeek Harness

[官方仓库及中文说明](https://github.com/deepseek-ai/deepseek-harness/blob/master/README.zh.md)介绍的是官方 Agent 框架，主要使用 TypeScript，并提供 Python SDK。可先读思路；本项目没有因此改用该框架。

| 示例入口 | 主要看什么 | 课程位置 |
| --- | --- | --- |
| [Python SDK 最小示例](https://github.com/deepseek-ai/deepseek-harness/blob/master/python/sdk/examples/README.md) | minimal.py 怎样连接 Agent 运行环境并提交任务 | 工具循环之后选读 |
| [Cordis 插件教程](https://github.com/deepseek-ai/deepseek-harness/blob/master/docs/cordis-tutorial/index.md) | 插件、生命周期、服务、事件、配置、组合与热更新、接入 Harness，共 7 个循序练习 | 工作流与扩展方式选读 |
| [cordis 配置示例](https://github.com/deepseek-ai/deepseek-harness/tree/master/apps/cli/config/examples/cordis) | 查看插件相关的配置组合 | 扩展方式选读 |
| [github-review 配置示例](https://github.com/deepseek-ai/deepseek-harness/tree/master/apps/cli/config/examples/github-review) | 查看代码评审场景的配置 | 综合练习选读 |
| [mcp-memory 配置示例](https://github.com/deepseek-ai/deepseek-harness/tree/master/apps/cli/config/examples/mcp-memory) | 查看外部记忆能力的接入配置 | 检索、记忆与状态选读 |
| [schedule 配置示例](https://github.com/deepseek-ai/deepseek-harness/tree/master/apps/cli/config/examples/schedule) | 查看定时任务相关配置 | 工作流选读 |

配置示例的目录依据：[当前 examples 目录](https://github.com/deepseek-ai/deepseek-harness/tree/master/apps/cli/config/examples)。网上旧资料中的顶层 examples、packages/examples 路径本次未能稳定读取，不将旧目录说明当作当前可运行步骤。

### deepseek-recipe

[官方仓库](https://github.com/deepseek-ai/deepseek-recipe)主要讲请求格式转换、提示内容编码和模型输出解析，使用 Rust 与 Python。它不是“拿来就能执行任务”的完整 Agent。

| 示例入口 | 主要看什么 |
| --- | --- |
| [README 的 Python 与 Rust 入门示例](https://github.com/deepseek-ai/deepseek-recipe#using-deepseek-recipe) | 把请求转换为内部对话，再生成模型接收的提示内容 |
| [encoding-decoding-demo](https://github.com/deepseek-ai/deepseek-recipe/tree/main/encoding-decoding-demo) | 在网页中观察编码、特殊标记与输出解码 |
| [server-py](https://github.com/deepseek-ai/deepseek-recipe/tree/main/server-py) | Python/FastAPI 模拟推理服务，观察不同 API 格式 |
| [server-rs](https://github.com/deepseek-ai/deepseek-recipe/tree/main/server-rs) | Rust/Axum 版本的模拟推理服务 |
| [流式转换示例](https://github.com/deepseek-ai/deepseek-recipe/blob/main/docs/streaming.md) | 把输出逐段转换成 API 流式响应 |
| [Tokenizer 用法](https://github.com/deepseek-ai/deepseek-recipe/blob/main/docs/tokenizer.md) | 对话内容与 token ID 的编码、解码；以仓库 README 的最新入口为准 |

两个 server 是模拟推理示例；编码解码演示也不替你执行工具。它们适合在理解 API 后，选读“服务背后怎样转换数据”，不要求初学者先学 Rust 或模型部署。

## 四、官方模型演示选读

这些与“调用在线 API 写 Go Agent”不是同一条学习路线，仅作为补充入口；不是官方模型仓库的全量清单。

| 官方仓库 | 演示方向 |
| --- | --- |
| [DeepSeek-Coder](https://github.com/deepseek-ai/DeepSeek-Coder) | 代码模型的本地使用及 demo 目录 |
| [DeepSeek-VL](https://github.com/deepseek-ai/DeepSeek-VL) | 图文理解模型的本地推理与演示 |
| [DeepSeek-VL2](https://github.com/deepseek-ai/DeepSeek-VL2) | 多模态理解与网页演示 |
| [Janus](https://github.com/deepseek-ai/Janus) | 多模态理解与图像生成的演示 |
| [DeepSeek-OCR](https://github.com/deepseek-ai/DeepSeek-OCR) | 图片、文档图像相关的识别与推理示例 |

其他仓库从 [官方 GitHub 组织](https://github.com/deepseek-ai)继续查找。这里没有把旧模型演示等同于当前在线 API 的能力。

## 五、官方整理的第三方接入资料

[awesome-deepseek-integration](https://github.com/deepseek-ai/awesome-deepseek-integration) 是官方组织维护的接入资料清单，其中的第三方项目不因此变成 DeepSeek 官方开发的 demo。

本次 API 文档导航还列出 Claude Code、Codex、OpenCode、OpenClaw、Hermes、Reasonix、WorkBuddy/CodeBuddy、Qoder 的接入说明，入口见 [API 首页的 Agent Integrations](https://api-docs.deepseek.com/)。它们主要解决“现有工具怎么接 DeepSeek”，不直接代替“自己用 Go 写 Agent”的课程。

## 六、与现有课程逐项对应

| 现有章节 | 官方依据 | 本项目补充的内容 |
| --- | --- | --- |
| [Agent 到底是什么](00-overview.md) | A03、A05 中模型与工具的交互 | 用白话区分模型、程序、工具及任务循环 |
| [普通聊天](01-chat.md) | A01；P01～P13 可作为单次请求练习 | Go 请求和响应的阅读方法 |
| [消息历史](02-messages.md) | A02、A08 | 历史保存、会话隔离和输入长度控制 |
| [工具循环](03-agent.md) | A03、A05 | 参数检查、工具白名单与有限循环 |
| [结构化输出](04-json.md) | A04、P05；A03 strict 用于对照 | Go 解码、字段检查和业务值校验 |
| [多个工具协作](multiple-tools.md) | A05 的日期与天气工具 | 查价格再计算等本项目练习 |
| [流式回答与生成模式](05-stream-thinking.md) | A01、A05、A12 | Go 分段读取、用户停止和请求取消 |
| [失败、预算与执行边界](06-reliability.md) | 官方 [错误码](https://api-docs.deepseek.com/quick_start/error_codes/)、[Token 用量](https://api-docs.deepseek.com/quick_start/token_usage/)、[限流说明](https://api-docs.deepseek.com/quick_start/rate_limit/) | 超时、重试、预算及工具执行边界；属于工程补充 |
| [检索、记忆与会话状态](07-retrieval-memory.md) | A11；Harness mcp-memory 配置可选读 | 资料切分、检索、引用和记忆管理；没有把搜索接口说成完整 RAG demo |
| [任务拆分与工作流](09-workflows.md) | A05 可观察连续工具调用；Harness 可选读 | 固定步骤与动态循环的取舍；没有对应的单一官方课程 |
| [日志与链路观测](openobserve.md) | 本清单没有对应的 DeepSeek 专用 demo | Go 日志与 OpenObserve 观测实践 |
| [评估与回归](08-evaluation.md) | 本清单没有对应的完整应用评估 demo | 固定题集、离线样本及回归检查 |
| [综合练习与交付](10-delivery.md) | 综合前面已学的官方用法 | 项目组织、服务交付及检查方法 |

原路线没有单独展开的官方主题：**前缀续写、FIM、缓存、图片与文件、网页搜索接口、不同 API 格式、Harness 与 recipe**。本次补齐学习入口，后续由你安排是否展开，不自动增加实现任务。

## 以后逐章细化的要求

1. 开头写明对应的官方示例编号和链接；没有直接对应示例，就写“工程补充”。
2. 先用简单例子说清楚，再讲请求、响应和流程；Go 示例是课程改写时，明确标注。
3. 本项目设计的案例写明“本项目练习”，不冒充官方案例。
4. 展开时重新核对模型、参数、Beta 状态和仓库版本。旧链接能打开首页，不代表原示例仍在。
5. 官方最小示例没有展示的校验、取消和测试，单独解释补充原因，不把这些都归因于官方要求。

[返回完整学习路线](README.md)。
