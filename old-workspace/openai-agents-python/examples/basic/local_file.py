import asyncio
import base64
import os

from agents import Agent, Runner

FILEPATH = os.path.join(os.path.dirname(__file__), "media/partial_o3-and-o4-mini-system-card.pdf")


def file_to_base64(file_path: str) -> str:
    with open(file_path, "rb") as f:
        return base64.b64encode(f.read()).decode("utf-8")


async def main():
    agent = Agent(
        name="Assistant",
        instructions="你是一个乐于助人的助手。",
    )

    b64_file = file_to_base64(FILEPATH)
    result = await Runner.run(
        agent,
        [
            {
                "role": "user",
                "content": [
                    {
                        "type": "input_file",
                        "file_data": f"data:application/pdf;base64,{b64_file}",
                        "filename": "partial_o3-and-o4-mini-system-card.pdf",
                    }
                ],
            },
            {
                "role": "user",
                "content": "引言的第一句话是什么？",
            },
        ],
    )
    print(result.final_output)


if __name__ == "__main__":
    asyncio.run(main())
