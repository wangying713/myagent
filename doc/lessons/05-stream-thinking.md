# 05：流式输出与思考模式

## 流式改变的是交付方式

非流式等结果完整后返回；流式在生成过程中逐段传来。你更早看到首段文字，但不一定减少总生成时间，也不减少需要执行的工具步骤。

```bash
uv run python -m demos.intermediate.intermediate_02_stream
```

`llm.py` 的 `chat_stream()` 直接读取 Server-Sent Events（SSE）。这些行里，`data:` 后面的 JSON 提供增量字段 `delta`。空行和保活注释不是内容。客户端累积内容，直到收到 `[DONE]`。具体格式参见 [DeepSeek Chat Completions](https://api-docs.deepseek.com/api/create-chat-completion/)。

## 不要把碎片当完整消息

文本分两段“你”“好”，拼接起来才是“你好”。工具 arguments 也可能先来 `{"a":`，再来 `2,"b":3}`。在参数完整之前调用 `json.loads()` 或执行工具都会出错。

本项目按工具调用的 `index` 收集碎片，最终生成完整的 `tool_calls`。可以收到带结束原因的最后一个片段，也兼容独立的 usage 片段。用量按服务端实际返回值记录，不用片段数推算 token。

观测只生成一个模型 span，包含完整重组结果；流中途异常时保存已收到的部分内容，标记 ERROR。没有读到 `[DONE]` 就不宣称流完整完成。终端已打印部分文本不等于调用成功。

## 思考模式是另外一个维度

流式 / 非流式决定返回方式；思考 / 非思考决定模型的生成模式。两组可以组合。入门示例默认显式传 `thinking={'type': 'disabled'}`，让工具协议和成本更容易观察。

已提供 `uv run python -m demos.advanced.advanced_01_thinking`，真实验收检查答案为 74。下面是同类调用的写法，仍使用统一底层客户端：

```python
from llm import DeepSeekClient, final_text

with DeepSeekClient('thinking_experiment') as model:
    print('trace_id:', model.trace_id)
    data = model.chat(
        [{'role': 'user', 'content': '一个两位数，十位数比个位数大3，数位和为11，求这个数。简短回答。'}],
        thinking={'type': 'enabled'},
        max_tokens=4096,
    )
    print(final_text(data))
```

开启后延迟和 token 使用可能增加；预算太小也可能在生成最终答案前结束。响应中的 `reasoning_content` 是服务端选择提供的字段，不应把它当作模型内部计算的完整、可靠证明，事实仍需要外部验证。

思考模式配合工具时，需遵守当前服务端的消息回传要求。当前官方指导要求在同一工具交互中保留所需的 `reasoning_content`，参见 [DeepSeek Thinking Mode](https://api-docs.deepseek.com/guides/thinking_mode/)。本项目保存完整 assistant 消息，不把它简化成仅 `role` 和 `content`。思考模式工具实验尚未做真实验收，不要在初学阶段同时改工具、流式和思考参数。

## 实验与自检

1. 运行流式脚本，确认屏幕逐段显示，而 OpenObserve 仍只有根节点和一个模型节点。
2. 运行离线测试中的流中断场景，查看测试如何验证部分内容和错误状态。
3. 比较同一个简单问题的思考与非思考请求，记录耗时、token、结果是否正确。简单问题不一定需要更多推理。

流式性能后续可以增加“首字耗时”与“总耗时”两个指标；目前只记录总耗时。异步、并发、取消传播属于后续工程扩展，本实现是同步教学客户端。

下一节：[失败与预算](06-reliability.md)。
