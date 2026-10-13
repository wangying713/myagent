"""一个简单的 with 例子：读取文件后，自动关闭文件。

运行：python3 python-learning/with_simple.py
"""

# __file__ 表示当前这个 Python 文件的路径，直接用它练习读取即可。
# open 打开文件；as file 让我们用 file 这个变量操作文件。
with open(__file__, encoding="utf-8") as file:
    text = file.read()
    print("读取成功，共", len(text), "个字符")
    print("with 里面，文件关闭了吗？", file.closed)  # False

# 不再缩进，表示已经离开 with；文件在这里已自动关闭。
print("with 外面，文件关闭了吗？", file.closed)  # True

# 记住：在 with 里面使用文件，离开后自动关闭，不必手动写 file.close()。
