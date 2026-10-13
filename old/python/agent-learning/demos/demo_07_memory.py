"""中级：关闭并重建存储连接后继续对话；SQLite 保存的是历史，不是模型记忆。"""

import argparse
from pathlib import Path
from tempfile import TemporaryDirectory

from old.agent_toolkit.llm import DeepSeekClient, final_text, message_of
from old.agent_toolkit.session_store import SessionStore


def ask(llm_client, db_path, session_id, question):
    """加载会话历史、请求模型回答，并以 revision 检查后保存新历史。

    Args:
        llm_client: 已进入 with 块的 DeepSeekClient。
        db_path: SQLite 数据库文件路径。
        session_id: 用于隔离不同对话的会话 ID。
        question: 本轮用户问题。

    Returns:
        模型生成的最终文本回答。

    Raises:
        ModelError: 模型未生成完整文本时。
        SessionConflictError: 保存时发现会话已被其他操作更新时。
    """
    session_store = SessionStore(db_path)
    messages, revision = session_store.load(session_id)
    if not messages:
        messages = [
            {
                "role": "system",
                "content": "简短回答；仅根据会话中明确给出的事实，不知道就说明。",
            }
        ]
    messages.append({"role": "user", "content": question})
    response_data = llm_client.chat(messages)
    answer = final_text(response_data)
    messages.append(message_of(response_data))
    session_store.save(session_id, messages, revision)
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
        TemporaryDirectory() as temp_dir,
        DeepSeekClient("intermediate_memory") as llm_client,
    ):
        db_path = args.db or Path(temp_dir) / "sessions.sqlite3"
        print("trace_id:", llm_client.trace_id)
        if args.ask:
            print(ask(llm_client, db_path, args.session, args.ask))
        else:
            print(
                ask(
                    llm_client,
                    db_path,
                    args.session,
                    "请记住我的项目代号是海盐。",
                )
            )
            print(
                ask(llm_client, db_path, args.session, "我的项目代号是什么？")
            )
            print(
                "另一个会话 bob 的历史条数:",
                len(SessionStore(db_path).load("bob")[0]),
            )


if __name__ == "__main__":
    main()
