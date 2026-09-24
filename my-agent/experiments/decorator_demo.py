"""装饰器（@xxx）到底是什么意思 —— 一步步跑给你看。

运行：python3 my-agent/experiments/decorator_demo.py
只用标准库，跟 agents 无关，任何 Python 3 都能跑。
看输出比看文字快。
"""

print("=" * 60)


# ── 1. 先看一个普通函数 ──────────────────────────────────────
def greet():
    return "你好"


print("1) 直接调用 greet()：", greet())


# ── 2. 函数可以当参数传进去，也可以当返回值传出来 ─────────────
#    这是装饰器的地基：函数就是一种普通的值，能拿来拿去。
def add_exclamation(fn):        # fn 收到的是一个函数
    def wrapper():              # 里面造一个新函数
        return fn() + "！"      # 调用传进来的那个函数，再加工一下
    return wrapper              # 把新函数返回出去


greet_2 = add_exclamation(greet)   # 不写 @，手动做这件事
print("2) 手动包装后 greet_2()：", greet_2())
print("   注意 greet_2 的名字变成了：", greet_2.__name__)


# ── 3. @ 就是第 2 步的省写形式 ───────────────────────────────
#    下面这三行，完全等于上面那行 greet_2 = add_exclamation(greet)
@add_exclamation
def greet_3():
    return "你好"


print("3) 用 @ 的效果 greet_3()：", greet_3())
print("   名字同样变成了：", greet_3.__name__)
print("   因为 @add_exclamation 就等于 greet_3 = add_exclamation(greet_3)")


# ── 4. 带参数的函数照样能装饰 ────────────────────────────────
def log_call(fn):
    def wrapper(*args, **kwargs):        # *args/**kwargs：把参数原样收下
        print(f"   [日志] 调用了 {fn.__name__}，参数={args}")
        return fn(*args, **kwargs)       # 再原样传给真正的函数
    return wrapper


@log_call
def add(a, b):
    return a + b


print("4) 调用 add(2, 3)，会先看到装饰器打的日志：")
result = add(2, 3)          # 分两步写，输出顺序更清楚
print("   结果是：", result)


# ── 5. 关键细节：里面的 wrapper 是怎么记住 fn 的？ ────────────
#    fn 是 add_exclamation / log_call 的参数，被里面的 wrapper 用上了。
#    外层函数其实早就执行完了，wrapper 却还能用 fn —— 这个特性叫「闭包」。
#    装饰器能工作，全靠它。


# ── 6. 回到 agents 的 @tool（一句话） ────────────────────────
#    @tool 就是第 3 步那个形式：
#        fetch_random_image = tool(fetch_random_image)
#    区别只在于 tool() 返回的不是「wrapper 函数」，
#    而是一个 FunctionTool 对象（带工具名、说明、参数表），模型才能调用它。
print("=" * 60)
