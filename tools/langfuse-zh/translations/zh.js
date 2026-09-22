export default {
  exact: {
    "Project Settings": "项目设置",
    "LLM Connections": "LLM连接",
    "Models": "模型",
    "Scores": "评分",
    "Evaluation": "评测",
    "Playground": "演练场",
    "Observability": "可观测性",
    "Prompt Management": "提示管理",
    "Tracing": "调用链",
    "Sessions": "会话",
    "Users": "用户",
    "Prompts": "提示词",
    "LLM-as-a-Judge": "LLM 评判",
    "Human Annotation": "人工标注",
    "Datasets": "数据集",
    "Integrations": "集成",
    "Members": "成员",
    "Exports": "导出",
    "Audit Logs": "审计日志",
    "Notifications": "通知",
    "General": "通用",
    "API Keys": "API 密钥",
    "Add model definition": "添加模型定义",
    "Clone": "克隆",
    "Save": "保存",
    "Cancel": "取消",
    "Project Not Found": "未找到项目或无权限",
    "A model represents a LLM model. It is used to calculate tokens and cost.": "模型用于计量 token 与成本。",
    "Row height": "行高",
    "Columns": "列",
    "Go to Home": "返回首页",
    "Support": "支持",
    "Toggle Sidebar": "切换侧边栏",
    "Star Langfuse": "为 Langfuse 加星",
    "See the latest releases and help grow the community on GitHub": "在 GitHub 查看最新发布并支持社区"
    ,"Organizations": "组织",
    "New Organization": "新建组织",
    "New project": "新建项目",
    "Go to project": "进入项目",
    "Search projects": "搜索项目",
    "Projects": "项目",
    "Dashboard": "仪表盘",
    "Generations": "生成",
    "Settings": "设置",
    "Docs": "文档",
    "Theme": "主题",
    "Sign out": "退出登录",
    "CH Query": "ClickHouse 查询",
    "Loading": "加载中",
    "Redirecting": "跳转中"
    ,"Get Started with": "开始使用",
    "Create Prompt": "创建提示词",
    "Learn More": "了解更多",
    "Decoupled from code": "与代码解耦",
    "Edit in UI or programmatically": "在界面或以编程方式编辑",
    "Performance optimized": "性能优化",
    "Compare metrics": "对比指标",
    "Automations": "自动化",
    "New prompt": "新建提示词",
    "Tools": "工具",
    "Schema": "模式",
    "Variables": "变量",
    "System": "系统",
    "User": "用户",
    "Message": "消息",
    "Placeholder": "占位符",
    "Save as prompt": "保存为提示词",
    "New split window": "新建分屏窗口",
    "Reset playground": "重置演练场",
    "Run All (Ctrl + Enter)": "全部运行 (Ctrl + Enter)",
    "1 window": "1 个窗口"
    ,"Show filters": "显示筛选",
    "Search...": "搜索...",
    "Names, Tags": "名称、标签",
    "Name": "名称",
    "Versions": "版本",
    "Type": "类型",
    "Latest Version": "最新版本",
    "Created At": "创建时间",
    "Number of Observations": "观测数",
    "Tags": "标签",
    "Model": "模型",
    "No LLM API key set in project.": "项目未设置 LLM API 密钥。",
    "Add LLM Connection": "添加 LLM 连接",
    "Delete message": "删除消息",
    "Output": "输出",
    "Submit": "提交"
    ,"Developer": "开发者",
    "Assistant": "助手"

    // ↓↓↓ 本地补充：调用链 / 观测页与通用界面术语 ↓↓↓
    ,"Traces": "调用链"
    ,"Trace Detail": "调用链详情"
    ,"Trace Overview": "调用链概览"
    ,"Observations": "观测项"
    ,"Metadata": "元数据"
    ,"Details": "详情"
    ,"Input": "输入"
    ,"Environment": "环境"
    ,"Level": "级别"
    ,"Filters": "筛选"
    ,"Time Range": "时间范围"
    ,"Apply": "应用"
    ,"Clear": "清空"
    ,"Reset": "重置"
    ,"Refresh": "刷新"
    ,"Export": "导出"
    ,"Delete": "删除"
    ,"Copy": "复制"
    ,"Share": "分享"
    ,"Download": "下载"
    ,"No data": "无数据"
    ,"No results": "无结果"
    ,"No results found": "未找到结果"
    ,"No traces found": "未找到调用链"
    ,"Search": "搜索"
  },
  regex: [
    { pattern: "^Last used$", flags: "i", replace: "最近使用" },
    { pattern: "^Maintainer$", flags: "i", replace: "维护者" },
    { pattern: "^Tokenizer Configuration$", flags: "i", replace: "分词器配置" },
    { pattern: "^Prices per unit$", flags: "i", replace: "单价" },
    { pattern: "^Rows per page$", flags: "i", replace: "每页行数" },
    { pattern: "^Go to next page$", flags: "i", replace: "下一页" },
    { pattern: "^Go to previous page$", flags: "i", replace: "上一页" },
    { pattern: "^Go to first page$", flags: "i", replace: "首页" },
    { pattern: "^Go to last page$", flags: "i", replace: "末页" },
    { pattern: "^Columns\\s+(\\d+\/\\d+)$", flags: "i", replace: "列 $1" },
    { pattern: "^Model Name$", flags: "i", replace: "模型名称" },
    { pattern: "^Match Pattern$", flags: "i", replace: "匹配模式" },
    { pattern: "^Tokenizer$", flags: "i", replace: "分词器" },
    { pattern: "^Actions$", flags: "i", replace: "操作" },
    { pattern: "^Page\\s+(\\d+)\\s+of$", flags: "i", replace: "第 $1 页" },
    { pattern: "\\bGeneral\\b", flags: "gi", replace: "通用" },
    { pattern: "\\bAPI Keys\\b", flags: "gi", replace: "API 密钥" },
    { pattern: "\\bLLM Connections\\b", flags: "gi", replace: "LLM连接" },
    { pattern: "\\bModels\\b", flags: "gi", replace: "模型" },
    { pattern: "\\bScores\\b", flags: "gi", replace: "评分" },
    { pattern: "\\bEvaluation\\b", flags: "gi", replace: "评测" },
    { pattern: "\\bMembers\\b", flags: "gi", replace: "成员" },
    { pattern: "\\bIntegrations\\b", flags: "gi", replace: "集成" },
    { pattern: "\\bExports\\b", flags: "gi", replace: "导出" },
    { pattern: "\\bAudit Logs\\b", flags: "gi", replace: "审计日志" },
    { pattern: "\\bNotifications\\b", flags: "gi", replace: "通知" },
    { pattern: "\\bObservability\\b", flags: "gi", replace: "可观测性" },
    { pattern: "\\bPrompt Management\\b", flags: "gi", replace: "提示管理" },
    { pattern: "\\bTracing\\b", flags: "gi", replace: "调用链" },
    { pattern: "\\bSessions\\b", flags: "gi", replace: "会话" },
    { pattern: "\\bUsers\\b", flags: "gi", replace: "用户" },
    { pattern: "\\bPrompts\\b", flags: "gi", replace: "提示词" },
    { pattern: "\\bLLM-as-a-Judge\\b", flags: "gi", replace: "LLM 评判" },
    { pattern: "\\bHuman Annotation\\b", flags: "gi", replace: "人工标注" },
    { pattern: "\\bDatasets\\b", flags: "gi", replace: "数据集" },
    { pattern: "^Loading\\.\\.\\.$", flags: "i", replace: "正在加载..." },
    { pattern: "^(\\d+) window(s?)$", flags: "i", replace: "$1 个窗口" },
    { pattern: "^Latest Version\\s+Created At\\s+▼$", flags: "i", replace: "最新版本 创建时间 ▼" },
    { pattern: "^Search\\.\\.\\.$", flags: "i", replace: "搜索..." },
    { pattern: "^Get Started with\\s+.*$", flags: "i", replace: "开始使用 提示管理" },
    { pattern: "^Langfuse\\s+.*helps you centrally manage, version control, and collaboratively iterate on your.*$", flags: "i", replace: "Langfuse 提示管理帮助你集中管理、进行版本控制并协作迭代提示词" },
    { pattern: "^Start using\\s+.*to improve your LLM application's performance and maintainability\.$", flags: "i", replace: "开始使用 提示管理 以提升你的 LLM 应用的性能与可维护性。" },
    { pattern: "^Deploy new\\s+.*without application redeployment, making updates faster and easier$", flags: "i", replace: "无需重新部署应用即可发布新的提示词，使更新更快更简单" },
    { pattern: "^Non-technical\\s+.*can easily edit\\s+.*in the UI\.$", flags: "i", replace: "非技术用户可在界面轻松编辑提示词。" },
    { pattern: "^Developers can optionally update\\s+.*programmatically via the API and SDKs\.$", flags: "i", replace: "开发者可通过 API 与 SDK 以编程方式更新提示词。" },
    { pattern: "^Client-side caching prevents latency or availability issues for your applications$", flags: "i", replace: "客户端缓存可避免应用的延迟或可用性问题" },
    { pattern: "^Track latency, cost, and\\s+.*metrics across different prompt versions$", flags: "i", replace: "跟踪不同提示词版本的延迟、成本和评测指标" },
    { pattern: "^Enter a system message here\\.$", flags: "i", replace: "在此输入系统消息。" },
    { pattern: "^Enter a user message here\\.$", flags: "i", replace: "在此输入用户消息。" },
    { pattern: "^Enter a developer message here\\.$", flags: "i", replace: "在此输入开发者消息。" },
    { pattern: "^Enter an assistant message here\\.$", flags: "i", replace: "在此输入助手消息。" }

    // ↓↓↓ 本地补充：调用链 / 观测页表头、类型徽标与分页 ↓↓↓
    // 全部用 ^...$ 锚定，只匹配"整段就是这个词"的文本节点，避免误改数据内容
    ,{ pattern: "^Timestamp$", flags: "i", replace: "时间" }
    ,{ pattern: "^Trace Name$", flags: "i", replace: "调用链名称" }
    ,{ pattern: "^Trace ID$", flags: "i", replace: "调用链 ID" }
    ,{ pattern: "^Observation ID$", flags: "i", replace: "观测项 ID" }
    ,{ pattern: "^Latency$", flags: "i", replace: "延迟" }
    ,{ pattern: "^Total Tokens$", flags: "i", replace: "总 token" }
    ,{ pattern: "^Input Tokens$", flags: "i", replace: "输入 token" }
    ,{ pattern: "^Output Tokens$", flags: "i", replace: "输出 token" }
    ,{ pattern: "^Total Cost$", flags: "i", replace: "总成本" }
    ,{ pattern: "^Input Cost$", flags: "i", replace: "输入成本" }
    ,{ pattern: "^Output Cost$", flags: "i", replace: "输出成本" }
    ,{ pattern: "^User ID$", flags: "i", replace: "用户 ID" }
    ,{ pattern: "^Session ID$", flags: "i", replace: "会话 ID" }
    ,{ pattern: "^Status Message$", flags: "i", replace: "状态信息" }
    ,{ pattern: "^Release$", flags: "i", replace: "发布版本" }
    ,{ pattern: "^Add Filter$", flags: "i", replace: "添加筛选" }
    ,{ pattern: "^Select a date range$", flags: "i", replace: "选择日期范围" }
    ,{ pattern: "^Showing\\s+(\\d+)\\s*-\\s*(\\d+)\\s+of\\s+(\\d+)$", flags: "i", replace: "显示第 $1-$2 条，共 $3 条" }
    ,{ pattern: "^GENERATION$", replace: "生成" }
    ,{ pattern: "^SPAN$", replace: "步骤" }
    ,{ pattern: "^EVENT$", replace: "事件" }
    ,{ pattern: "^TOOL$", replace: "工具" }
    ,{ pattern: "^TRACE$", replace: "调用链" }
  ]
}
