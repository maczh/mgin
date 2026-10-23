# 版本发布与变更记录规范（jh 分支）

| 项 | 内容 |
|----|------|
| 当前基线 | `jh` 分支，最新 tag `v1.25.27-jh`（HEAD 领先 2 个提交） |
| 相关 | [需求文档](../02-concepts/04-requirements.md) · [测试策略](03-testing-strategy.md) · [编码规范](02-coding-standards.md) |

---

## 1. 版本号规范

采用语义化版本 + 分支后缀：

```
v<major>.<minor>.<patch>-jh
```

| 段 | 递增条件（jh 线） |
|----|-------------------|
| `major` | 不兼容变更（删除/重命名导出 API、改变返回协议、删除配置键） |
| `minor` | 向后兼容的新能力（新增数据源、中间件、配置项、脚手架选项） |
| `patch` | 向后兼容的缺陷修复、依赖安全升级、文档/日志修正 |
| `-jh` | 定制线标识，不可省略；`master` 线使用 `v2.x.y` 且模块路径含 `/v2` |

规则：

- **Go module 兼容性**：`jh` 的模块路径是 `github.com/maczh/mgin`，只能发布 `v0/v1` tag；`v2` 必须配 `github.com/maczh/mgin/v2` 模块，两者不可混用。
- 脚手架默认依赖最新发布版；离线回退 `v1.25.13-jh`（见 `cmd/new.go`）。发布后需确认 `mgin new` 拉到的版本正确。
- 同一提交只能有一个版本 tag；禁止移动已推送的 tag。

---

## 2. 分支与发布模型

| 分支 | 角色 | 发布 |
|------|------|------|
| `jh` | 定制主线，保持可发布 | 打 `v1.25.x-jh` |
| `feat/*`、`fix/*`、`docs/*` | 短期分支 | 不发布，PR 合回 `jh` |
| `hotfix/*` | 线上紧急修复 | 从 tag 拉出，修复后合回 `jh` 并打 patch 版本 |
| `master` | 上游通用线（`/v2`） | 独立发布 `v2.x.y` |

发布冻结：打 tag 前 24 小时只接受修复类提交；冻结期间在分支上打 `release/<version>` 标记（或使用 PR 说明）并完成验证清单。

---

## 3. 发布流程

```mermaid
flowchart LR
    A[变更合并到 jh] --> B[冻结 + 验证清单]
    B --> C[更新 CHANGELOG]
    C --> D[打 tag vX.Y.Z-jh]
    D --> E[推送 tag]
    E --> F[验证 go get / 脚手架]
    F --> G[通知使用方]
```

### 3.1 步骤

1. **确认范围**：`git log --oneline <上次tag>..HEAD`，区分 feat/fix/docs，确认没有未完成的破坏性变更。
2. **验证**（必须全部通过）：
   ```bash
   go build ./...
   (cd cmd && go build ./...)
   gofmt -l $(git ls-files '*.go' | grep -v '^.Trash-0/')
   go test ./health/ ./metrics/ ./logs/ ./logsink/ ./utils/ ./middleware/cors/ ./middleware/limit/ ./middleware/ratelimit/
   ```
   外部依赖相关包按 [测试策略 §6](03-testing-strategy.md#6-发布前验证清单) 在目标环境验证。
3. **更新变更记录**：在 `CHANGELOG.md` 增加本次版本条目（当前仓库尚无该文件，首次发布时创建），格式见 §4。
4. **打 tag**：
   ```bash
   git tag -a v1.25.28-jh -m "v1.25.28-jh: <一句话摘要>"
   git push origin v1.25.28-jh
   ```
5. **验证产物**：
   ```bash
   GOFLAGS=-mod=mod go get github.com/maczh/mgin@v1.25.28-jh
   ./cmd/bin/mgin new smoke --module example.com/smoke --db mysql --output /tmp
   (cd /tmp/smoke && go mod tidy && go build ./...)
   ```
6. **通知**：在使用方群/发布记录中说明版本、变更点、升级注意事项与回滚方式。

### 3.2 禁止事项

- 不验证就发版；用「测试环境跑过」代替版本验证。
- 同一版本重复打 tag 或强推覆盖 tag。
- 把 `jh` 与 `master` 的模块路径混用（`/v2` 依赖 `v1.25.x-jh` 会直接失败）。

---

## 4. CHANGELOG 规范

采用 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 风格，文件名 `CHANGELOG.md`，最新版本在最上方。

```markdown
## [v1.25.28-jh] - 2026-11-02

### 新增
- ratelimit 支持 `dimension: header`，按请求头限流 (#123)

### 修复
- logsink 队列参数与文档不一致，统一上限为 1000/配置文件 100 (#124)
- iplimit 使用非恒定格式串导致 go vet 失败 (#125)

### 变更
- `client.CallT` 返回 `vo.Response`，旧 `CallResultT` 保留但标记废弃

### 安全
- 升级 `golang.org/x/crypto` 至 0.40.0

### 升级说明
- 无需改配置；使用 `client.CallResultT` 的服务建议迁移到 `CallT`
```

要求：

- 每条变更写「行为变化 + 影响面」，不写「修复 bug」这类无信息描述。
- 破坏性变更单独加 `### 破坏性变更`，并在标题后标 `BREAKING`。
- 关联 issue/PR 编号；安全修复在披露前不要写可利用细节。
- 文档-only 变更可合并为一条 `文档`。

---

## 5. 兼容性策略

| 对象 | 兼容要求 |
|------|----------|
| 导出 API | patch/minor 不删除、不改签名；新增用可选参数或新函数 |
| 配置键 | 不删除、不改默认值；新增键默认关闭且不改变旧行为 |
| 返回协议 | `models.Result` 字段只增不改名；`vo.Response` 单独版本化 |
| 数据表 | 只做加法（新增列/索引）；删除列必须走 major |
| 日志格式 | 允许新增字段；删除/重命名字段视为破坏性 |
| 脚手架生成物 | 生成工程必须可编译；选项语义变化写入 CHANGELOG |

废弃流程：

1. 在代码注释与文档标注 `Deprecated: use X`；
2. 保留至少一个 minor 周期；
3. CHANGELOG 的 `### 废弃` 中列出；
4. 下个 major 删除，并在升级说明中给出替换方案。

---

## 6. 回滚

**代码回滚**

```bash
git revert <bad-commit>          # 生成反向提交，保留历史
# 或从上一个 tag 拉 hotfix 分支修复后重新发 patch
```

禁止对 `jh` 强推；紧急情况下用 hotfix + 新 patch tag。

**版本回退（使用方）**

```bash
go get github.com/maczh/mgin@v1.25.27-jh
```

**配置/数据回滚**

- 配置：保留上一版 `application.yml` 与子配置；`/cfg/reload` 只重载资源，回滚后调用一次即可。
- 表结构：发布前备份；只加不删的变更通常无需回滚；涉及数据迁移必须提供反向脚本。
- 缓存：必要时清空相关前缀（避免使用 `FLUSHDB`）。

回滚后必须在 CHANGELOG 记录：回滚原因、影响版本、修复版本。

---

## 7. 依赖与安全更新

1. 每月检查一次依赖漏洞与停止维护的库（优先 `dgrijalva/jwt-go`、`sirupsen/logrus` 等）。
2. 升级依赖单独提交，附 `go mod tidy` 与构建/测试结果。
3. 安全修复可走 patch 版本并在 CHANGELOG 的 `### 安全` 标注；不要在同一提交混入功能变更。
4. 升级 Go 版本（如 1.25.7 → 1.26）属于 minor 变更，需同步更新 README、开发环境文档与 CI 镜像。

---

## 8. 发布检查清单

- [ ] 变更范围已确认，无未完成的破坏性改动
- [ ] 构建通过（根模块 + `cmd` 模块）
- [ ] 格式检查通过（排除 `.Trash-0/`）
- [ ] 可独立运行的测试通过；外部依赖包已在目标环境验证
- [ ] `CHANGELOG.md` 已更新（含升级说明）
- [ ] README/docs 中受影响的章节已同步（能力边界、配置表、示例）
- [ ] tag 名称符合 `vX.Y.Z-jh`，注释包含摘要
- [ ] `go get` 与脚手架冒烟通过
- [ ] 已通知使用方，并给出回滚版本号
