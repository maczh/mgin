# 接口与持久化模式

## 保留项目分层与协议

订单项目按 `admin/app/mp` 分端，桌台服务为平铺结构，支付请求包名是 `model/request`。沿目标结构扩展，不强制统一包路径。

典型链路：

```go
// router: 绑定 handler，HTTP 状态遵循项目契约
prefix.POST("/create", func(ctx *gin.Context) {
    result := controller.Item.Create(ctx)
    ctx.JSON(http.StatusOK, result)
})

// controller: 示意，实际类型和 service 单例来自目标项目
func (c *itemController) Create(ctx *gin.Context) vo.Response[any] {
    var req ro.CreateReq
    if err := ctx.ShouldBindJSON(&req); err != nil {
        return vo.Error(400, "请求参数错误")
    }
    return service.Item.Create(&req)
}
```

GET 对应 `ShouldBindQuery` / `Query`，请求结构使用 `form` tag；JSON 使用 `json` tag，必要时加 `binding` 校验。需透传的 token 从可信头/鉴权上下文获取，内部字段使用 `json:"-"`，不接受请求体伪造。沿项目中间件保证路由鉴权，新接口不能照搬另一个服务的 `/admin/` 放行规则。

方法返回类型经常是 `vo.Response[any]` 或 `vo.ResponsePage[any]`，相关单例由 `controller/enter.go`、`service/enter.go` 汇总。service 不持有 Gin Context；内部 helper 可以返回领域值及 error，最终出口再封装响应。

协议差异：

| 封装 | 成功字段 | 典型位置 |
|---|---|---|
| 框架 `models.Result[T]` | `status: 1` | mgin `models/result.go` |
| 业务 `vo.Response[T]` | `code: 1` | store-table `vo/result.go`、订单/支付 `model/vo`、common `vo` |

业务分页常用 `vo.SuccessPage(list, resultPage)`，框架分页为 `models.SuccessPage(data, page)`，外层 JSON 结构不同。泛型不协变：`Response[Entity]` 不是 `Response[any]`；根据目标封装使用 `.ToAny()` 或 `Success[any](entity)`，不要为编译通过随意断言。

## MySQL

已核实的 v1 声明与约束：

```go
var ItemDao = dao.MySQLDao[model.Item]{}
// model.Item 需实现 TableName() string（schema.Tabler）
```

`mysql/enter.go` 常用 `db.Mysql.GetConnection()` 后 `AutoMigrate(tables...)`，新增实体要补进 tables。运行入口会执行迁移，因此编译检查不要启动 main。

常见真实签名：

- `One(entity E) (*E, error)`：未命中是 `nil, nil`，分别处理连接失败与业务不存在。
- `Create(entity *E) error`、`Save(entity *E) error`、`Updates(entity *E) error`、`Delete(entity E) error`。
- `Where(query interface{}, args ...interface{}) *gorm.DB`：返回的是 GORM 查询链，不是另一个 DAO；连接失败可能返回 nil，继续 `.Where/.Order` 前检查。
- `Pager(conn *gorm.DB, page, size int) ([]E, *models.ResultPage, error)`。

分页参照 `jh-pay/service/order.go`：参数化过滤，校验页码和大小，指定稳定排序，再调用 DAO.Pager；不要把字符串排序直接接受为未校验用户输入。

特别注意：

- GORM struct 条件可能忽略零值。筛选 `0/false/空字符串` 时使用显式条件或 map；可选过滤参数用指针区分未传与零值。
- struct Updates 也可能略过零值；重置状态需明确更新字段并验证实际行为。Save 可能写回整行，不作为所有局部更新的替代。
- 删除条件必须非空且包含正确主键/业务隔离条件；`Delete(entity)` 在本地是 `Where(entity).Delete(&e)`，不是按任意唯一字段自动安全删除。
- 事务不能混用独立取连接的 DAO 写入；按项目可用封装或事务连接执行全部相关操作，并检查回滚。
- 本地 DAO `WithContext(*context.Context)` 会修改接收者；不要把每请求 context 并发写入共享 DAO 单例。缓存配置也不能在请求间任意改写；GORM 链写入可能绕开 DAO 的缓存失效逻辑。
- 涉及 storeId、orderId 等隔离条件时沿业务约束过滤；不要复制只按 ID 查询的例子后丢失访问范围。

## MongoDB 与共享模型

```go
var ItemDao = dao.MgoDao[model.Item]{
    CollectionName: model.Item{}.CollectionName(),
}
```

`CollectionName` 必须先赋值。`One(bson.M)` 未命中也是 `nil, nil`；`Pager(query, sort, page, size)` 与 MySQL 签名不同。手工取连接的旧实现必须对应 `ReturnConnection`，优先延续目标已有 DAO。

RIS 服务的公共实体、消息信封和 VO 常在 `jh-ris-common`，并通过本地 replace 引用。先查公共模型，避免复制出两个不兼容定义。金额单位、ID 类型、JSON/BSON/GORM tag 以实际模型为准；状态逻辑复用领域常量及谓词。

## 文档与验证

controller 的 Swagger 注释包含 Summary、Tags、参数、响应和 Router；Router 必须与最终前缀拼接后的路由一致。响应声明反映项目真实封装，避免只抄类型名。文档生成命令先查 Makefile、go:generate 或现有 CI，不手改生成 docs。

对数据变化验证未命中、零值过滤/更新、越权范围、分页边界；对订单/支付变化验证状态允许条件、金额单位、重复请求与失败回滚。复用项目测试方式，不为简单文档或可逆小修改强加测试套件。

