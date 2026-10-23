# MGin v2 版本发布与变更记录规范

| 项 | 内容 |
|----|------|
| 文档版本 | v2.0 |
| 适用对象 | 框架仓库、`cmd/` 脚手架、生成的业务工程 |
| 版本体系 | 语义化版本（SemVer 2.0.0） |
| 当前状态 | 仓库无 CI、无 `CHANGELOG.md`；本文定义规范，落地需新增文件与流水线 |

---

## 1. 版本号规范

### 1.1 格式

```
vMAJOR.MINOR.PATCH[-prerelease][+build]
例：v2.0.0、v2.1.0、v2.1.1-rc.1
```

| 位 | 何时递增 | 示例 |
|----|----------|------|
| MAJOR | 破坏公开 API、import 路径、配置语义或行为契约 | 顶层包迁到 `pkg/`（v1 → v2） |
| MINOR | 新增向后兼容能力、新包、新配置项、新错误码 | 新增 `pkg/loadbalancer`、`/metrics` |
| PATCH | 向后兼容的缺陷修复、文档修正、依赖补丁 | 修复 HTTPS 未关闭、修正 LB 名称 |

> 预发布：`-rc.N`（候选）、`-beta.N`（公测）。预发布版本允许不兼容，但需在
> CHANGELOG 明确说明。

### 1.2 Go module 标签

- 模块路径已含 `/v2`，标签必须是 `v2.x.y`（**不能**是 `v2.1.0` 之外的 `2.1.0`）。
- 同一 commit 打标签后不可移动；修复必须发新 PATCH。
- `cmd/` 是嵌套 module（`github.com/maczh/mgin/v2/cmd/mgin`），发布时需在
  `cmd/` 目录单独打前缀标签：`cmd/mgin/v2.1.0`。
- 预发布标签同样带前缀：`cmd/mgin/v2.1.0-rc.1`。

### 1.3 三类版本载体（必须一致）

| 载体 | 位置 | 更新方式 |
|------|------|----------|
| Go module 版本 | git tag | `git tag v2.x.y` |
| 脚手架工具版本 | `cmd/templates.go` 的 `toolVersion` | 手工改（当前 `2.1.0`） |
| 生成工程版本 | `main.Version` / `main.BuildTime` / `main.GitHash` | Makefile `-ldflags` 注入 |
| 框架运行时元数据 | `config.Config.Runtime` | 启动时填 `StartedAt/Pid/GoVersion`；`CommitHash/BuildTime` 待业务注入 |

> `Config.Runtime.CommitHash/BuildTime` 当前**不会**被框架自动填充（`mgin.go` 注释已说明）。
> 若需要，业务在 `NewApp` 之后自行赋值。

---

## 2. 变更分类（用于 CHANGELOG）

遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 的分类：

| 分类 | 含义 | 版本影响 |
|------|------|----------|
| `Added` | 新能力、新 API、新配置、新错误码 | MINOR |
| `Changed` | 既有行为/默认值/状态码变化 | MINOR 或 MAJOR |
| `Deprecated` | 即将移除，仍可用 | MINOR |
| `Removed` | 已移除 | MAJOR |
| `Fixed` | 缺陷修复 | PATCH（行为修复若影响契约则 MINOR） |
| `Security` | 安全修复 | PATCH（必要时单独公告） |
| `Docs` | 仅文档 | PATCH |

### 2.1 CHANGELOG 文件格式

建议在仓库根新增 `CHANGELOG.md`，按版本倒序：

```markdown
# Changelog

本文件记录 MGin 的所有重要变更。
格式基于 Keep a Changelog，版本号遵循 SemVer。

## [Unreleased]

### Added
- `pkg/xxx`：新增 ...

## [2.1.0] - 2026-09-04

### Added
- 客户端负载均衡：`pkg/loadbalancer`，注册名 `round_robin`/`random`/`least_connections`/`consistent_hash`。
- Prometheus 指标：`/metrics` + 6 个核心指标。

### Changed
- `NoRoute` 由 HTTP 200 改为 404；panic 恢复由 200 改为 500。
- **BREAKING**：对外包由仓库顶层迁移至 `pkg/`，`logs/` 保留兼容壳。

### Deprecated
- `logs/` 顶层别名壳，计划在 2.2.0 删除。

### Fixed
- HTTPS server 未纳入优雅关闭。
- `signalChan` 无缓冲导致退出信号丢失。

## [2.0.0] - 2026-08-xx
...
```

要求：

- `[Unreleased]` 持续累积；发布时改为版本号 + 日期（`YYYY-MM-DD`）。
- 每条变更说明**影响面**与**迁移动作**，不要只写"修复 bug"。
- 破坏性变更以 `**BREAKING**` 开头，并在版本发布说明中单列。
- 关联 issue/PR 编号（可选）：`- 修复 ... (#123)`。

### 2.2 提交信息到 CHANGELOG 的映射

| 提交前缀 | CHANGELOG 分类 |
|----------|----------------|
| `feat` | Added |
| `fix` | Fixed |
| `perf` | Changed |
| `refactor` | 通常不记（除非影响行为） |
| `docs` | Docs（重要文档变更才记） |
| `deprecate` | Deprecated |
| `remove` | Removed |
| `security` | Security |

---

## 3. 发布流程

### 3.1 角色

| 角色 | 职责 |
|------|------|
| 发布负责人 | 执行清单、打标签、发布说明 |
| 模块 Owner | 确认变更完整性与兼容性 |
| 评审人 | 复核 CHANGELOG 与迁移文档 |

### 3.2 标准发布（MINOR/PATCH）

```
1. 冻结变更
   └─ 从 v2 拉 release/vX.Y，只接受修复与版本相关提交

2. 质量门禁
   ├─ go build ./...
   ├─ 改动包 go vet（全仓库基线有 3 处历史 unreachable code）
   ├─ 本次改动文件 gofmt（历史有 14 个未格式化文件）
   ├─ 核心测试（L1+L2，见测试策略；pkg/logsink 失败用例需先修复）
   ├─ -race 关键包
   ├─ (cd cmd && go build ./... && go vet ./...)   // cmd 模块干净
   └─ 脚手架冒烟：mgin new + make build + 探针 curl

3. 版本与文档
   ├─ 更新 CHANGELOG（Unreleased → 版本号 + 日期）
   ├─ 更新 cmd/templates.go 的 toolVersion（若脚手架有变更）
   ├─ 更新 README 的版本/能力表
   ├─ 更新 docs/requirements.md 追踪矩阵
   └─ 破坏性变更补充迁移文档（参考 docs/migration-v1-to-v2.md）

4. 合入与打标签
   ├─ PR 合入 v2（squash）
   ├─ git tag -a vX.Y.Z -m "vX.Y.Z"
   ├─ git tag -a cmd/mgin/vX.Y.Z -m "cmd/mgin vX.Y.Z"（脚手架有变更时）
   └─ git push origin v2 --tags

5. 发布说明
   ├─ 标题：vX.Y.Z
   ├─ 摘要：1–3 条核心变化
   ├─ 破坏性变更与迁移步骤
   └─ 已知问题
```

### 3.3 发布前检查清单

- [ ] 无未合并的 release 分支改动
- [ ] `go build ./...` 通过；改动包 `go vet`、改动文件 `gofmt` 干净
      （仓库基线：3 处 vet 告警、14 个未格式化文件，不得新增）
- [ ] 核心测试通过；集成测试在发布环境通过
- [ ] 脚手架生成工程可编译、可启动
- [ ] CHANGELOG 完整，版本号与日期正确
- [ ] 破坏性变更已写迁移说明
- [ ] README 配置键/能力表与代码一致
- [ ] 未实现能力仍标注为 ❌
- [ ] 依赖许可证审查（新增依赖）
- [ ] 安全相关修复已评估是否需要单独公告

### 3.4 发布后

- 验证 `go get github.com/maczh/mgin/v2@vX.Y.Z` 可拉取。
- 验证 `go install github.com/maczh/mgin/v2/cmd/mgin@vX.Y.Z` 可用。
- 在示例工程执行一次升级 smoke。
- 观察 24 小时内的 issue；必要时发 PATCH。

---

## 4. 破坏性变更管理

### 4.1 判定

满足任一条即为破坏性：

- 删除/重命名导出符号，或修改签名。
- 接口新增方法（实现方会编译失败）。
- 配置键改名、默认值改变导致行为变化。
- HTTP 状态码/响应结构变化。
- import 路径变化。
- 依赖大版本升级导致传递类型变化。

### 4.2 处理流程

1. 在 `[Unreleased]` 的 `Changed`/`Removed` 中标注 `**BREAKING**`。
2. 提供迁移文档：旧写法 → 新写法 → 一键脚本（如 sed）。
3. 能提供兼容层就提供，并注明删除版本：
   `// Deprecated: 将于 v3.0.0 删除，请使用 pkg/xxx。`
4. MAJOR 发布说明必须包含"升级步骤"与"回滚方式"。

### 4.3 兼容壳生命周期

| 兼容项 | 引入版本 | 计划删除 | 规则 |
|--------|----------|----------|------|
| `logs/` 顶层别名壳 | v2.0.0 | 下一个 release | 删除前需 MAJOR 或在 MINOR 中提前一个版本公告 |
| v1 顶层 import 路径 | — | 已在 v2 移除 | 仅保留迁移文档 |

---

## 5. 热修复（Hotfix）

```
1. 从最近发布标签拉 hotfix/vX.Y.Z+1
2. 最小修复 + 回归测试
3. 走完整质量门禁（可缩短集成测试范围，但必须记录）
4. CHANGELOG 的 Fixed/Security 记录
5. 合入 v2 并打 PATCH 标签
6. 同步回 release 分支（如有）
```

约束：

- 热修复**不得**夹带重构或新功能。
- 安全修复先私下评估影响，发布说明避免给出可直接利用的细节。
- 若修复涉及行为变化，按 MINOR 而非 PATCH 发布。

---

## 6. 回滚策略

| 场景 | 回滚方式 |
|------|----------|
| 框架版本缺陷 | 业务 `go.mod` 回退到上一个 tag；框架侧发 PATCH |
| 配置不兼容 | 保留旧键读取一个 MINOR 周期；文档标注废弃 |
| 数据迁移失败 | 迁移前备份；回滚脚本随迁移文档提供 |
| 标签错误 | **不移动标签**；删除错误标签需维护者审批并公告，重新发新 PATCH |

发布说明中必须写"若出现问题，回退到 vX.Y.(Z-1)"。

---

## 7. 生成工程的版本规范

脚手架生成的 `version.go`：

```go
var (
    Version   = "dev"
    BuildNum  = ""
    BuildTime = ""
    GitHash   = ""
)
```

Makefile 注入：

```make
LDFLAGS=-ldflags "-X main.Version=${VERSION}-build${BUILD_TIME_SHORT} \
                  -X 'main.BuildTime=${BUILD_TIME}' \
                  -X main.GitHash=${GIT_HASH}"
```

约定：

- `Version` 取最近 git tag；无 tag 时为 `dev`。
- `-v` 输出格式由业务 `main` 决定（框架 `NewApp` 会打印 `appName, 版本号: version`）。
- 制品命名建议：`<app>_<version>_<os>_<arch>`，多平台产物放 `dist/`。
- 生成工程的 CHANGELOG 由业务自行维护，框架不代管。

---

## 8. 文档发布要求

每次发布必须同步：

| 文档 | 更新点 |
|------|--------|
| `README.md` | 版本、能力清单、配置键、FAQ、已知限制 |
| `docs/requirements.md` | FR 状态、验收标准、风险表 |
| `docs/architecture.md` | 架构变化、ADR、债务表 |
| `docs/modules-and-api.md` | 新增/变更 API 签名 |
| `docs/data-design.md` | 表结构/键空间变化 |
| `docs/testing-strategy.md` | 新增测试与门禁 |
| `docs/migration-*.md` | 破坏性变更迁移步骤 |
| `CHANGELOG.md` | 全部变更（建议新增此文件） |

---

## 9. 版本记录模板

```markdown
## [X.Y.Z] - YYYY-MM-DD

### 摘要
一句话说明本次发布的核心价值。

### Added
- ...

### Changed
- **BREAKING**：...（迁移：...）

### Deprecated
- ...

### Removed
- ...

### Fixed
- ...

### Security
- ...

### 升级指引
1. `go get github.com/maczh/mgin/v2@vX.Y.Z`
2. 按迁移文档替换 import / 配置键
3. 运行 `go build ./...` 与核心测试

### 已知问题
- ...
```
