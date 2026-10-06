# DeepSeek Demo 分级导航

当前课程提供 12 个完整 demo，代码位于 [`agent-learning/`](../agent-learning/)。

- [初级：3 个实验](levels/beginner.md)——一次调用、多轮对话、工具循环。
- [中级：5 个实验](levels/intermediate.md)——JSON、流式、故障处理、RAG、会话保存。
- [高级：4 个实验](levels/advanced.md)——思考模式、幂等恢复、评估、资料 Agent。

各级文档包含运行命令、读代码顺序、练习及验收标准。真实调用自动上报；离线故障实验和默认评估只记录内存，避免观测噪音。

先读 [学习入口](README.md)，在 `agent-learning/` 目录运行 `uv run python verify_observe.py`，然后运行 `uv run python -m demos.beginner.beginner_01_chat`。完整验证结果见 [验收记录](verification.md)。
