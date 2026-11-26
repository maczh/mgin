# 09 代码审查·规范轴

规范源：`docs/coding-standards.md`（成文规范优先于 smell baseline）。`gofmt -l` 改动文件无输出。

## (a) 成文规范违反（硬）

1. §1.2 导出符号须有首词符号名注释：`mgo.go:38 ErrClosed`、`:139 InitWithError`；`redis.go:37 ErrClosed`、`:198 InitWithError`；`sqlite.go:39 InitWithError` 均无注释。
2. §3.1.1 禁止无说明的 `_ = f()`：`sqlite.go:36 _ = m.InitWithError(...)` 未注明"为兼容旧签名而忽略"。
3. §3.1.3 英文错误小写开头：新增 `"Mongodb is nil"`、`"Redis is nil"`、`"Redis address is empty"`、`"SQLite configuration is unavailable"` 均大写开头（与 legacy 风格冲突，格式级）。

## (b) Baseline smell（判断性，非硬违规）

- Duplicated Code：`plugin.go` 五个 `safeXxx` 同形 defer/recover、`firstErr==nil` 赋值重复约 6 处；`mgo.go`/`redis.go` 的 `closed→ErrClosed` 与 needsInit/confData 拷贝模板重复 4 处；`redis_cache.go` 8 处 nil 守卫。
- Middle Man（边缘）：`sqlite.Init` 仅转调 `InitWithError`，为兼容旧签名可接受。
- 其余 baseline 项未见显著气味。

## 合规/抑制

§5 Ping 移出锁、ticker.Stop+done+WaitGroup 齐备；§4.1 日志均 `{}`；删不可达 return 清理 vet 告警——符合规范，抑制对应 smell。
