# 课程与 Demo 验收记录

日期：2026-10-05。已完成本轮三级课程 review、补充实现和验收。使用现有 `.env`，服务名为 `chat-demo`；原有凭据和历史观测数据未更改。

## 本轮结果

- 三级共 12 个可运行 demo：初级 3 个，中级 5 个，高级 4 个。
- 25 项离线测试全部通过；故障、恢复和离线评估脚本独立运行成功。
- 真实验收 10 个场景，加 4 个计算评估用例，全部通过。实际共 21 次模型请求、9 次工具执行、14 个 trace、44 个 span，并已查询确认入库。
- 资料 Agent 本次一轮返回两个搜索请求，因此该 trace 是 5 个 span；验收按实际调用数检查，不硬编码为 4。
- 本地原始摘要位于 `agent-learning/.local/verification-levels.json`，已被 Git 忽略。

## 真实验收证据

| 场景 | 模型调用 | 工具调用 | span 数 | trace_id |
|---|---:|---:|---:|---|
| chat | 1 | 0 | 2 | `2db4218d5936c18634fbd33931c413ef` |
| messages | 2 | 0 | 3 | `d55b3aa9999766369c2e72c0c69864c0` |
| agent | 2 | 1 | 4 | `387482bfad5d4e4384e3f4b14e1dd07b` |
| json | 1 | 0 | 2 | `cdedecb173e1afbe58a53c6f5151d94c` |
| stream | 1 | 0 | 2 | `25f8afc3be9be5408950f2becf57721c` |
| rag | 1 | 1 | 3 | `8ef1f1214b059b2a5fb3986148116129` |
| rag_unknown | 0 | 1 | 2 | `6ebe20f224768d31cdb526581637d582` |
| memory | 2 | 0 | 3 | `986275add7115636937b5c3cb5c87cd8` |
| thinking | 1 | 0 | 2 | `1573f8179015e57abe7c03227673f8e1` |
| knowledge | 2 | 2 | 5 | `bf912e1e9ddd19351f587663f00e78fa` |
| eval_positive | 2 | 1 | 4 | `27d1886946c473e00ed2a010d0d756de` |
| eval_zero | 2 | 1 | 4 | `cc7045576f1b6628eb91310d21f598a0` |
| eval_negative | 2 | 1 | 4 | `80eb19f4bb5addf35ccc8be3a90a4255` |
| eval_small | 2 | 1 | 4 | `1fecff7288fc76fb9bb5f20814e5c6ef` |

## 具体检查了什么

真实实验同时验证业务结果与观测：问候、暗号、17×23 的最终数字和工具结果、订单字段、完整流式文字、检索引用、无资料时不调用模型、SQLite 恢复后的项目代号、思考题答案 74、资料助手的 30 天恢复依据。四个真实评估用例还逐项检查工具参数和结果。

所有真实运行均核对完整 span 数量、类型、父子关系、请求响应正文、输入用量，以及模型密钥未进入记录。离线测试覆盖不合法协议、token 字段、开关拼写、有限重试、参数限制、预算、流中断、上报失败、部分拒收、引用伪造、会话冲突和事务回滚。

## 重现命令

```bash
cd /Users/wangying/apps/sakelei/ai/agent-learning
uv run python -m unittest discover -s tests -v
uv run python -m demos.intermediate.intermediate_03_reliability
uv run python -m demos.advanced.advanced_02_recovery
uv run python -m demos.advanced.advanced_03_evaluation
uv run python verify_live.py --level all --evaluate --report .local/verification-levels.json
```

最后一条会调用真实模型并消耗 token，平时按级运行即可。模型输出会变化，后续某次失败应保留 trace 排查，不修改参考答案来掩盖失败。

## 验证边界

已验证的是教学级成功路径和离线边界，不是生产可靠性认证。思考模式配合工具、向量检索、自动长期记忆、远程副作用幂等、多用户身份鉴权、分布式恢复与生产并发仍未实现。恢复实验的审批是模拟输入，事务保证仅限同一 SQLite 数据库。

这些 trace 可能随服务的保留策略清除。查询脚本默认查最近 24 小时，日后需调整 UI 时间范围，或重新运行对应实验。
