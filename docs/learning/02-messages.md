# 第二课：消息历史由应用保存

```sh
go run ./apps/interfaces/learning -lesson messages -env-file .env -verify
```

阅读 `apps/interfaces/learning/lessons.go` 的 `messagesLesson`，重点看调用前后的两次 `append`。

## 两轮的消息变化

第一轮发送两条消息：system、user。收到回答后，保存 assistant 消息。第二轮再追加 user，共发送四条消息：system、第一轮 user、第一轮 assistant、第二轮 user。

模型并不是凭某个会话名称自动找回历史；应用必须显式传递相关材料。这里的历史只在当前进程的一次课程运行中存在，还不是跨进程的持久记忆。

正常 trace 是根 span 加两次模型请求，共三个 span。查看第二次请求的 `app.request`，确认第一轮消息确实存在。

## 小练习

1. 修改虚构小猫的名字，检查第二次请求与最终回答。
2. 临时让第二轮只携带 system 和最后一句 user，比较模型回答，再恢复。
3. 对比两轮输入 token，说明历史增长为何影响用量。服务端没有报告用量时，不自行编造数字。

通过标准：能写出两轮消息顺序，区分请求上下文、进程内历史与持久化会话；能解释接口替换如何让测试检查真实传出的消息。
