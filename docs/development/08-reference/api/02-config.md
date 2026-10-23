# 配置

> 本文件由脚本从源码 AST 生成，签名与字段与当前代码一致；新增导出符号后请重新生成。
>
> 配置加载、查询与远程配置中心。

## `github.com/maczh/mgin/v2/pkg/config`

```go
import "github.com/maczh/mgin/v2/pkg/config"
```

### 常量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `DefaultShutdownTimeout` | = 5 * time.Second | DefaultShutdownTimeout 优雅关闭的默认超时时间。 未配置 go.application.shutdownTimeout 或配置值非法（<=0）时使用该值， 与升级前硬编码的 5 秒保持一致，确保向后兼容。 |

### 变量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `Config` | = &config{} |  |

### 类型

#### `LogOutput`

```go
type LogOutput struct
```

LogOutput configures one application log destination. External targets name a registered RecordHandler.

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Target` | string | json:"target" |  |
| `Format` | string | json:"format" |  |
| `Fields` | []string | json:"fields" |  |
| `LogText` | string | json:"logtext" |  |
| `File` | string | json:"file" |  |

#### `VictoriaLogsConfig`

```go
type VictoriaLogsConfig struct
```

VictoriaLogsConfig configures the built-in HTTPS collector.

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `URL` | string | json:"url" |  |
| `APIToken` | string | json:"api_token" koanf:"api_token" |  |
| `APICert` | string | json:"api_crt" koanf:"api_crt" |  |
| `Request` | bool | json:"request" |  |
| `Client` | bool | json:"client" |  |
| `Timeout` | int | json:"timeout" |  |

