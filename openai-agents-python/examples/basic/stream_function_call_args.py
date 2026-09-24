import asyncio
from typing import Annotated, Any

from openai.types.responses import ResponseFunctionCallArgumentsDeltaEvent

from agents import (
    Agent,
    Runner,
)
from agents.decorators import tool


@tool
def write_file(filename: Annotated[str, "文件名"], content: str) -> str:
    """把内容写入文件。"""
    return f"文件 {filename} 写入成功"


@tool
def create_config(
    project_name: Annotated[str, "项目名"],
    version: Annotated[str, "项目版本"],
    dependencies: Annotated[list[str] | None, "依赖列表（包名）"],
) -> str:
    """生成项目配置文件。"""
    return f"已为 {project_name} v{version} 生成配置"


async def main():
    """
    演示函数调用参数的实时流式输出。

    函数参数在生成过程中会被逐段推送出来，
    让你在参数生成期间就能立刻看到反馈。
    """
    agent = Agent(
        name="CodeGenerator",
        instructions="你是一个乐于助人的编码助手。请使用提供的工具来创建文件和配置。",
        tools=[write_file, create_config],
    )

    print("🚀 函数调用参数流式输出演示")

    result = Runner.run_streamed(
        agent,
        input="创建一个名为 'my-app' 的 Python Web 项目，使用 FastAPI。版本 1.0.0，依赖：fastapi、uvicorn",
    )

    # 跟踪函数调用，便于输出详细信息
    function_calls: dict[Any, dict[str, Any]] = {}  # call_id -> {name, arguments}
    current_active_call_id = None

    async for event in result.stream_events():
        if event.type == "raw_response_event":
            # 函数调用开始
            if event.data.type == "response.output_item.added":
                if getattr(event.data.item, "type", None) == "function_call":
                    function_name = getattr(event.data.item, "name", "unknown")
                    call_id = getattr(event.data.item, "call_id", "unknown")

                    function_calls[call_id] = {"name": function_name, "arguments": ""}
                    current_active_call_id = call_id
                    print(f"\n📞 函数调用流式输出开始：{function_name}()")
                    print("📝 参数生成中……")

            # 参数的实时流式输出
            elif isinstance(event.data, ResponseFunctionCallArgumentsDeltaEvent):
                if current_active_call_id and current_active_call_id in function_calls:
                    function_calls[current_active_call_id]["arguments"] += event.data.delta
                    print(event.data.delta, end="", flush=True)

            # 函数调用完成
            elif event.data.type == "response.output_item.done":
                if hasattr(event.data.item, "call_id"):
                    call_id = getattr(event.data.item, "call_id", "unknown")
                    if call_id in function_calls:
                        function_info = function_calls[call_id]
                        print(f"\n✅ 函数调用流式输出完成：{function_info['name']}")
                        print()
                        if current_active_call_id == call_id:
                            current_active_call_id = None

    print("所有函数调用汇总：")
    for call_id, info in function_calls.items():
        print(f"  - #{call_id}: {info['name']}({info['arguments']})")

    print(f"\n结果：{result.final_output}")


if __name__ == "__main__":
    asyncio.run(main())
