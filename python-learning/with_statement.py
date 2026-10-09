"""用可见的输出学习 with、as 和多个上下文管理器。"""

import sqlite3
from contextlib import closing
from pathlib import Path
from tempfile import TemporaryDirectory


class DemoResource:
    """打印资源何时进入和离开 with 代码块。"""

    def __init__(self, name):
        """保存资源名称，供输出演示使用。"""
        self.name = name

    def __enter__(self):
        """进入 with 代码块，并把本对象交给 as 后面的变量。"""
        print(f"进入：{self.name}")
        return self

    def __exit__(self, error_type, error, traceback):
        """离开 with 代码块；返回 False 表示异常要继续向外传递。"""
        if error_type is None:
            print(f"离开：{self.name}（正常结束）")
        else:
            print(f"离开：{self.name}（遇到 {error_type.__name__}）")

        return False

    def use(self):
        """模拟使用这个资源。"""
        print(f"使用：{self.name}")


def show_as():
    """演示 as 把 __enter__ 返回的对象交给变量。"""
    print("示例一：as 给对象起一个临时变量名")
    with DemoResource("水杯") as cup:
        # DemoResource.__enter__ 返回 self，所以 cup 指向这个 DemoResource。
        print("as 后的变量是：", cup.name)
        cup.use()

    print()


def show_multiple_files():
    """演示同时打开输入、输出文件的实际用法。"""
    print("示例二：读取一个文件，并把内容写入另一个文件")
    with TemporaryDirectory() as folder:
        source_path = Path(folder) / "source.txt"
        target_path = Path(folder) / "copy.txt"
        source_path.write_text("复制这段文字。", encoding="utf-8")

        # source 和 target 是两个不同文件，各自用 as 绑定变量名。
        with (
            source_path.open(encoding="utf-8") as source,
            target_path.open("w", encoding="utf-8") as target,
        ):
            target.write(source.read())

        print("复制后的内容：", target_path.read_text(encoding="utf-8"))
        print("离开 with 后，输入和输出文件都已自动关闭。")
        print("如果打开输出文件失败，Python 也会关闭已经打开的输入文件。")

    print()


def show_exception_cleanup():
    """演示即使代码块发生异常，__exit__ 仍会被调用。"""
    print("示例三：代码块出错时也会执行清理")
    try:
        with DemoResource("会遇到错误的对象"):
            print("现在故意制造一个错误。")
            raise ValueError("这是教学用的错误")
    except ValueError as error:
        # __exit__ 返回 False，所以异常继续传到这里，由 except 接住。
        print("外面的 except 接住了：", error)

    print()


def show_database_connection():
    """对比只关闭连接与提交事务后再关闭连接。"""
    print("示例四：关闭连接不等于提交事务")
    with TemporaryDirectory() as folder:
        database_path = Path(folder) / "with-demo.sqlite3"
        connection = sqlite3.connect(database_path)
        connection.execute("CREATE TABLE notes (message TEXT)")
        connection.commit()
        connection.close()

        print("情况一：只用 closing 关闭连接")
        connection = sqlite3.connect(database_path)
        with closing(connection) as db:
            db.execute("BEGIN")
            db.execute("INSERT INTO notes VALUES (?)", ("还没提交",))

        # 重新打开数据库，检查刚才未提交的写入是否保留下来。
        connection = sqlite3.connect(database_path)
        with closing(connection) as db:
            row_count = db.execute("SELECT count(*) FROM notes").fetchone()[0]
        print("只用 closing 后，表中的记录数：", row_count)
        print("close 只负责关闭；未提交的事务会回滚，所以记录数是 0。")

        print("情况二：写入后显式调用 commit，再关闭连接")
        connection = sqlite3.connect(database_path)
        with closing(connection) as db:
            db.execute("BEGIN")
            db.execute("INSERT INTO notes VALUES (?)", ("已提交",))
            # commit 把事务中的更改保存到数据库；closing 随后负责关闭连接。
            db.commit()

        # 重新连接并读取，确认 commit 已经保存了数据。
        connection = sqlite3.connect(database_path)
        with closing(connection) as db:
            saved_message = db.execute("SELECT message FROM notes").fetchone()[0]
        print("用 db 管理事务后，重新读取到：", saved_message)


def main():
    """依次运行四个不联网的 with 语法小实验。"""
    show_as()
    show_multiple_files()
    show_exception_cleanup()
    show_database_connection()


if __name__ == "__main__":
    main()
