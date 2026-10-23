# 国际化与错误码

> 本文件由脚本从源码 AST 生成，签名与字段与当前代码一致；新增导出符号后请重新生成。
>
> 错误码定义、HTTP 映射、i18n 字典与文案格式化。

## `github.com/maczh/mgin/v2/pkg/i18n`

```go
import "github.com/maczh/mgin/v2/pkg/i18n"
```

### 变量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `ErrNotInitialized` | = errors.New("i18n: dictionary is not initialized") |  |
| `ErrInvalidEntry` | = errors.New("i18n: invalid dictionary entry") |  |
| `ErrNotFound` | = errors.New("i18n: dictionary entry not found") |  |
| `ErrConflict` | = errors.New("i18n: messageId and language already exist") |  |

### 类型

#### `Entry`

```go
type Entry struct
```

Entry stores one translation. A message can have one entry per language.

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `MessageID` | string | json:"messageId" gorm:"column:message_id;size:191;primaryKey;autoIncrement:false" |  |
| `Language` | string | json:"language" gorm:"size:35;primaryKey;autoIncrement:false" |  |
| `Text` | string | json:"text" gorm:"type:text;not null" |  |
| `CreatedAt` | time.Time | json:"createdAt" |  |
| `UpdatedAt` | time.Time | json:"updatedAt" |  |

#### `Store`

```go
type Store struct
```

Store uses the application's relational connection, or owns a fallback SQLite connection.

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|

**方法**

```go
func (s *Store) TableName() string
```

```go
func (s *Store) Create(ctx context.Context, entry *Entry) error
```

```go
func (s *Store) Get(ctx context.Context, messageID, language string) (*Entry, error)
```

```go
func (s *Store) Update(ctx context.Context, entry *Entry) error
```

Update changes only the translation text; the key is immutable.

```go
func (s *Store) Delete(ctx context.Context, messageID, language string) error
```

```go
func (s *Store) List(ctx context.Context, options ListOptions) ([]Entry, int64, error)
```

```go
func (s *Store) Translate(ctx context.Context, messageID, language string) (string, error)
```

Translate falls back to the configured default language. A missing key returns ErrNotFound.

#### `ListOptions`

```go
type ListOptions struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `MessageID` | string |  |  |
| `Language` | string |  |  |
| `Page` | int |  |  |
| `PageSize` | int |  |  |

### 函数

#### `CheckParametersLost`

```go
func CheckParametersLost(params map[string]string, paramNames ...string) models.Result[any]
```

#### `Close`

```go
func Close()
```

Close releases only the SQLite connection owned by i18n.

#### `Error`

```go
func Error(code int, messageId string) models.Result[any]
```

#### `ErrorDef`

```go
func ErrorDef(def errcode.Definition, args ...any) models.Result[any]
```

ErrorDef v2.1 新增：基于 errcode.Definition 的标准化错误响应。

把三向映射（errcode.Definition → i18n 文案 → HTTP 状态码）一步完成：
- 文案通过 def.MessageKey + args 走 i18n.Format
- 业务码写入返回体的 Code 字段
- HTTP 状态码由 def.HTTPStatus 决定（调用方需在自己的 handler 里手动 c.JSON(def.HTTPStatus, result)）

返回的 result 是已渲染文案的 models.Result，业务侧调用方式：

c.JSON(errcode.LookupDef(errcode.PARAM_ERROR).HTTPStatus,
i18n.ErrorDef(errcode.PARAM_ERROR, paramName))

#### `ErrorDefT`

```go
func ErrorDefT[T any](def errcode.Definition, args ...any) models.Result[T]
```

ErrorDefT 泛型版，行为同 ErrorDef。

#### `ErrorT`

```go
func ErrorT[T any](code int, messageId string) models.Result[T]
```

#### `ErrorWithMsg`

```go
func ErrorWithMsg(code int, messageId, msg string) models.Result[any]
```

#### `ErrorWithMsgT`

```go
func ErrorWithMsgT[T any](code int, messageId, msg string) models.Result[T]
```

#### `Format`

```go
func Format(messageId string, args ...any) string
```

Format 格式化数据，messageId对应的内容为带{}的模板

#### `GetStore`

```go
func GetStore() (*Store, error)
```

#### `GetString`

```go
func GetString(messageID, language string) string
```

GetString resolves a message from the built-in dictionary with language fallback.

#### `GetXLangString`

```go
func GetXLangString(messageID, language string) string
```

GetXLangString is kept for source compatibility and uses the local dictionary.
Deprecated: use GetString.

#### `Init`

```go
func Init() error
```

Init initializes the built-in database dictionary; no external service is used.

#### `NewStore`

```go
func NewStore(conn *gorm.DB, appName string) (*Store, error)
```

NewStore creates the dictionary table on an existing GORM connection.
The caller retains ownership of the connection.

#### `ParamError`

```go
func ParamError(param string) models.Result[any]
```

#### `ParamLostError`

```go
func ParamLostError(param string) models.Result[any]
```

#### `RouterGroup`

```go
func RouterGroup(group *gin.RouterGroup)
```

RouterGroup mounts dictionary CRUD on a caller-supplied group.
GET /list lists entries; GET /get and POST /delete use messageId/language query parameters;
POST /save and POST /update accept {"messageId":"hello","language":"en","text":"Hello"}.

#### `String`

```go
func String(messageId string) string
```

String 将messageId根据当前协程X-Lang参数转换成当前语言字符串

#### `Success`

```go
func Success[T any](data T) models.Result[T]
```

#### `SuccessWithPage`

```go
func SuccessWithPage[T any](data T, count, index, size, total int) models.Result[T]
```

## `github.com/maczh/mgin/v2/pkg/errcode`

```go
import "github.com/maczh/mgin/v2/pkg/errcode"
```

### 常量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `URI_NOT_FOUND` | = 1000 |  |
| `SYSTEM_ERROR` | = 1001 |  |
| `DB_CONNECT_ERROR` | = 1002 |  |
| `REQUEST_PARAMETER_LOST` | = 1003 |  |
| `DATA_NOT_FOUND` | = 1004 |  |
| `USER_NOT_FOUND` | = 1005 |  |
| `PASSWORD_ERROR` | = 1006 |  |
| `VERIFY_CODE_ERROR` | = 1007 |  |
| `TOKEN_ERROR` | = 1008 |  |
| `AUTHENTICATION_FAILURE` | = 1009 |  |
| `SERVICE_UNAVAILABLE` | = 1010 |  |
| `PARAM_ERROR` | = 1014 |  |
| `TOO_MANY_REQUESTS` | = 1011 |  |
| `JOB_ERROR` | = 1012 |  |
| `STORAGE_ERROR` | = 1013 |  |
| `ModuleCommon` | = "common" | Module 业务模块常量：用于错误码分组与文档归类。 v2 新增。v1 时代的错误码常量（URI_NOT_FOUND=1000 等）不重新分配模块，保持兼容。 |
| `ModuleAuth` | = "auth" | Module 业务模块常量：用于错误码分组与文档归类。 v2 新增。v1 时代的错误码常量（URI_NOT_FOUND=1000 等）不重新分配模块，保持兼容。 |
| `ModuleUser` | = "user" | Module 业务模块常量：用于错误码分组与文档归类。 v2 新增。v1 时代的错误码常量（URI_NOT_FOUND=1000 等）不重新分配模块，保持兼容。 |
| `ModuleDatabase` | = "database" | Module 业务模块常量：用于错误码分组与文档归类。 v2 新增。v1 时代的错误码常量（URI_NOT_FOUND=1000 等）不重新分配模块，保持兼容。 |
| `ModuleJob` | = "job" | Module 业务模块常量：用于错误码分组与文档归类。 v2 新增。v1 时代的错误码常量（URI_NOT_FOUND=1000 等）不重新分配模块，保持兼容。 |
| `ModuleStorage` | = "storage" | Module 业务模块常量：用于错误码分组与文档归类。 v2 新增。v1 时代的错误码常量（URI_NOT_FOUND=1000 等）不重新分配模块，保持兼容。 |
| `ModuleClient` | = "client" | Module 业务模块常量：用于错误码分组与文档归类。 v2 新增。v1 时代的错误码常量（URI_NOT_FOUND=1000 等）不重新分配模块，保持兼容。 |
| `ModulePlugin` | = "plugin" | Module 业务模块常量：用于错误码分组与文档归类。 v2 新增。v1 时代的错误码常量（URI_NOT_FOUND=1000 等）不重新分配模块，保持兼容。 |
| `UrlNotFound` | = "404" |  |
| `SystemError` | = "系统异常" |  |
| `DbConnectErr` | = "数据库连接失败" |  |
| `DbInsertErr` | = "数据库插入失败" |  |
| `DbUpdateErr` | = "数据库更新失败" |  |
| `DbDeleteErr` | = "数据库删除失败" |  |
| `DataNotFound` | = "数据库查无数据" |  |
| `ParamLost` | = "参数不可为空" |  |
| `ParamError` | = "参数错误" |  |
| `ConnectFail` | = "网络连接失败" |  |
| `ServiceUnavailable` | = "服务不存在" |  |
| `Success` | = "success" |  |
| `DbQueryErr` | = "数据库查询失败" |  |
| `TooManyRequests` | = "请求过于频繁，请稍后再试" |  |
| `JobError` | = "定时任务执行失败" |  |
| `StorageError` | = "对象存储操作失败" |  |

### 类型

#### `Definition`

```go
type Definition struct
```

Definition v2 新增：一个标准化的"业务错误定义"。

字段含义：
- Code:       业务错误码（int），与 v1 常量同空间（1000+，与 HTTP status 错开）
- HTTPStatus: HTTP 状态码（int）。v1 时代所有错误都 200，v2 起按定义走正确状态码
- MessageKey: i18n 翻译键（如 "errcode.uri_not_found"），用于 ErrorDef 自动取文案
- Module:     业务模块，便于按模块聚合
- Args:       可选，预留默认占位参数

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Code` | int |  |  |
| `HTTPStatus` | int |  |  |
| `MessageKey` | string |  |  |
| `Module` | string |  |  |

### 函数

#### `AllDefs`

```go
func AllDefs() map[int]Definition
```

AllDefs 导出全部已注册 Definition，主要给 docs / OpenAPI 生成器使用。

#### `LookupDef`

```go
func LookupDef(code int) Definition
```

LookupDef 按 v1 错误码常量查 v2 Definition。未注册则返 500 + SYSTEM_ERROR。

#### `New`

```go
func New(module string, code, httpStatus int, messageKey string) Definition
```

New 构造一个 Definition。HTTPStatus 必须为 0 或一个合法 HTTP 状态码，
非法值会被自动修正为 500（服务端错误），避免误返 2xx 给客户端。

#### `RegisterDef`

```go
func RegisterDef(d Definition)
```

RegisterDef 把自定义 Definition 注册到全局映射（业务侧可在 init() 中调用）。
同 code 后注册会覆盖前者。

