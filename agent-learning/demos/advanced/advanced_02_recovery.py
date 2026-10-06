"""高级：模拟审批与崩溃恢复，只创建临时 SQLite 中的工单，不调用外部系统。"""

import json
import sqlite3
from contextlib import closing
from pathlib import Path
from tempfile import TemporaryDirectory


class LocalJobs:
    """用 SQLite 示范审批状态、事务提交和本地幂等重放。"""

    def __init__(self, path):
        """创建演示所需的任务表和工单表。

        Args:
            path: SQLite 数据库文件路径。
        """
        self.path = str(path)
        with closing(sqlite3.connect(self.path)) as db, db:
            db.execute(
                "CREATE TABLE IF NOT EXISTS jobs "
                "(id TEXT PRIMARY KEY, title TEXT NOT NULL, "
                "state TEXT NOT NULL, "
                "result TEXT)"
            )
            db.execute(
                "CREATE TABLE IF NOT EXISTS tickets "
                "(job_id TEXT PRIMARY KEY, title TEXT NOT NULL)"
            )

    def plan(self, job_id, title):
        """登记一项等待审批的任务；相同 ID 不能代表不同内容。

        Args:
            job_id: 用于重放去重的任务 ID。
            title: 任务要创建的工单标题。

        Raises:
            ValueError: ID 或标题为空，或 ID 已绑定到其他标题时。
        """
        if not job_id or not title:
            raise ValueError("任务 ID 和标题不能为空")
        with closing(sqlite3.connect(self.path)) as db, db:
            db.execute("BEGIN IMMEDIATE")
            row = db.execute(
                "SELECT title FROM jobs WHERE id=?", (job_id,)
            ).fetchone()
            if row and row[0] != title:
                raise ValueError("同一幂等键不能对应不同操作内容")
            db.execute(
                "INSERT OR IGNORE INTO jobs VALUES (?, ?, ?, NULL)",
                (job_id, title, "awaiting_approval"),
            )

    def execute(self, job_id, *, approved, before_commit=None):
        """获批后在同一事务中创建工单并保存可重放结果。

        Args:
            job_id: 已规划任务的 ID。
            approved: 必须为 True 才允许执行。
            before_commit: 可选测试钩子，用于模拟提交前故障。

        Returns:
            新建或此前已完成任务的工单结果。

        Raises:
            PermissionError: 任务未获批准时。
            ValueError: 找不到已规划任务时。
        """
        # approved 只是教学中模拟的审批结果，不是生产权限系统。
        if approved is not True:
            raise PermissionError("任务未获批准，保持待审批状态")
        with closing(sqlite3.connect(self.path)) as db, db:
            db.execute("BEGIN IMMEDIATE")
            row = db.execute(
                "SELECT title, state, result FROM jobs WHERE id=?", (job_id,)
            ).fetchone()
            if row is None:
                raise ValueError("任务尚未规划")
            if row[1] == "done":
                return json.loads(row[2])
            result = {"ticket_id": job_id, "title": row[0]}
            db.execute("INSERT INTO tickets VALUES (?, ?)", (job_id, row[0]))
            db.execute(
                "UPDATE jobs SET state=?, result=? WHERE id=?",
                ("done", json.dumps(result, ensure_ascii=False), job_id),
            )
            if before_commit:
                before_commit()  # 仅供测试注入故障，异常使整个本地事务回滚。
        return result

    def ticket_count(self):
        """返回数据库中已创建的演示工单数量。"""
        with closing(sqlite3.connect(self.path)) as db:
            return db.execute("SELECT count(*) FROM tickets").fetchone()[0]


def main():
    """在临时数据库中演示审批拒绝、执行和幂等重放。"""
    with TemporaryDirectory() as folder:
        path = Path(folder) / "jobs.sqlite3"
        jobs = LocalJobs(path)
        jobs.plan("demo-001", "演示工单：检查导出说明")
        try:
            jobs.execute("demo-001", approved=False)
        except PermissionError:
            print("未批准：实际工单数=", jobs.ticket_count())
        print("模拟批准，执行结果:", jobs.execute("demo-001", approved=True))
        print("模拟：数据库已提交，但进程还没来得及把结果展示给用户就退出。")
        restarted = LocalJobs(path)
        print(
            "重建连接后重放同一任务:",
            restarted.execute("demo-001", approved=True),
        )
        print("最终工单数=", restarted.ticket_count(), "（应为 1）")
        print(
            "本实验仅证明同一 SQLite 事务内的幂等性；远程 API 需要独立的幂等协议。"
        )


if __name__ == "__main__":
    main()
