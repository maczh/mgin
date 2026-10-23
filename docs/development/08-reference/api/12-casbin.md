# 鉴权策略

> 本文件由脚本从源码 AST 生成，签名与字段与当前代码一致；新增导出符号后请重新生成。
>
> Casbin 模型与角色-接口策略维护。

## `github.com/maczh/mgin/v2/pkg/casbin`

```go
import "github.com/maczh/mgin/v2/pkg/casbin"
```

### 变量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `Casbin` | = &casbinService{ UnAuthPath: make([]CasbinInfo, 0), } |  |

### 类型

#### `CasbinInfo`

```go
type CasbinInfo struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Path` | string |  |  |
| `Method` | string |  |  |

