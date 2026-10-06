import asyncio

from agents import Agent, Runner

URL = "https://images.unsplash.com/photo-1505761671935-60b3a7427bad?auto=format&fit=crop&w=400&q=80"


async def main():
    agent = Agent(
        name="Assistant",
        instructions="你是一个乐于助人的助手。",
    )

    result = await Runner.run(
        agent,
        [
            {
                "role": "user",
                "content": [{"type": "input_image", "detail": "auto", "image_url": URL}],
            },
            {
                "role": "user",
                "content": "你在这张图里看到了什么？",
            },
        ],
    )
    print(result.final_output)


if __name__ == "__main__":
    asyncio.run(main())
