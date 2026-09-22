"""业务逻辑层（当前为空，等你写第一个真实业务时往里放）。

放什么：
    真正的业务逻辑 —— 查库、调 RPC、算业务规则、拼装领域对象。
    它**不 import** SDK 的任何东西，也不知道自己被 Agent 调用。

不放什么：
    参数校验、护栏、把结果转成给模型看的文本 —— 那些是 tools/ 的事。

为什么值得单独一层（三个好处，缺一不可）：
    1. 能被单测：import 它就测，不需要模型、不花 token
    2. 能被复用：HTTP 接口、定时任务、Agent 可以共用同一份逻辑
    3. 换框架不慌：Agent 框架换掉，这一层一行不用改

依赖方向：tools/ → services/（单向）。
真实例子（订单域）：

    # services/order.py
    def get_order(order_id: str) -> dict:
        return db.query_one("SELECT ... FROM orders WHERE id = %s", order_id)

    # tools/order.py
    @function_tool
    def query_order(ctx, order_id: str) -> str:
        order = order_service.get_order(order_id)      # 调这一层
        return f"订单 {order_id}：{order['status']}"    # 只做薄转换
"""
