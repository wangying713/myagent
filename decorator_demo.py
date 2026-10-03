"""装饰器（@xxx）到底是什么意思 —— 一步步跑给你看。

运行：python3 decorator_demo.py
临时文件，不属于 my-agent，不依赖任何第三方库。
看输出比看文字快。
"""

import functools

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


# ── 4. 第 2、3 步留了个副作用：名字被换成了 wrapper ──────────
#    functools.wraps 把原函数的 __name__、__doc__ 抄回 wrapper 上。
def add_exclamation_fixed(fn):
    @functools.wraps(fn)        # 就加这一行
    def wrapper():
        return fn() + "！"
    return wrapper


@add_exclamation_fixed
def greet_4():
    """打个招呼。"""
    return "你好"


print("4) 加了 functools.wraps 之后：")
print("   名字：", greet_4.__name__, "| docstring：", greet_4.__doc__)
print("   不加这一行，上面两个会变成 wrapper 和 None")
print("   —— agents 的 @tool 靠这两样取工具名和工具说明")


# ── 5. 带参数的函数照样能装饰 ────────────────────────────────
def log_call(fn):
    def wrapper(*args, **kwargs):        # *args/**kwargs：把参数原样收下
        print(f"   [日志] 调用了 {fn.__name__}，参数={args}")
        return fn(*args, **kwargs)       # 再原样传给真正的函数
    return wrapper


@log_call
def add(a, b):
    return a + b


print("5) 调用 add(2, 3)，会先看到装饰器打的日志：")
result = add(2, 3)          # 分两步写，输出顺序更清楚
print("   结果是：", result)


# ── 6. 关键细节：里面的 wrapper 是怎么记住 fn 的？ ────────────
#    fn 是 log_call 的参数，被里面的 wrapper 用上了。
#    外层函数其实早就执行完了，wrapper 却还能用 fn —— 这个特性叫「闭包」。
#    装饰器能工作，全靠它。
def make_counter():
    count = 0
    def bump():
        nonlocal count      # 不加这行，count 只能读不能改
        count += 1
        return count
    return bump


counter = make_counter()
print("6) 闭包演示：make_counter() 早就返回了，count 却还活着")
print("   连调三次：", counter(), counter(), counter())


# ── 7. 装饰器自己也想收参数 → 那就再包一层 ───────────────────
#    @repeat(3) 是「先调用 repeat(3) 拿到装饰器，再去包 say」。
def repeat(times):
    def deco(fn):                    # 这一层才是真正的装饰器
        def wrapper(*args, **kwargs):
            for _ in range(times):
                result = fn(*args, **kwargs)
            return result
        return wrapper
    return deco


@repeat(3)
def say(msg):
    print("   say:", msg)
    return msg


print("7) @repeat(3) 会执行 3 次：")
say("你好")
print("   对比 @log_call 后面没括号：log_call 自己就是装饰器，不需要配置参数")


# ── 8. 叠多个装饰器：从下往上包 ──────────────────────────────
def add_star(fn):
    def wrapper(*args, **kwargs):
        return "*" + fn(*args, **kwargs) + "*"
    return wrapper


def add_line(fn):
    def wrapper(*args, **kwargs):
        return "=" + fn(*args, **kwargs) + "="
    return wrapper


@add_star               # 后包（外面一层）
@add_line               # 先包（贴着函数）
def text():
    return "abc"


print("8) text() 的结果：", text())
print("   等于 add_star(add_line(text))，离 def 最近的那个先执行")


# ── 9. 执行时机：定义函数的时候就跑掉了，不是调用的时候 ───────
def shout_deco(fn):
    print(f"   [装饰器执行了] 正在包装 {fn.__name__}")
    def wrapper(*args, **kwargs):
        return fn(*args, **kwargs).upper()
    return wrapper


print("9) 下面定义 hi() 时，装饰器会立刻打印一行：")


@shout_deco
def hi():
    return "hello"


print("   定义完了。现在调用 3 次，装饰器不会重复打印：")
print("   ", hi(), "|", hi(), "|", hi())


# ── 10. 另一类装饰器：返回的压根不是函数 ─────────────────────
#     前面 1~9 都是「包一层再返回函数」，调用方式不变。
#     agents 的 @tool 属于另一类：它返回一个对象，不是函数。
class FakeTool:                     # 假装这就是 SDK 的 FunctionTool
    def __init__(self, fn):
        self.name = fn.__name__                        # 工具名取自函数名
        self.description = (fn.__doc__ or "").strip()  # 说明取自 docstring
        self.func = fn


def fake_tool(fn):
    return FakeTool(fn)             # 注意：返回的不是 wrapper


@fake_tool
def get_weather(city):
    """查询指定城市的当前天气信息。"""
    return f"{city}：晴"


print("10) 装饰后 get_weather 是什么：", type(get_weather).__name__)
print("    工具名：", get_weather.name)
print("    说明  ：", get_weather.description)
try:
    get_weather("东京")             # 名字已经被换成对象了，调不动
except TypeError as e:
    print("    直接调用：失败 ->", e)
print("    要执行得走它肚子里的 func：", get_weather.func("东京"))


# ── 11. 回到 agents 的 @tool（一句话） ───────────────────────
#     decorators.py 里写着 tool = function_tool，
#     @tool 就是第 10 步这个形式，只是 FunctionTool 比 FakeTool 复杂得多：
#     它还会读参数的类型标注，生成一张 JSON 参数表一起发给模型。
print("=" * 60)
