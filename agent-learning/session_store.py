"""按 session_id 保存消息；版本检查防止并发请求悄悄覆盖新历史。"""

import json
import sqlite3
from contextlib import closing


class SessionConflict(RuntimeError):
    """表示会话版本已变化，当前写入不能安全覆盖新历史。"""

    pass


class SessionStore:
    """用 SQLite 按会话 ID 保存消息，并用 revision 检测覆盖冲突。"""

    def __init__(self, path):
        """打开指定数据库并确保会话表存在。

        Args:
            path: SQLite 数据库文件路径。
        """
        self.path = str(path)
        with closing(sqlite3.connect(self.path)) as db, db:
            db.execute(
                "CREATE TABLE IF NOT EXISTS sessions "
                "(id TEXT PRIMARY KEY, revision INTEGER NOT NULL, "
                "messages TEXT NOT NULL)"
            )

    def load(self, session_id):
        """读取会话消息及其版本号；新会话返回空消息和版本 0。

        Args:
            session_id: 1 到 80 个字符的会话标识。

        Returns:
            (messages, revision) 二元组。

        Raises:
            ValueError: 会话 ID 不符合长度要求时。
        """
        if not isinstance(session_id, str) or not 1 <= len(session_id) <= 80:
            raise ValueError("session_id 必须为 1–80 字符")
        with closing(sqlite3.connect(self.path)) as db:
            row = db.execute(
                "SELECT messages, revision FROM sessions WHERE id=?",
                (session_id,),
            ).fetchone()
        return (json.loads(row[0]), row[1]) if row else ([], 0)

    def save(self, session_id, messages, expected_revision):
        """仅当数据库版本等于 expected_revision 时保存新消息历史。

        Args:
            session_id: 要更新的会话标识。
            messages: 要保存的完整消息列表。
            expected_revision: 读取历史时得到的版本号。

        Raises:
            ValueError: 会话 ID 不符合长度要求时。
            SessionConflict: 会话已由另一请求更新时。
        """
        self.load(session_id)  # 检查 ID；真正的版本比较在同一个事务里完成。
        with closing(sqlite3.connect(self.path)) as db, db:
            db.execute("BEGIN IMMEDIATE")
            row = db.execute(
                "SELECT revision FROM sessions WHERE id=?", (session_id,)
            ).fetchone()
            if (row[0] if row else 0) != expected_revision:
                raise SessionConflict(
                    "会话已被另一请求更新；请重新加载，不自动重复模型调用"
                )
            db.execute(
                "INSERT INTO sessions VALUES (?, ?, ?) "
                "ON CONFLICT(id) DO UPDATE SET revision=excluded.revision, "
                "messages=excluded.messages",
                (
                    session_id,
                    expected_revision + 1,
                    json.dumps(messages, ensure_ascii=False),
                ),
            )
