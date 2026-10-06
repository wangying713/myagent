"""中级：关闭并重建存储连接后继续对话；SQLite 保存的是历史，不是模型记忆。"""

import argparse
from pathlib import Path
from tempfile import TemporaryDirectory

from llm import DeepSeekClient, final_text, message_of
from session_store import SessionStore


def ask(model, db_path, session_id, question):
    """加载会话历史、请求模型回答，并以 revision 检查后保存新历史。

    Args:
        model: 已进入 with 块的 DeepSeekClient。
        db_path: SQLite 数据库文件路径。
        session_id: 用于隔离不同对话的会话 ID。
        question: 本轮用户问题。

    Returns:
        模型生成的最终文本回答。

    Raises:
        ModelError: 模型未生成完整文本时。
        SessionConflict: 保存时发现会话已被其他操作更新时。
    """
    store = SessionStore(db_path)
    messages, revision = store.load(session_id)
    if not messages:
        messages = [
            {
                "role": "system",
                "content": "简短回答；仅根据会话中明确给出的事实，不知道就说明。",
            }
        ]
    messages.append({"role": "user", "content": question})
    data = model.chat(messages)
    answer = final_text(data)
    messages.append(message_of(data))
    store.save(session_id, messages, revision)
    return answer


def main():
    """演示会话保存、重新读取和不同 session ID 之间的隔离。"""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--db", type=Path, help="指定文件可跨进程保存；省略则使用临时库"
    )
    parser.add_argument("--session", default="alice")
    parser.add_argument("--ask")
    args = parser.parse_args()
    with (
        TemporaryDirectory() as folder,
        DeepSeekClient("intermediate_memory") as model,
    ):
        path = args.db or Path(folder) / "sessions.sqlite3"
        print("trace_id:", model.trace_id)
        if args.ask:
            print(ask(model, path, args.session, args.ask))
        else:
            print(ask(model, path, args.session, "请记住我的项目代号是海盐。"))
            print(ask(model, path, args.session, "我的项目代号是什么？"))
            print(
                "另一个会话 bob 的历史条数:",
                len(SessionStore(path).load("bob")[0]),
            )


if __name__ == "__main__":
    main()
