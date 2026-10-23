# 编码规范与协作流程（jh 分支）

适用范围：`github.com/maczh/mgin` 仓库内所有 Go 代码、配置模板、文档与提交。

---

## 1. 代码风格

| 项 | 规范 |
|----|------|
| 格式化 | `gofmt`（提交前必须执行）；不要手工对齐 |
| 导入 | 标准库 / 第三方 / 本项目 三组，组间空行；`goimports` 整理 |
| 包名 | 小写单词，不用下划线；一个目录一个包 |
| 文件名 | 小写下划线（`access_limit.go`、`mysql_cache.go`） |
| 导出符号 | 必须有注释，首句以符号名开头 |
| 接收者 | 同一类型统一使用同一接收者名（如 `m`/`app`），不要混用 |
| 常量 | 错误码、messageId 集中在 `errcode`；配置键以字符串常量或文档表维护 |
| 魔法值 | 端口、超时、队列长度等必须来自配置或有命名常量 |

反例：

```go
// 不要：可变全局 + init 副作用
var db *gorm.DB
func init() { db, _ = gorm.Open(...) }

// 不要：忽略错误
conn, _ := db.Mysql.GetConnection()
```

---

## 2. 错误处理与响应

### 2.1 统一入口

- HTTP 业务响应统一用 `i18n.Success/SuccessWithPage/Error/ErrorT/ParamLostError`，返回类型是 `models.Result[T]`。
- 业务码使用 `errcode` 常量，禁止裸写 `500`、`-1`、`"error"`。
- HTTP 状态码默认 200；只有协议层语义明确时（401/403/429/503）才使用非 200，并在文档中注明。

```go
// 推荐
if id == "" {
    c.JSON(200, i18n.ParamLostError("id"))
    return
}
c.JSON(200, i18n.Success(user))

// 禁止
c.JSON(500, gin.H{"error": "param error"})
```

### 2.2 错误包装

- 底层错误用 `fmt.Errorf("...: %w", err)` 保留链；不要吞错后返回空值。
- 对外错误信息必须经过 i18n（`messageId`），不要把 SQL/DSN/堆栈返回给客户端。
- `logs.Error` 记录内部细节，响应只给业务码与文案。

### 2.3 nil 与 panic

- `db.Mysql.GetConnection`、`dao.Where/DB/Alias` 可能返回 nil，使用前判空。
- 中间件/框架代码不要 `panic` 作为控制流；唯一例外是 `i18n.Init`/VictoriaLogs 初始化失败（当前实现会 panic）。
- 处理用户输入的函数要能接受空字符串、超长字符串与非法 UTF-8。

---

## 3. 日志规范

```go
logs.Info("加载{}成功, 耗时{}ms", name, cost)
logs.Error("查询失败: {}", err.Error())
```

- **必须**使用 `{}` 占位符，按顺序替换；不要 `logs.Info(fmt.Sprintf(...))`。
- 级别：`Debug` 调试细节、`Info` 生命周期、`Warn` 可恢复异常、`Error` 需要人工关注。
- 不打印密码、token、完整 DSN、身份证/手机号等敏感信息；必要时脱敏。
- 高频路径（每请求）避免 `Info` 级别刷屏；访问日志由 `postlog` 统一采集，不要重复记录。
- 需要落外部系统时用 `logsink` 自定义 handler，不要在业务里直接发 HTTP。

---

## 4. 数据访问规范

1. 业务只通过 `db` 门面或 `db/dao` 访问数据库，不直接 `gorm.Open`。
2. DAO 的泛型参数必须实现 `TableName()`；表名与 `models` 保持一致。
3. 并发场景不要复用同一个 `MySQLDao` 实例调用 `WithContext`（它会修改接收者），先复制。
4. 更新必须带条件：使用 `Where(...).Updates(...)` 或 `DB()`，不要依赖 `Updates(entity)`。
5. 事务内所有操作必须使用 `tx`；不要在事务回调里 `db.Mysql.GetConnection()` 另开连接。
6. 多库模式下所有调用都要传库名；新增跨库逻辑需在评审中说明。
7. 缓存：显式设置 `CacheConfig.TTL`；不要用 0（永久）；写操作后确认缓存失效。

---

## 5. 新增配置键清单

新增一个配置键，必须同时完成：

- [ ] `config/configure.go`：在对应结构体加字段并在 `Init` 中读取，给出默认值。
- [ ] `application.example.yml`：给出带注释的示例。
- [ ] `README.md` §5：加入配置表。
- [ ] 对应设计文档（`docs/architecture.md` 或 `docs/data-design.md`）。
- [ ] 若为开关：默认值必须保持**旧行为**（默认关闭/兼容）。
- [ ] 单元测试覆盖默认值与显式值。

命名约定：`go.<模块>.<键>`，驼峰；布尔开关用 `enabled` 结尾；时间单位写进键名或注释（`timeout` 秒、`life` 分钟）。

---

## 6. 新增组件 / 中间件 / 表

### 6.1 组件（数据源、消息队列等）

1. 在 `db/`（或独立包）实现 `Init([]byte)/Close()/Check() error`。
2. 在 `db/db.go` 暴露门面（如需）。
3. 在 `mgin.go` 的 `initResources`、`closeResources`、`checkAll`、`SafeExit` 中登记（四处缺一不可）。
4. 在 `config` 的 `Prefix` 结构与 `Init` 中登记前缀键。
5. 更新 README §2 功能表与 §5 配置表、`docs/modules-and-api.md`。
6. `used` 未包含时不得产生任何连接或协程。

### 6.2 中间件

1. 放在 `middleware/<name>/`，导出构造函数（如 `Xxx() gin.HandlerFunc`）。
2. 是否启用由配置或调用方决定；默认不改变既有行为。
3. 明确失败响应（HTTP 状态 + 响应体），并在 `docs/modules-and-api.md` 的中间件表登记。
4. 需要鉴权上下文时，说明依赖（如 casbin 依赖 jwt 写入的 `claims`）。
5. 提供单元测试（可用 `httptest`）。

### 6.3 数据表

1. 模型放 `models/` 或 `models/sys/`，实现 `TableName()`。
2. 明确建表归属：框架迁移（如 job）还是业务迁移（如 sys）。默认**业务迁移**。
3. 在 `docs/data-design.md` 补字段表、唯一索引与生命周期。
4. 敏感字段（密码、token）说明加密/脱敏方式。

---

## 7. Git 工作流

### 7.1 分支

| 分支 | 用途 |
|------|------|
| `jh` | 当前定制主线，禁止直接在其上开发大功能 |
| `master` | 上游通用线（模块 `/v2`），与 `jh` 不合并 |
| `feat/<topic>` | 新功能 |
| `fix/<topic>` | 缺陷修复 |
| `docs/<topic>` | 文档 |
| `hotfix/<version>` | 线上紧急修复，合并后打 tag |

### 7.2 提交信息

历史提交以「1. xxx 2. xxx」列举为主。新增提交统一使用：

```
<type>(<scope>): <subject>

<body: 动机、影响面、兼容性>
<footer: 关联 issue / BREAKING CHANGE>
```

`type`：`feat`、`fix`、`refactor`、`perf`、`test`、`docs`、`build`、`chore`。
示例：

```
feat(ratelimit): 支持按 header 维度限流

- 新增 dimension=header，键取自 rules[].headerKey
- 未配置 header 时回退 global，保持旧行为
```

要求：

- 一个提交只做一件事；格式化与逻辑修改分开提交。
- 不提交 `logs/`、二进制、`conf/*.db`、真实密钥、`.Trash-0/`。
- 提交前 `git status` 检查是否混入生成物。

### 7.3 Pull Request

PR 描述必须包含：

1. 变更动机与方案摘要；
2. 影响面（配置键、协议、表结构、兼容性）；
3. 验证命令与结果（测试输出、curl 结果）；
4. 文档更新清单（README/docs）；
5. 回滚方式（如涉及数据/配置）。

---

## 8. 代码评审清单

**正确性**

- [ ] 边界：空值、超长、并发、重复调用、未初始化。
- [ ] 错误路径有日志且不泄露内部信息。
- [ ] 配置默认值保持旧行为；开关关闭时无副作用。

**安全**

- [ ] 不硬编码密钥；token/密码不落日志。
- [ ] 外部输入经过校验；不把 `CheckSqlValidate` 当注入防护。
- [ ] 新增公开接口考虑鉴权与限流；重载类接口限制来源 IP + token。
- [ ] 加密不使用 ECB；JWT 校验算法白名单。

**性能与资源**

- [ ] 协程有退出路径；`ticker`/`time.After` 不泄漏。
- [ ] 数据库连接/游标/响应体正确关闭。
- [ ] 缓存键带前缀与 TTL；避免大 key、全库扫描。

**可维护性**

- [ ] 命名与术语符合 [术语表](../02-concepts/04-requirements.md#8-术语表)。
- [ ] 新增配置键/能力已更新文档。
- [ ] 有对应测试；测试可在无外部服务时运行或明确跳过。

---

## 9. 文档规范

- 中文文档使用全角标点；代码、键名、路径用反引号。
- 每篇文档开头给出：适用分支、读者、关联文档。
- 描述行为时给出**代码位置或验证命令**，避免「应该」「可能」。
- 已过时的章节不要保留模糊表述：要么更新，要么标注「本分支未实现」。
- 新增文档必须在 `README.md` §10 文档索引中登记。

---

## 10. 安全红线

1. 禁止把生产配置、证书、token 提交到仓库；示例配置只放占位符。
2. 新增对外接口默认需要鉴权；公开接口要在评审中说明理由。
3. 不引入已知停止维护的加密/JWT 库的新用法（现有 `dgrijalva/jwt-go` 仅维护兼容）。
4. XSS 默认 `StrictPolicy`；确需富文本时在评审中说明并限定字段。
5. 日志、错误响应、监控标签中不得出现个人敏感信息。
