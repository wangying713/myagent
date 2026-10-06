# 中级：让 Agent 使用资料，并处理真实程序的边界

**目标：**从一次演示走向可验证的小应用。先完成初级，理解消息和工具配对。本级 5 个 demo 均已提供完整代码；可靠性实验完全离线，其余使用已有模型配置。

## I01：结构化输出与业务校验

- 文档：[结构化输出](../lessons/04-json.md)。
- 运行：`uv run python -m demos.intermediate.intermediate_01_json`。
- 阅读：`parse_order()`，正常一次模型请求、2 条 span。

将“三本笔记本”改为“一百零一本笔记本”，观察模型虽然可以输出合法 JSON，业务仍应拒绝越界数量。再离线传入 bool 类型的 quantity，理解类型检查。

**通过标准：**能分别指出 JSON 语法错误、字段错误、业务错误。不要把 `json.loads()` 成功当作业务允许执行。

## I02：流式输出

- 文档：[流式与思考模式](../lessons/05-stream-thinking.md) 的流式部分。
- 运行：`uv run python -m demos.intermediate.intermediate_02_stream`。
- 阅读：`llm.py` 的 `chat_stream()`，正常一条模型 span，不逐片段上报。

比较终端逐段显示与 trace 中聚合正文。检查 `app.stream.complete`。测试中的中断流会保存部分正文但报告失败；流中的工具参数也必须拼完整后才能执行。

**通过标准：**能解释“已经显示半句话”为什么不等于请求完成，能说明片段数不能用来计算 token。

## I03：有限重试与故障实验

- 文档：[失败、预算和安全](../lessons/06-reliability.md)。
- 运行：`uv run python -m demos.intermediate.intermediate_03_reliability`。
- 阅读：`retry_chat()` → `offline_lab.py`。

本实验不需要密钥，不访问 DeepSeek，也不向 OpenObserve 写入。第一种场景模拟 503 后恢复，应出现 2 次请求、3 个内存 span；第二种模拟 401，只尝试一次。离线 trace 与真实上报使用相同的观测代码，仅 exporter 换成内存。

这里显式展示最多 3 次尝试和退避。没有给所有请求偷偷加重试，也没有对工具副作用做重放。退避示例未实现生产级抖动或 `Retry-After` 处理。

**通过标准：**能解释哪些错误应改配置、哪些可重试；能修改假响应让三次都失败；能证明每次实际尝试都有记录。

## I04：最小 RAG 与引用检查

- 文档：[检索、记忆与状态](../lessons/07-retrieval-memory.md)。
- 运行：`uv run python -m demos.intermediate.intermediate_04_rag`。
- 阅读：`data/knowledge.json` → `search_documents()` → `answer_question()` → `validate_answer()`。

知识库是三条虚构产品资料；检索使用关键词计分，没有向量数据库。程序固定执行一次搜索，再把命中片段放进 user 消息，最后让模型输出带来源 ID 的 JSON。常规运行共 3 个 span：根、检索、模型。

```bash
uv run python -m demos.intermediate.intermediate_04_rag '星河笔记支持导出哪些格式？'
uv run python -m demos.intermediate.intermediate_04_rag '是否支持脑机接口？'
```

后一题没有关键词命中，程序直接返回依据不足：0 次模型请求，仍有根与检索两条 span。`local-retrieval` 是本地步骤标识，不是伪造模型 tool call 插入 API 历史；这里是固定检索流程。

引用校验拒绝未提供的来源 ID，但不能证明答案忠实于来源。关键词未命中也可能只是查询表达不同，不能证明整个知识库没有答案。这正是后续语义检索、检索评估要解决的问题。

**通过标准：**能分别检查检索命中与答案依据；添加第四条虚构资料并验证；能让凭空生成的来源 ID 被程序拒绝。

## I05：SQLite 会话持久化

- 运行：`uv run python -m demos.intermediate.intermediate_05_memory`。
- 阅读：`demos/intermediate/intermediate_05_memory.py` → `session_store.py`。

默认演示使用临时数据库：写入 Alice 的项目代号，重新创建存储对象后读取，再询问代号；检查 Bob 的历史为空。正常 2 次模型请求、3 条 span。默认退出会删除临时库，避免积累练习数据。

跨两个进程实验：

```bash
mkdir -p .local
uv run python -m demos.intermediate.intermediate_05_memory --db .local/sessions.sqlite3 --session alice --ask '请记住我的项目代号是海盐。'
uv run python -m demos.intermediate.intermediate_05_memory --db .local/sessions.sqlite3 --session alice --ask '我的项目代号是什么？'
uv run python -m demos.intermediate.intermediate_05_memory --db .local/sessions.sqlite3 --session bob --ask '我的项目代号是什么？'
```

`.local/` 已被 Git 忽略。会话内容作为业务数据保存在本机，不等于观测副本的脱敏内容。使用虚构资料即可。

存储使用 revision 检查：两个请求读取同一版本时，第二个提交者不能悄悄覆盖第一个的新历史。冲突会明确报错，不自动重复付费模型调用。会话 ID 隔离只是存储隔离，真实多用户系统还必须验证用户身份及其访问权限。

**通过标准：**能证明重建连接后状态仍存在、不同 ID 不串线、失败请求不保存半轮对话、旧 revision 不能覆盖新状态。

## 中级验收

```bash
uv run python -m demos.intermediate.intermediate_03_reliability
uv run python verify_live.py --level intermediate
uv run python -m unittest discover -s tests -v
```

真实验收通常共 5 次模型请求，覆盖 JSON、流式、检索命中、无资料和会话恢复。离线测试覆盖重试、引用伪造、并发覆盖与失败保存等边界。

下一阶段：[高级](advanced.md)。
