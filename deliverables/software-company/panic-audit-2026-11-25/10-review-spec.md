# 规格轴审查（git diff aaae6a3...HEAD，9 提交；基线 merge-base=aaae6a3）

## (a) 规格要求但 diff 缺失/部分实现

- **X1 Init 锁内 I/O**：07 计划 §3 批一「Init 锁外解析配置和 Dial/Ping，锁内一次性发布」。实际 `mgo.go:44-47,103,124`、`redis.go:44-46,185` 仍在 `initMu→mu` 双锁内逐库建连/Ping；RCU 范式完全未抄。
- **X2 关旧连接**：07§3「摘出连接快照后锁外关闭 session」。实际 `mgo.go:153-169`、`redis.go:212-226` 仍在写锁内 `Close()`。三态 `uninitialized/ready/closed` 退化为单个 `closed bool`。
- **X7 订阅**：07§3「Close 取消内部订阅 context；握手 Receive 响应取消；所有退出路径关闭 pubsub」。实际 `redis.go:363-364` 仍 `context.Background()`，`Close:208-231` 无 cancel，`:405 Receive()` 无超时，`:432` 提前 return 漏关 pubsub。
- **X5b 两端守护** ✓（`plugin.go:40-48,121-134,240-268`）；但 06 报告 §4#19 `mgin.go:124-125` VictoriaLogs `panic(err)` 未改。
- **X8**（`redis.go:401-403` 原地改 channel 模板）、**X11 注释残骸**（`:233-246`）未删（06§5 批四）。
- **initErr 漏报**：`mgo.go:103-107` multi 模式 Dial 失败仅 `continue`，`InitWithError` 返 nil → 假成功。
- **07§4 回归清单仅落地 4/16**：缺 X3/X4/X10 零值 gormDB/X5 三入口/X6/X13/App 顺序等用例。

## (b) 范围外行为（scope creep）

- `4f1d9de` 改 `pkg/middleware/xss/xss.go`、`pkg/utils/rsautil.go`，与 panic 审计无关。
- `29e2a28`（关停顺序）混入 §5 批四的 `WithTx` nil 守护、`RedisCache` nil receiver/Range(nil)、`validateSqlite`，批次边界混乱。

## (c) 看似实现、实际错误

- **硬约束违反**：`5c17214` 是**纯 test 提交**（4 文件 +73 行，0 生产代码），违反 04§6/§10.11「X 系列回归用例不得单独入仓、必须与修复同 PR」。且在 `aaae6a3` 上 `git grep ErrClosed` 证实 redis/mongo 无该符号 → 新增用例是**编译失败**而非规范的干净红；`plugin_panic_test.go:23` 的 `Register` panic 未 recover（违反 04§10.10「recover 只能转 t.Fatalf」），会杀掉整包。
- **X3 以删除代迁移**：`mgo.go:256-294` 彻底不再 Ping/mgoCheck，`:175-199` `mgoCheck` 成死代码，连接失效后失去自愈（规格要求「移出锁 + 1s 节流而非取消校验」）；`Copy()/DB()` 仍在 RLock 内（`:276/:289`），未做到「快照后立即解锁」。
- **关停顺序**：`app.go:391 checkWG.Wait()` 先于 `Shutdown(:399)` 且无超时，与 07§3「Shutdown → 等待健康检查 → SafeExit」顺序相反，健康检查卡住时反而阻塞 HTTP 排空。
- **9 提交未按 06§5 四批**：`6ba617c/29e2a28` 混批、`10fe036` 为 07 新增项、`4f1d9de` 越界；仅 X4/X9-1/X12/X14/RedisCache/X10 的修复是干净达标项。
