# Go 编辑器配置与代码跳转

使用 VS Code 打开项目根目录 `ai`。项目的 [.vscode/settings.json](../../.vscode/settings.json) 为这个工作区指定 Go 与语言服务路径，不修改系统安装或其他项目的设置。

## 输入缩进

工作区按 Tab 键输入空格，默认每级 2 个空格，Go 文件每级 4 个空格。关闭自动识别缩进，避免已有文件中的 Tab 改变输入设置。

这些设置控制手动输入，不批量转换已有文件。Go 的 `gofmt` 和 gopls 格式化仍按 Go 规则使用 Tab，运行格式化后会恢复标准 Go 排版。

## 当前工具

- Go：项目内 `.tools/go1.27.2`，与 `go.mod` 的工具链版本一致。
- gopls：项目内 `.tools/gopls/bin/gopls`，本次安装版本为 v0.23.0。
- 工作区为工具进程设置 `GOROOT`、`GOTOOLCHAIN=local` 和 Go 优先的 `PATH`，使扩展与语言服务使用同一套工具链。

`gopls` 是负责 Go 跳转、补全和诊断的语言服务。只在终端使用 `scripts/go`，不会自动让编辑器切换工具链，编辑器也需要自己的工作区配置。

`.tools` 不提交。新机器需要先准备项目 Go 工具链，然后在项目根目录安装语言服务：

```sh
editor_os=$(./scripts/go env GOHOSTOS)
editor_arch=$(./scripts/go env GOHOSTARCH)
GOOS="$editor_os" GOARCH="$editor_arch" GOBIN="$PWD/.tools/gopls/bin" \
  ./scripts/go install golang.org/x/tools/gopls@v0.23.0
```

该命令按本机架构安装工具，避免交叉编译设置影响工具安装；不更改业务的 `go.mod`、`go.sum`、全局 gopls 或全局目标架构。首次安装需要联网下载依赖。

## 让编辑器重新加载配置

1. 按 `Command+Shift+P` 打开命令面板。
2. 搜索并执行 `Developer: Reload Window`（开发人员：重新加载窗口）。Go 扩展此前启动失败时，需要重新激活扩展。
3. 打开 `apps/interfaces/http/main.go`，等待 Go 语言服务加载。
4. 按住 `Command` 点击 `flag.String`，或者将光标放到 `String` 上按 `F12`（部分 Mac 键盘需要 `fn+F12`）。

目标是跳到项目 Go 工具链的 `src/flag/flag.go`。进入领域方法或外部依赖时，同样由语言服务解析位置。普通单击通常只移动光标，不执行跳转。

扩展已经正常运行时，可执行 `Go: Restart Language Server`（Go：重启语言服务）重新加载 gopls。若仍无法跳转，查看“输出”面板的 Go 日志，先排查工具启动错误，而非修改业务代码。

## 本次故障原因

2026-10-15 的本机 Go 扩展日志显示，扩展使用系统 Go 1.23.10 执行项目的 `go env`，触发下载 Go 1.27.2，随后出现 `zip: not a valid zip file`。扩展激活失败，因此代码跳转服务没有启动。本机项目内的 Go 1.27.2 已可用，工作区配置直接选择它，避免再次依赖这次失败的自动下载。全局 gopls v0.17.1 则由项目内的新版本替代使用。

参考：[Go 扩展选择工具链](https://github.com/golang/vscode-go/wiki/advanced#choosing-a-different-version-of-go)、[Go 语言服务说明](https://go.dev/gopls/)。
