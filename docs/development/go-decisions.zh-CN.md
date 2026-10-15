# 本项目 Go 具体约定

本文件记录项目自己的工程约定，是对 Google Go 规范的补充，不代表 Google 官方规则。核心原则、具体语言规则和辅助建议的层级仍按 [规范阅读顺序](README.md#go-规范阅读顺序) 执行。

## 导入分组与顺序

手写 Go 文件的 `import` 按以下顺序分为三组，组间空一行：

1. **标准库（系统）**：Go 标准库，例如 `context`、`net/http`、`time`。
2. **本项目包（本地）**：当前模块 `sakelei/ai` 及其子包，例如 `sakelei/ai/apps/config`、`sakelei/ai/domain/agent`。
3. **外部依赖（第三方）**：当前模块以外的依赖，例如 `github.com/gin-gonic/gin`、`google.golang.org/protobuf/proto`。Go 官方维护但不属于标准库的模块，例如 `golang.org/x/sync`，也属于这一组。

```go
import (
	"context"
	"net/http"
	"time"

	"sakelei/ai/apps/config"
	"sakelei/ai/domain/agent"

	"github.com/gin-gonic/gin"
)
```

- 没有导入项的组直接省略，不保留空组；只有一个导入项时可使用单行 `import`。
- 组内按导入路径排序，交给 `gofmt` 处理；保留确有必要的导入别名，不根据别名或业务功能再拆组。
- `gofmt` 会保留组间空行，不会自动按上述类别重新分组，添加导入时需要确认分组和顺序。
- 工具生成的文件沿用生成器输出，不手工修改 `.pb.go` 等生成文件的导入分组。

“标准库 → 本项目包 → 外部依赖”是本项目统一选择的顺序。Google 规范中的一般导入要求见 [Google Go 具体规范：导入](google-go-decisions.zh-CN.md#三导入)。
