# MGin v2 编码规范与协作流程

适用范围：`github.com/maczh/mgin/v2` 全部代码（根包、`pkg/`、`logs/`、`cmd/`）。
规范以**现有代码风格为基线**，新增代码必须与之一致；与规范冲突的历史代码不强制批量重写，
但修改该文件时应顺带对齐。

---

## 1. 基础规范

### 1.1 格式化与静态检查

| 项 | 要求 |
|----|------|
| 格式化 | `gofmt`（不使用 `goimports -local` 强制分组，但 import 顺序须 gofmt 化） |
| 静态检查 | 新增/修改代码不得引入新的 `go vet` 告警 |
| 可选 | `golangci-lint run --timeout=5m`（仓库未提供配置，使用默认规则） |
| 命名 | 遵循 Go 官方 Effective Go；缩写词全小写或全大写（`URL`、`ID`、`HTTP`），不要 `Url`/`Id` |
| 文件名 | 小写、下划线分词：`mysql_cache.go`、`ratio_limit.go`；测试 `*_test.go` |

**基线事实**：`go build ./...` 通过；`go vet ./...` 有 3 处历史告警
（`pkg/middleware/xss/xss.go:513`、`pkg/utils/rsautil.go:369`、`pkg/utils/rsautil.go:406`）；
`gofmt -l .` 会列出 14 个历史文件。因此门禁只针对**本次改动**：

```bash
gofmt -l $(git diff --name-only -- '*.go')   # 必须无输出
go build ./...
go vet ./pkg/<你改动的包>/...
```

不要为了清零历史告警而夹带无关重构（应单独提 PR）。

### 1.2 注释

- 导出符号必须有注释，首词为符号名：`// NewApp 创建一个新的 MGin App 实例。`
- 注释使用中文（与现有代码一致），专有名词保留英文。
- 包注释写在 `doc.go` 或首个文件顶部，说明职责、边界与已知限制。
- **不要**写"这是什么"的废话注释；解释"为什么"、约束、坑位。
- 已废弃 API 用 `// Deprecated: 使用 Xxx 替代。`，并在文档记录删除版本。

### 1.3 包与目录

| 规则 | 说明 |
|------|------|
| 新能力放 `pkg/` | 禁止新增顶层包（`logs/` 兼容壳除外） |
| 框架私有实现放 `internal/` | 仅本 module 可 import |
| 不循环依赖 | `pkg/*` 不得 import 根包；根包负责装配 |
| 一包一职责 | 不建 `common`/`util` 大杂烩；工具统一进 `pkg/utils` 并按主题分文件 |
| 子包命名 | 与目录一致，避免 `pkg/foo/foo` 冗余（如 `pkg/db/mysql` 内 `package mysql`） |

### 1.4 依赖

- 新增第三方依赖需在 PR 说明理由、许可证、维护状态；优先标准库。
- 不在 `pkg/*` 中引入 `cmd/` 或示例代码。
- 框架公共 API 不暴露第三方类型，除非该类型是契约的一部分
  （例如 `client.Options.Files []grequests.FileUpload`、`db.GetConnection() (*gorm.DB, error)` 属既有例外）。
- 禁止引入 OTel SDK/Exporter 到框架依赖；由业务侧接入。

---

## 2. 命名与 API 设计

### 2.1 命名

| 类型 | 规范 | 示例 |
|------|------|------|
| 包 | 小写单词，不用下划线 | `loadbalancer`、`logsink` |
| 接口 | 名词或 `-er`；单方法接口用 `-er` | `Plugin`、`LoadBalancer`、`RegistryClient` |
| 构造函数 | `NewXxx`；返回接口时返回接口 | `NewCircuitBreaker`、`NewCachedRegistry` |
| 布尔 | `Is/Has/Enable/Enabled` | `IsStarted`、`Enabled()` |
| 错误变量 | `ErrXxx` | `ErrAllInstancesCircuitOpen` |
| 常量 | 包内 `CamelCase`；语义分组用括号块 | `StatusOK`、`AlgoTokenBucket` |
| 配置结构 | 与 YAML 键对应，字段名与 tag 分开 | `ShutdownTimeout` ↔ `shutdownTimeout` |

### 2.2 兼容性规则

1. **不删除、不改签名**已发布的导出符号；新增能力用新函数/新字段。
2. 结构体新增字段必须是**可选**的，零值语义与原行为一致。
3. 接口新增方法视为破坏性变更，需大版本 + 迁移文档。
4. 行为变化（如 HTTP 状态码从 200 改为 404）必须在 CHANGELOG 的 **Changed** 中标注。
5. 兼容壳（如 `logs/`）必须写清删除版本。

### 2.3 选项模式

- 参数超过 3 个或未来会扩展时，用 `Options` 结构体而非长参数列表。
- 选项字段必须有零值语义，并在注释写默认行为。
- 例：`client.Options`、`client.ResilienceOptions`、`dao.CacheConfig`。

---

## 3. 错误处理

### 3.1 原则

1. **不忽略错误**：禁止 `_ = doSomething()` 除非明确无害并在注释说明。
2. **包装保留链**：`fmt.Errorf("get database connection: %w", err)`。
3. **错误信息小写开头、无标点**（英文）；中文错误串保持现状但避免拼接用户输入。
4. **不 panic 作为控制流**：仅初始化不可恢复（如 `logs.InitVictoriaLogs` 失败）才 panic。
5. **边界收敛**：库函数返回 error；只在 `main`/`Run` 层决定退出。

### 3.2 业务错误

统一使用 `errcode.Definition` 三向映射：

```go
var ErrOrderNotFound = errcode.New("order", 4101, 404, "order.not_found")

func GetOrder(c *gin.Context) {
    def := ErrOrderNotFound
    c.JSON(def.HTTPStatus, i18n.ErrorDef(def, c.Param("id")))
}
```

要求：

- `Code` 业务码全局唯一，建议按模块分段（1000–1999 框架内置，2000+ 业务）。
- `HTTPStatus` 必须真实（`errcode.New` 会把非法值修正为 500）。
- `MessageKey` 必须写入 i18n 字典，否则返回 key 原文。

### 3.3 错误日志

- 记录错误时附上下文（服务名、库名、地址），不要只打 `err.Error()`。
- 同一错误只在一层记录，避免重复刷屏。
- 对外响应不暴露内部错误细节（SQL、堆栈）。

---

## 4. 日志规范

### 4.1 占位符

框架使用 `{}` 顺序占位，**不是** `%s`：

```go
logs.Info("正在连接{}，超时{}秒", name, timeout)   // 正确
logs.Info("正在连接%s", name)                     // 错误：不会被替换
```

- 参数数量必须与 `{}` 数量一致；多余的 `{}` 会保留原文。
- 结构化字段优先：避免把整个结构体拼进字符串（可读性与检索都差）。
- 日志级别：`Debug`（调试细节）、`Info`（关键流程）、`Warn`（可恢复异常）、`Error`（需处理）。

### 4.2 敏感信息

- 禁止打印密码、Token、完整连接串、身份证/手机号明文。
- 请求体日志需评估合规风险；生产建议 `go.log.get: off` 或脱敏。

---

## 5. 并发与资源

| 规则 | 说明 |
|------|------|
| 全局状态加锁 | map/切片用 `sync.RWMutex`；计数用 `atomic` |
| 协程必须有退出路径 | ticker/loop 用 `done chan struct{}`，`Stop` 时关闭 |
| channel 容量 | 信号通道至少缓冲 1（参考 `signalChan`）；超时 channel 缓冲 1 防泄漏 |
| 不裸起 goroutine | 起之前想清楚谁回收、panic 谁兜住 |
| 资源释放 | DB 连接、Mongo `ReturnConnection`、文件、`ticker.Stop()`、`cancel()` |
| 上下文 | 长耗时/可取消操作首参 `ctx context.Context`；不要存进结构体长期持有 |
| 锁内不做 IO | 锁只保护内存状态 |

示例（后台循环的正确关闭）：

```go
go func() {
    for {
        select {
        case <-ticker.C:
            doWork()
        case <-done:
            return
        }
    }
}()
```

---

## 6. 中间件与 HTTP 规范

1. 中间件构造函数返回 `gin.HandlerFunc`，命名 `Xxx()`（如 `TraceId()`、`RateLimit()`）。
2. 中间件内使用 `c.Set` 存请求级数据，键名统一小写点分（`claims`、`lang`）。
3. 鉴权失败：401（未认证）/ 403（无权限），不要一律 200。
4. 探针/指标路由**必须**在业务 `Use` 之前注册。
5. 新增全局中间件需评估对探针的影响；会拦截请求的中间件不得在探针之前。
6. 响应统一 `models.Result[T]` 或 `vo.Response[T]`，不混用多套包装。

---

## 7. 配置规范

- 键名统一挂在 `go.*` 下，层级不超过 4 段。
- 驼峰命名与现有键一致（`shutdownTimeout`、`callType`），不要混用下划线（历史 `port_ssl`、`model_file` 例外）。
- 每个键必须在 `pkg/config` 中**显式读取**，并同步 README 第 6 节。
- 缺失配置必须有合理默认值，不得 panic。
- 不在包级 `init()` 读配置（配置可能尚未加载）；在 `Init`/构造时读取。

```go
// 推荐
if config.Config.GetConfigBool("go.health.enabled") { ... }

// 不推荐：包级变量在配置加载前求值
var healthEnabled = config.Config.GetConfigBool("go.health.enabled")
```

---

## 8. 测试规范（摘要）

详细策略见 [testing-strategy.md](testing-strategy.md)。

- 新增/修改逻辑必须有单测；纯转发/装配代码可豁免。
- 表驱动 + `t.Run`；错误路径与边界优先。
- 不依赖真实网络的测试用接口桩（参考 `client_test.go` 的 `testRegistry`）。
- 依赖外部中间件的测试用 `t.Skip` 守卫，CI 无服务时跳过而非失败。
- 并发代码跑 `-race`。

---

## 9. 文档规范

| 改动 | 必须更新的文档 |
|------|----------------|
| 新增/修改公开 API | `docs/modules-and-api.md`、README 示例 |
| 新增/修改配置键 | README §6、`application.example.yml`（若适用） |
| 表结构变化 | `docs/data-design.md` |
| 生命周期/架构变化 | `docs/architecture.md`、ADR 摘要 |
| 新增能力 | `docs/requirements.md` 的 FR 条目与追踪矩阵 |
| 修复已知缺陷 | README §9 / `docs/architecture.md` 债务表 |

文档要求：

- 术语统一：插件 / 注册中心 / 负载均衡器 / 探针 / 业务错误码 / HTTP 状态码。
- 签名必须可被 `go doc` 验证；不写"大概是这样"。
- 未实现能力必须标注 ❌，不得作为可用能力描述。

---

## 10. 协作流程

### 10.1 分支模型

| 分支 | 用途 | 规则 |
|------|------|------|
| `v2` | 当前主线 | 受保护；仅通过 PR 合入 |
| `feat/<topic>` | 新功能 | 从 `v2` 拉出 |
| `fix/<topic>` | 缺陷修复 | 从 `v2` 拉出 |
| `docs/<topic>` | 文档 | 可只改 Markdown |
| `release/vX.Y` | 发布准备 | 只接受修复与版本号/CHANGELOG |

### 10.2 提交信息

推荐 Conventional Commits（与仓库现有中英混合风格兼容）：

```
feat(client): 支持按调用指定负载均衡策略
fix(app): HTTPS server 未纳入优雅关闭
docs(readme): 修正健康探针配置键
refactor(plugin): 用注册表替代 if 链
test(loadbalancer): 补充一致性哈希用例
chore(deps): 升级 gin 到 v1.11.0
```

要求：

- 标题 ≤ 72 字符，动词开头，说明"做了什么"。
- 正文说明动机、影响面、兼容性（破坏性变更以 `BREAKING CHANGE:` 开头）。
- 一个提交只做一件事；格式化与功能改动分开。
- 不提交生成物、二进制、日志、`.idea/`。

### 10.3 PR 流程

1. 自查：改动文件 `gofmt` / `go build ./...` / 改动包的 `go vet` / 相关测试。
2. 填写 PR 模板（建议）：

```markdown
## 动机
## 变更内容
## 兼容性（是否破坏 API/配置/行为）
## 测试方式（命令与结果）
## 文档更新
```

3. 至少 1 名维护者评审；涉及生命周期/兼容性需 2 名。
4. 评审通过后 squash 合并，保持线性历史。

### 10.4 评审清单

- [ ] 是否有未处理的 error / 未关闭的资源 / 泄漏的 goroutine
- [ ] 全局状态是否加锁；并发路径是否 race
- [ ] 日志占位符是否成对；是否泄露敏感信息
- [ ] 新配置是否被真实消费（避免"配了不生效"）
- [ ] 公开 API 是否破坏兼容；是否更新文档
- [ ] 探针/指标路由位置是否仍在所有 `Use` 之前
- [ ] 测试是否覆盖失败路径
- [ ] 是否存在"文档声称但代码没有"的能力

### 10.5 缺陷修复流程

1. 先写复现用例（能失败的测试）。
2. 最小改动修复，不夹带重构。
3. 在 PR 描述附"修复前后行为对比"。
4. 若属线上事故，补充到 `docs/architecture.md` 债务表或 CHANGELOG 的 Fixed。

---

## 11. 反模式清单（本项目已出现，禁止扩散）

| 反模式 | 现状 | 正确做法 |
|--------|------|----------|
| 文档声称未实现能力 | README 曾写 S3/Postgres | 只写代码可验证的能力 |
| 配置键无消费者 | `go.framework.*` 多数未接线 | 要么接线，要么删除并在启动日志告警 |
| 子串匹配启用组件 | `strings.Contains(used, "mysql")` | 切分逗号后精确比较 |
| 静默回落 | LB 名称未命中回落轮询且无日志 | 记录 Warn 并暴露指标 |
| 接口无实现 | `Dao[E]` | 删除或补齐实现，避免误导 |
| Check 恒 nil | Redis/SQLite | 实现真实 Ping |
| 重复挂载 panic 风险 | 路由重复注册 | 用 `xxxMounted` 去重 |
| 无 CI | 仓库无流水线 | 至少跑 build/vet/test 三件套 |
