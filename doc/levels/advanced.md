# 高级：思考模式、评估与综合项目

**目标：**能说明模型如何使用工具与资料、答案是否正确、改动是否有效。本级 3 个 demo 都有完整代码，均使用真实模型。这里的高级是单机工程与综合实践，不代表已经实现生产部署、分布式一致性或完整多用户权限。

## A01：思考模式与资源消耗

- 文档：[流式与思考模式](../lessons/05-stream-thinking.md) 的思考部分。
- 运行：`uv run python -m demos.advanced.advanced_01_thinking`。
- 阅读：`solve()`，通常 1 次真实请求、2 个 span。

题目是“十位比个位大 3，数位和 11”，正确结果为 74。先检查最终答案，再看 token 与耗时。终端只显示最终答案；服务端提供的 `reasoning_content` 仍保存在脱敏、限长的响应中。

关闭思考模式重复同一题，比较答案和资源消耗。推理文本更长不能代替答案验算。一次题目通过也不证明思考模式在所有任务上更好。

**通过标准：**能区分思考开关与流式开关，能解释 max_tokens 太小时为什么可能没有最终答案。思考模式配合工具需要额外遵循回传规则，本课程的真实高级验收只验证单次思考调用。

## A03：固定用例评估

- 文档：[评估与交付](../lessons/08-evaluation.md)。
- 运行：`uv run python -m demos.advanced.advanced_03_evaluation`。
- 阅读：`data/eval_cases.json` → `evaluate()` → `grade()`。

使用真实 DeepSeek，四个固定用例覆盖正数、零和负数。评估同时检查最终数字、实际工具参数和结果、预算。只有答案碰巧正确但没有工具证据，不能通过。

评估直接读取客户端的 `tool_results` 校验实际执行结果；OpenObserve 上报由共享客户端自动处理，示例无需创建 exporter 或读取 span。

通常需要 8 次真实模型请求，会消耗 token，并自动上报 OpenObserve。任一用例失败返回非零退出码。模型输出具有变化性；失败要查 trace，不能直接改成宽松断言来让报告变绿。

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
uv run python -m demos.advanced.advanced_03_evaluation
uv run python verify_live.py --level advanced
```

高级真实验收通常调用 3 次模型，验证思考题结果为 74，资料助手引用 recovery 且答出 30 天。要一起核对四个真实评估用例及它们的入库，再加 `--evaluate`。

毕业时应交付：一个可运行的综合脚本，一组固定评估问题，一份失败行为说明，三条可解释的 trace（成功、依据不足、失败）。当前单元测试提供确定性的失败覆盖，真实模型验收验证默认成功路径；它们的验证范围不同。

后续扩展可学习向量检索、长期记忆筛选、异步服务、总截止时间、任务队列、身份鉴权、Collector 与持久化缓冲、多 Agent 协作。它们是进一步的课程方向，不是本次 demo 已实现的功能。掌握这些底层边界后再读框架源码，会更容易理解它封装了什么。
