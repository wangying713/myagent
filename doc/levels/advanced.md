# 高级：恢复、评估与综合项目

**目标：**能说明一个 Agent 为什么可信、失败后如何恢复、改动是否有效。本级 4 个 demo 都有完整代码。这里的高级是单机工程与综合实践，不代表已经实现生产部署、分布式一致性或完整多用户权限。

## A01：思考模式与资源消耗

- 文档：[流式与思考模式](../lessons/05-stream-thinking.md) 的思考部分。
- 运行：`uv run python -m demos.advanced.advanced_01_thinking`。
- 阅读：`solve()`，通常 1 次真实请求、2 个 span。

题目是“十位比个位大 3，数位和 11”，正确结果为 74。先检查最终答案，再看 token 与耗时。终端只显示最终答案；服务端提供的 `reasoning_content` 仍保存在脱敏、限长的响应中。

关闭思考模式重复同一题，比较答案和资源消耗。推理文本更长不能代替答案验算。一次题目通过也不证明思考模式在所有任务上更好。

**通过标准：**能区分思考开关与流式开关，能解释 max_tokens 太小时为什么可能没有最终答案。思考模式配合工具需要额外遵循回传规则，本课程的真实高级验收只验证单次思考调用。

## A02：审批状态、幂等与崩溃恢复

- 运行：`uv run python -m demos.advanced.advanced_02_recovery`。
- 阅读：`LocalJobs.plan()` → `execute()` → demo 中的重新打开数据库。
- 完全离线：只在临时 SQLite 创建演示工单，无模型调用、无外发行为。

实验分三步：未批准时拒绝执行；模拟批准后提交本地事务；模拟程序未展示结果就退出，然后重新打开数据库重放同一任务。最终应只有一张工单。测试还覆盖“提交前异常”，确保动作与状态一同回滚。

幂等键不仅是字符串：同一个 job ID 必须绑定相同内容。把它用于不同工单标题会被拒绝。`approved=True` 是模拟审批输入，不是可信的权限系统；真实审批要绑定审阅者、具体操作与版本。

本例把动作和 done 状态放在同一个 SQLite 事务里，所以能保证该本地动作不会重复。远程扣款、发邮件与本地数据库不在同一个事务里，不能照搬这个保证；需要远端幂等键、结果查询、outbox 或人工处理不确定状态。

**通过标准：**能解释提交前崩溃、提交后响应丢失的区别；能证明未批准不产生工单、重放不会生成第二张、修改操作内容不能复用原 ID。

## A03：固定用例评估

- 文档：[评估与交付](../lessons/08-evaluation.md)。
- 运行：`uv run python -m demos.advanced.advanced_03_evaluation`。
- 阅读：`data/eval_cases.json` → `evaluate()` → `grade()`。

默认使用假 HTTP 和内存 span，四个固定用例覆盖正数、零和负数。评估同时检查最终数字、真实工具参数和结果、预算。只有答案碰巧正确但没有工具证据，不能通过。

这是一套验证执行链的离线契约评估，模拟 token 是固定值，不能据此声称模型准确率。要评价真实 DeepSeek：

```bash
uv run python -m demos.advanced.advanced_03_evaluation --live
```

通常需要 8 次真实模型请求，并自动上报 OpenObserve。任一用例失败返回非零退出码。真实模式的模型输出具有变化性；失败要查 trace，不能直接改成宽松断言来让报告变绿。

**通过标准：**能故意改错一条参考答案并看到评估失败；能拒绝“正确答案但没用工具”；保持用例不变比较两种 prompt 的通过率、用量和调用次数。

## A04：综合项目——只读资料 Agent

- 运行：`uv run python -m demos.advanced.advanced_04_knowledge_agent`。
- 阅读：`run_knowledge_agent()` → `SEARCH_TOOLS` / `dispatch_search()` → `validate_answer()`。
- 典型为 2 次模型调用 + 1 次搜索 + 根，共 4 个 span；允许在预算内调整关键词再次搜索。

本项目复用中级资料，但控制流程变了：中级由程序固定先检索；这里模型生成搜索参数，程序只允许 `search_documents`，执行结果回填给模型，最终答案必须通过来源 ID 校验。第一轮强制请求工具，后续自动决定是否继续搜索。

```bash
uv run python -m demos.advanced.advanced_04_knowledge_agent '星河笔记误删的笔记能恢复多久？请给出资料依据。'
uv run python -m demos.advanced.advanced_04_knowledge_agent '星河笔记支持导出哪些格式？'
```

限制包括：最多 4 轮模型调用、4 次工具执行、累计 token 软预算 8000、查询最长 200 字符、每次返回至多两个片段、无任意路径读写或 shell。答案校验能发现引用不存在，不能自动判定每句话都被资料支持。

**通过标准：**能展示完整问题 → 搜索参数 → 实际片段 → 模型最终 JSON；能识别资料不足；能说明这与中级固定 RAG 的差别；能加一个问题到固定评估集。

## 高级验收与毕业要求

```bash
uv run python -m demos.advanced.advanced_02_recovery
uv run python -m demos.advanced.advanced_03_evaluation
uv run python verify_live.py --level advanced
```

高级真实验收通常调用 3 次模型，验证思考题结果为 74，资料助手引用 recovery 且答出 30 天。要一起核对四个真实评估用例及它们的入库，再加 `--evaluate`。

毕业时应交付：一个可运行的综合脚本，一组固定评估问题，一份失败行为说明，三条可解释的 trace（成功、依据不足、失败）。当前单元测试提供确定性的失败覆盖，真实模型验收验证默认成功路径；它们的验证范围不同。

后续扩展可学习向量检索、长期记忆筛选、异步服务、总截止时间、任务队列、身份鉴权、Collector 与持久化缓冲、多 Agent 协作。它们是进一步的课程方向，不是本次 demo 已实现的功能。掌握这些底层边界后再读框架源码，会更容易理解它封装了什么。
