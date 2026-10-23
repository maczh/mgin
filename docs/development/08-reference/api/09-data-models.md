# 响应模型与系统模型

> 本文件由脚本从源码 AST 生成，签名与字段与当前代码一致；新增导出符号后请重新生成。
>
> 统一响应结构、分页、系统管理表模型与请求 DTO。

## `github.com/maczh/mgin/v2/pkg/models`

```go
import "github.com/maczh/mgin/v2/pkg/models"
```

### 类型

#### `PostLog`

```go
type PostLog struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Time` | string | json:"time" bson:"time" | ID bson.ObjectId `bson:"_id"` |
| `RequestId` | string | json:"requestId" bson:"requestId" |  |
| `ResponseTime` | string | json:"responseTime" bson:"responseTime" |  |
| `TTL` | int | json:"ttl" bson:"ttl" |  |
| `AppName` | string | json:"appName" bson:"appName" |  |
| `Apiname` | string | json:"apiName" bson:"apiName" |  |
| `Method` | string | json:"method" bson:"method" |  |
| `ContentType` | string | json:"contentType" bson:"contentType" |  |
| `Uri` | string | json:"uri" bson:"uri" |  |
| `ClientIP` | string | json:"clientIP" bson:"clientIP" |  |
| `RequestHeader` | map[string]string | json:"requestHeader" bson:"requestHeader" |  |
| `RequestParam` | any | json:"requestParam" bson:"requestParam" |  |
| `RequestBody` | any | json:"requestBody" bson:"requestBody" |  |
| `ResponseStr` | string | json:"responseStr" bson:"responseStr" |  |
| `ResponseMap` | any | json:"responseMap" bson:"responseMap" |  |

#### `Result`

```go
type Result[T any] struct
```

*
通用返回结果类

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Status` | int | json:"status" bson:"status" |  |
| `Msg` | string | json:"msg" bson:"msg" |  |
| `Data` | T | json:"data,omitempty" bson:"data,omitempty" |  |
| `Page` | *ResultPage | json:"page,omitempty" bson:"page,omitempty" |  |

**方法**

```go
func (r Result[T]) ToAny() Result[any]
```

从泛型转any

#### `ResultPage`

```go
type ResultPage struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Count` | int | json:"count" | 总页数 |
| `Index` | int | json:"index" | 页号 |
| `Size` | int | json:"size" | 分页大小 |
| `Total` | int | json:"total" | 总记录数 |

#### `Timestamp`

```go
type Timestamp time.Time
```

**方法**

```go
func (t Timestamp) MarshalJSON() ([]byte, error)
```

MarshalJSON implements json.Marshaler.

```go
func (t *Timestamp) UnmarshalJSON(data []byte) (err error)
```

```go
func (t Timestamp) Value() (driver.Value, error)
```

```go
func (t *Timestamp) Scan(v any) error
```

```go
func (t Timestamp) Time() time.Time
```

### 函数

#### `Error`

```go
func Error(s int, m string) Result[any]
```

#### `ErrorT`

```go
func ErrorT[T any](s int, m string) Result[T]
```

#### `NewTimestamp`

```go
func NewTimestamp(t time.Time) Timestamp
```

#### `Success`

```go
func Success[T any](data T) Result[T]
```

#### `SuccessPage`

```go
func SuccessPage[T any](data T, page *ResultPage) Result[T]
```

#### `SuccessWithMsg`

```go
func SuccessWithMsg[T any](msg string, data T) Result[T]
```

#### `SuccessWithPage`

```go
func SuccessWithPage[T any](data T, count, index, size, total int) Result[T]
```

#### `ToAny`

```go
func ToAny[T any](result Result[any]) Result[T]
```

从any转指定泛型，必须明确Data的类型一致，否则断言可能panic

## `github.com/maczh/mgin/v2/pkg/models/sys`

```go
import "github.com/maczh/mgin/v2/pkg/models/sys"
```

### 类型

#### `BaseModel`

```go
type BaseModel struct
```

BaseModel 共享属性

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `DelFlag` | uint | gorm:"type:tinyint;default:0;index;comment:删除标记;column:del_flag;" json:"delFlag" | 删除标记 |
| `CreateAt` | time.Time | gorm:"type:datetime;autoCreateTime;comment:创建日期" json:"createAt" | 创建日期 |
| `UpdateAt` | time.Time | gorm:"type:datetime;autoUpdateTime;comment:更新日期" json:"updateAt" | 更新日期 |
| `UpdateBy` | string | gorm:"type:string;size:32;comment:更新者;" json:"updateBy" | 更新者 |
| `CreateBy` | string | gorm:"type:string;size:32;comment:创建者;" json:"createBy" | 创建者 |
| `TenantId` | int64 | gorm:"type:string;size:32;comment:租户id;" json:"tenantId" form:"tenantId" | 租户id |

#### `SysApi`

```go
type SysApi struct
```

SysApi 系统接口表
记录后端所有API接口的权限信息

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | uint | gorm:"primaryKey;comment:接口ID" json:"id" | 主键ID |
| `Name` | string | gorm:"type:varchar(100);unique;not null;comment:API接口名称" json:"name" | API接口名称 |
| `APIPath` | string | gorm:"type:varchar(255);not null;uniqueIndex:api_unique;comment:接口路径(支持通配符)" json:"api_path" | API路径(支持通配符) |
| `Method` | string | gorm:"type:varchar(10);not null;default:'GET';uniqueIndex:api_unique;comment:请求方法(GET\|POST\|PUT\|DELETE...)" json:"method" | 请求方法(GET\|POST\|PUT\|DELETE...) |
| `Description` | string | gorm:"type:varchar(255);comment:接口描述" json:"description" | 接口描述 |
| `APIGroup` | string | gorm:"type:varchar(50);index;comment:接口分组(模块名称)" json:"api_group" | 接口分组(模块名称) |
| `Enabled` | uint8 | gorm:"type:tinyint;default:1;index;comment:启用状态(1启用 0禁用)" json:"enabled" | 启用状态(1启用 0禁用) |
| `NeedAuth` | uint8 | gorm:"type:tinyint;comment:是否需要认证(1是 0否);column:need_auth" json:"need_auth" | 是否需要JWT认证(1是 0否) |
| `NeedLog` | uint8 | gorm:"type:tinyint;default:1;comment:是否需要记录日志" json:"need_log" | 是否需要记录日志 |
| `BaseModel` | BaseModel |  |  |

**方法**

```go
func (r SysApi) TableName() string
```

#### `SysConfig`

```go
type SysConfig struct
```

SysConfig 系统配置表,相当于动态INI配置文件
该表用于存储系统的配置信息，包括模块、名称、键名、值和描述等字段。
可以根据需要添加更多字段，例如配置类型、配置范围等。
该表可以通过模块和键名进行查询和更新操作，方便管理系统的配置信息。
注意：该表中的配置项名称是唯一的，不允许重复。

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | int64 | gorm:"primaryKey;autoIncrement;comment:主键ID" json:"id" | 主键ID |
| `Module` | string | gorm:"type:varchar(100);comment:所属功能模块;index" json:"module" binding:"required" | 所属功能模块 |
| `Name` | string | gorm:"type:varchar(50);not null;index;comment:配置名" json:"name" binding:"required" | 配置名 |
| `Key` | string | gorm:"type:varchar(100);uniqueIndex;not null;comment:配置项名称，即配置变量名，唯一" json:"key" binding:"required" | 配置项名称，即配置变量名，唯一 |
| `Value` | string | gorm:"type:text;comment:该配置保存值" json:"value" binding:"required" | 该配置保存值 |
| `Description` | string | gorm:"type:varchar(255);comment:配置项描述" json:"description" | 配置项描述 |

**方法**

```go
func (r SysConfig) TableName() string
```

#### `SysDept`

```go
type SysDept struct
```

SysDept 系统部门表

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | uint | gorm:"type:uint;primary_key;auto_increment;部门id;" json:"id" | 主键ID |
| `ParentId` | uint | gorm:"type:uint;index;comment:父部门id;" json:"parentId" | 父部门id |
| `Ancestors` | string | gorm:"type:varchar(100);comment:祖级列表;index:idx_ancestors" json:"ancestors" | 祖级列表，以逗号分割 |
| `Name` | string | gorm:"type:varchar(30);unique;not null;comment:部门名称;" json:"name" | 部门名称 |
| `Sort` | int | gorm:"type:int(10);comment:显示顺序;" json:"sort" | 显示顺序 |
| `Leader` | string | gorm:"type:varchar(20);comment:负责人;" json:"leader" | 负责人 |
| `Mobile` | string | gorm:"type:varchar(11);comment:联系电话;" json:"mobile" | 联系电话 |
| `DeptType` | string | gorm:"type:varchar(50);index;default:0;comment:组织类型;" json:"deptType" | 组织类型(0:公司 1:部门) |
| `Status` | uint8 | gorm:"type:tinyint;default:1;comment:部门状态（1正常 0停用）;" json:"status" | 部门状态(1:正常 0:停用) |
| `ParentName` | string | gorm:"-" json:"parentName" | 父部门名称 |
| `BaseModel` | BaseModel |  |  |
| `Children` | []*SysDept | gorm:"-" json:"children" | 子部门列表 |

**方法**

```go
func (e SysDept) TableName() string
```

#### `SysDict`

```go
type SysDict struct
```

SysDict 字典数据

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | int | json:"id" gorm:"primaryKey;autoIncrement;type:int;comment:主键ID" | 主键ID |
| `Type` | string | json:"type" gorm:"type:varchar(50);not null;index:idx_type;comment:字典类型" | 字典类型 |
| `ParentId` | int | json:"parent_id" gorm:"type:int;default:0;comment:父级ID" | 父级ID |
| `Name` | string | json:"name" gorm:"type:varchar(100);not null;index;comment:字典中文名称" | 字典中文名称 |
| `Key` | string | json:"key" gorm:"type:varchar(50);not null;index:idx_key;comment:字典键名" | 字典键名 |
| `Value` | string | json:"value" gorm:"type:varchar(100);not null;comment:字典值" | 字典值 |
| `Sort` | int | json:"sort" gorm:"type:int;default:0;comment:排序" | 排序 |
| `Remark` | string | json:"remark" gorm:"type:varchar(255);comment:备注" | 备注 |
| `BaseModel` | BaseModel |  |  |

**方法**

```go
func (e SysDict) TableName() string
```

#### `SysUserOnline`

```go
type SysUserOnline struct
```

SysUserOnline 当前在线会话

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | int64 | gorm:"column:id;type:bigint(20);primary_key;auto_increment;ID;" json:"id" form:"id" | 主键ID |
| `TokenID` | string | gorm:"column:token_id;type:varchar(255);primaryKey;comment:会话编号" json:"tokenId" | 会话编号, md5(jwtToken) |
| `UserID` | uint64 | gorm:"column:user_id;type:bigint unsigned;index;comment:用户ID" json:"userId" | 用户ID |
| `UserName` | string | gorm:"column:user_name;type:varchar(255);index;comment:用户名称" json:"userName" | 用户名称 |
| `ClientKey` | string | gorm:"column:client_key;type:varchar(255);index;comment:客户端标识" json:"clientKey" | 客户端标识 |
| `DeviceType` | string | gorm:"column:device_type;type:varchar(50);comment:设备类型" json:"deviceType" | 设备类型 |
| `IpAddr` | string | gorm:"column:ip_addr;type:varchar(50);comment:登录IP地址" json:"ipAddr" | 登录IP地址 |
| `LoginLocation` | string | gorm:"column:login_location;type:varchar(255);comment:登录地址" json:"loginLocation" | 登录地址 |
| `Browser` | string | gorm:"column:browser;type:varchar(50);comment:浏览器类型" json:"browser" | 浏览器类型 |
| `Os` | string | gorm:"column:os;type:varchar(50);comment:操作系统" json:"os" | 操作系统 |
| `LoginTime` | int64 | gorm:"column:login_time;type:bigint;comment:登录时间" json:"loginTime" | 登录时间 |

**方法**

```go
func (r SysUserOnline) TableName() string
```

TableName 指定表名

#### `SysPost`

```go
type SysPost struct
```

SysPost 岗位信息

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | int64 | gorm:"type:bigint(20);primary_key;auto_increment;岗位ID;"     json:"id"  form:"id" | 主键ID |
| `PostCode` | string | gorm:"type:varchar(64);uniqueIndex;comment:岗位编码;" json:"post_code" form:"post_code" | 岗位编码(全局唯一) |
| `PostName` | string | gorm:"type:varchar(50);uniqueIndex;comment:岗位名称;" json:"post_name" form:"post_name" | 岗位名称(全局唯一) |
| `DeptId` | int64 | gorm:"type:bigint(20);index;comment:所属部门ID;" json:"dept_id" form:"dept_id" | 所属部门ID |
| `Sort` | int | gorm:"type:int(11);comment:显示顺序;" json:"sort" form:"sort" | 显示顺序 |
| `Status` | uint8 | gorm:"type:tinyint;comment:状态（1正常 0停用）;" json:"status" form:"status" | 状态(1:正常 0:停用) |
| `Remark` | string | gorm:"type:varchar(500);comment:备注;" json:"remark" form:"remark" | 备注 |
| `BaseModel` | BaseModel |  |  |

**方法**

```go
func (e SysPost) TableName() string
```

#### `SysResource`

```go
type SysResource struct
```

SysResource 系统前端资源表
用于存储前端路由和后端权限的菜单结构

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | uint | gorm:"primaryKey;comment:菜单ID" json:"id" | 菜单ID |
| `Type` | int | gorm:"type:tinyint;default:1;comment:资源类型(1菜单 2按钮)" json:"type" | 资源类型(1菜单 2按钮) |
| `MenuLevel` | int | gorm:"type:tinyint;default:1;comment:菜单等级(1主菜单 2子菜单)" json:"menu_level" | 菜单等级(1主菜单 2子菜单) |
| `ParentId` | uint | gorm:"index;comment:父菜单ID(0表示根菜单)" json:"parent_id" | 父菜单ID(0表示根菜单) |
| `Path` | string | gorm:"type:varchar(255);index;comment:路由路径(需符合Vue Router规范)" json:"path" | 路由路径(需符合Vue Router规范) |
| `Name` | string | gorm:"type:varchar(50);index;comment:路由名称" json:"name" | 路由名称 |
| `Component` | string | gorm:"type:varchar(255);comment:组件路径(views目录下的路径)" json:"component" | 组件路径(views目录下的路径) |
| `Redirect` | string | gorm:"type:varchar(255);comment:重定向路径" json:"redirect" | 重定向路径 |
| `Icon` | string | gorm:"type:varchar(50);comment:菜单图标(组件名称)" json:"icon" | 菜单图标(组件名称) |
| `Title` | string | gorm:"type:varchar(50);not null;comment:菜单标题(显示名称)" json:"title" | 菜单标题(显示名称) |
| `Sort` | int | gorm:"type:int;default:0;index;comment:排序序号(越小越靠前)" json:"sort" | 排序序号(越小越靠前) |
| `Hidden` | uint8 | gorm:"type:tinyint;default:0;comment:是否隐藏(0显示 1隐藏)" json:"hidden" | 是否隐藏(0显示 1隐藏) |
| `KeepAlive` | uint8 | gorm:"type:tinyint;default:1;comment:是否缓存(0否 1是)" json:"keep_alive" | 是否缓存(0否 1是) |
| `AlwaysShow` | uint8 | gorm:"type:tinyint;default:0;comment:是否总是显示(即使只有一个子菜单)" json:"always_show" | 是否总是显示(即使只有一个子菜单) |
| `Breadcrumb` | uint8 | gorm:"type:tinyint;default:1;comment:是否显示面包屑" json:"breadcrumb" | 是否显示面包屑 |
| `Affix` | uint8 | gorm:"type:tinyint;default:0;comment:是否固定到标签页" json:"affix" | 是否固定到标签页 |
| `ActiveResource` | string | gorm:"type:varchar(255);comment:激活菜单的路径" json:"active_resource" | 激活菜单的路径 |
| `Status` | int | gorm:"type:tinyint;default:1;index;comment:菜单状态(1正常 0停用)" json:"status" | 菜单状态(1正常 0停用) |
| `ApiId` | *uint | gorm:"type:uint;index;comment:关联的API接口ID" json:"api_id" | 点击资源调用的后端API接口ID |
| `BaseModel` | BaseModel |  |  |
| `Children` | []SysResource | gorm:"-;comment:子菜单列表" json:"children" | 关联关系 |
| `Roles` | []SysRole | gorm:"-" json:"roles" | 多对多关联 |
| `Parent` | *SysResource | gorm:"-" json:"parent" | 自关联 |

**方法**

```go
func (r SysResource) TableName() string
```

#### `SysRole`

```go
type SysRole struct
```

SysRole 角色表结构

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | uint | gorm:"primaryKey;comment:角色ID" | 角色ID |
| `RoleName` | string | gorm:"type:varchar(50);not null;uniqueIndex;comment:角色名称" | 角色名称 |
| `RoleIdent` | string | gorm:"type:varchar(50);not null;uniqueIndex;comment:角色标识符(英文)" | 角色标识符(英文) |
| `IsEnable` | uint8 | gorm:"type:tinyint;default:1;index;comment:启用状态(1启用 0禁用)" | 启用状态(1启用 0禁用) |
| `Description` | string | gorm:"type:varchar(255);comment:角色描述" | 角色描述 |
| `BaseModel` | BaseModel |  |  |

**方法**

```go
func (r SysRole) TableName() string
```

#### `SysRoleApi`

```go
type SysRoleApi struct
```

SysRoleApi 角色API权限信息模型

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | uint | gorm:"primaryKey;type:uint;comment:权限ID" json:"id" | 权限ID |
| `RoleId` | uint | gorm:"type:uint;not null;index;comment:角色ID" json:"role_id" | 角色ID |
| `ApiId` | uint | gorm:"type:uint;not null;comment:API接口ID" json:"api_id" | API接口ID |
| `Role` | *SysRole | gorm:"-" json:"role" | 关联的角色信息 |
| `Api` | *SysApi | gorm:"-" json:"api" | 关联的API接口信息 |
| `BaseModel` | BaseModel |  |  |

**方法**

```go
func (r SysRoleApi) TableName() string
```

#### `SysRoleResource`

```go
type SysRoleResource struct
```

SysRoleResource 角色资源权限信息模型

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | uint | gorm:"primaryKey;type:uint;comment:权限ID" json:"id" | 权限ID，主键 |
| `RoleId` | uint | gorm:"type:uint;not null;index;comment:角色ID" json:"role_id" | 角色ID，不能为空 |
| `ResourceId` | uint | gorm:"type:uint;not null;comment:资源ID" json:"resource_id" | 资源ID，不能为空 |
| `Role` | *SysRole | gorm:"-" json:"role" | 关联的角色信息 |
| `Resource` | *SysResource | gorm:"-" json:"resource" | 关联的资源信息 |
| `BaseModel` | BaseModel |  |  |

**方法**

```go
func (r SysRoleResource) TableName() string
```

#### `SysSocial`

```go
type SysSocial struct
```

SysSocial 社会化关系模型

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | uint64 | gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement;comment:主键" json:"id" | 主键 |
| `UserID` | uint64 | gorm:"column:user_id;type:bigint unsigned;index;comment:用户ID" json:"userId" | 用户ID |
| `AuthID` | string | gorm:"column:auth_id;type:varchar(255);not null;index:idx_auth;comment:第三方唯一ID" json:"authId" | 第三方唯一ID |
| `Source` | string | gorm:"column:source;type:varchar(50);not null;index:idx_auth;comment:用户来源平台" json:"source" | 用户来源平台 |
| `AccessToken` | string | gorm:"column:access_token;type:varchar(1024);not null;comment:访问令牌" json:"accessToken" | 访问令牌 |
| `ExpireIn` | int | gorm:"column:expire_in;type:int;default:0;comment:令牌有效期（秒，部分平台可能没有）" json:"expireIn" | 令牌有效期（秒，部分平台可能没有） |
| `RefreshToken` | string | gorm:"column:refresh_token;type:varchar(1024);comment:刷新令牌（部分平台可能没有）" json:"refreshToken" | 刷新令牌（部分平台可能没有） |
| `OpenID` | string | gorm:"column:open_id;type:varchar(255);comment:开放ID" json:"openId" | 开放ID |
| `UserName` | string | gorm:"column:user_name;type:varchar(255);comment:第三方账号" json:"userName" | 第三方账号 |
| `NickName` | string | gorm:"column:nick_name;type:varchar(255);comment:第三方昵称" json:"nickName" | 第三方昵称 |
| `Email` | string | gorm:"column:email;type:varchar(255);comment:第三方邮箱" json:"email" | 第三方邮箱 |
| `Avatar` | string | gorm:"column:avatar;type:varchar(1024);comment:第三方头像" json:"avatar" | 第三方头像 |
| `AccessCode` | string | gorm:"column:access_code;type:varchar(512);comment:授权码（部分平台可能没有）" json:"accessCode" | 授权码（部分平台可能没有） |
| `UnionID` | string | gorm:"column:union_id;type:varchar(255);comment:联合ID" json:"unionId" | 联合ID |
| `Scope` | string | gorm:"column:scope;type:varchar(255);comment:授权范围（部分平台可能没有）" json:"scope" | 授权范围（部分平台可能没有） |
| `TokenType` | string | gorm:"column:token_type;type:varchar(50);comment:令牌类型（部分平台可能没有）" json:"tokenType" | 令牌类型（部分平台可能没有） |
| `IDToken` | string | gorm:"column:id_token;type:varchar(1024);comment:ID令牌（部分平台可能没有）" json:"idToken" | ID令牌（部分平台可能没有） |
| `BaseModel` | BaseModel |  |  |

**方法**

```go
func (r SysSocial) TableName() string
```

TableName 指定表名

#### `SysUser`

```go
type SysUser struct
```

SysUser 用户信息

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | int64 | gorm:"type:bigint(20);primary_key;auto_increment;用户ID;"     json:"id"  form:"id" | 主键ID |
| `LoginName` | string | gorm:"type:varchar(30);not null;uniqueIndex;comment:登录账号;" json:"loginName" form:"loginName" | 登录账号(全局唯一) |
| `NickName` | string | gorm:"type:varchar(30);comment:用户昵称;" json:"nickName" form:"nickName" | 用户昵称或姓名 |
| `UserType` | string | gorm:"type:varchar(10);index;comment:用户类型（根据业务自由定义）;" json:"userType" form:"userType" | 用户类型(根据业务自由定义) |
| `Email` | string | gorm:"type:varchar(50);index;comment:用户邮箱;" json:"email" form:"email" | 用户邮箱(全局唯一) |
| `Mobile` | string | gorm:"type:varchar(11);index;comment:手机号码;" json:"mobile" form:"mobile" | 手机号码(全局唯一) |
| `Sex` | uint8 | gorm:"type:tinyint;comment:用户性别（1男 2女 3未知）;" json:"sex" form:"sex" | 用户性别(1:男 2:女 3:未知) |
| `Avatar` | string | gorm:"type:varchar(100);comment:头像路径;" json:"avatar" form:"avatar" | 头像路径 |
| `Password` | string | gorm:"type:varchar(50);comment:密码;" json:"password" form:"password" | 密码 |
| `Status` | uint8 | gorm:"type:tinyint;index;default:1;comment:帐号状态（1正常 2停用）;" json:"status" form:"status" | 状态(1:正常 2:停用) |
| `LoginIp` | string | gorm:"type:varchar(50);comment:最后登陆IP;" json:"loginIp" form:"loginIp" | 最后登陆IP |
| `LoginDate` | *time.Time | gorm:"type:datetime;comment:最后登陆时间;" json:"loginDate" form:"loginDate" | 最后登陆时间 |
| `Remark` | string | gorm:"type:varchar(500);comment:备注;" json:"remark"   form:"remark" | 备注 |
| `BaseModel` | BaseModel |  |  |

**方法**

```go
func (e SysUser) TableName() string
```

映射数据表

#### `SysUserExt`

```go
type SysUserExt struct
```

SysUserExt 扩展的SysUser模型

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | int64 | gorm:"type:bigint(20);primary_key;auto_increment;ID;" json:"id" form:"id" | 主键ID |
| `UserId` | int64 | gorm:"type:bigint(20);unique;not null;comment:用户ID;" json:"userId" form:"userId" | 用户ID |
| `DepartmentId` | int64 | gorm:"type:bigint(20);index;comment:所属部门ID;" json:"departmentId" form:"departmentId" | 所属部门ID |
| `PositionId` | int64 | gorm:"type:bigint(20);index;comment:所属岗位ID;" json:"positionId" form:"positionId" | 所属岗位ID |
| `RoleId` | int64 | gorm:"type:bigint(20);index;comment:所属角色ID;" json:"roleId" form:"roleId" | 所属角色ID |

**方法**

```go
func (e SysUserExt) TableName() string
```

## `github.com/maczh/mgin/v2/pkg/models/sys/request`

```go
import "github.com/maczh/mgin/v2/pkg/models/sys/request"
```

### 类型

#### `GetApiReq`

```go
type GetApiReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | uint | json:"id" form:"id" binding:"required" |  |

#### `GetUriReq`

```go
type GetUriReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Uri` | string | json:"uri" form:"uri" binding:"required" |  |

#### `ListApiReq`

```go
type ListApiReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Group` | string | json:"group" form:"group" |  |
| `NeedAuth` | int | json:"needAuth" form:"needAuth" |  |
| `Page` | int | json:"page" form:"page" |  |
| `PageSize` | int | json:"pageSize" form:"pageSize" |  |

#### `GetSysConfigReq`

```go
type GetSysConfigReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | uint | json:"id" form:"id" | ID |
| `Key` | string | json:"key" form:"key" | 配置键名 |

#### `ListSysConfigReq`

```go
type ListSysConfigReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Module` | string | json:"module" form:"module" | 模块 |
| `Page` | int | json:"page" form:"page" | 页码 |
| `PageSize` | int | json:"pageSize" form:"pageSize" | 每页数量 |

#### `CreateDeptReq`

```go
type CreateDeptReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ParentId` | uint | json:"parentId" form:"parentId" |  |
| `Ancestors` | string | json:"ancestors" form:"ancestors" |  |
| `Name` | string | json:"name" form:"name" binding:"required" |  |
| `Sort` | int | json:"sort" form:"sort" |  |
| `Leader` | string | json:"leader" form:"leader" |  |
| `Mobile` | string | json:"mobile" form:"mobile" |  |
| `Type` | string | json:"type" form:"type" |  |

#### `GetDeptReq`

```go
type GetDeptReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | uint | json:"id" form:"id" |  |
| `Name` | string | json:"name" form:"name" |  |

#### `ListDeptReq`

```go
type ListDeptReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ParentId` | uint | json:"parentId" form:"parentId" |  |
| `Page` | int | json:"page" form:"page" |  |
| `PageSize` | int | json:"pageSize" form:"pageSize" |  |

#### `CreateDictReq`

```go
type CreateDictReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Type` | string | json:"type" form:"type" binding:"required" |  |
| `ParentId` | int | json:"parentId" form:"parentId" |  |
| `Name` | string | json:"name" form:"name" binding:"required" |  |
| `Key` | string | json:"key" form:"key" binding:"required" |  |
| `Value` | string | json:"value" form:"value" binding:"required" |  |
| `Sort` | int | json:"sort" form:"sort" |  |
| `Remark` | string | json:"remark" form:"remark" |  |

#### `GetDictReq`

```go
type GetDictReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | uint | json:"id" form:"id" |  |
| `Type` | string | json:"type" form:"type" |  |
| `Name` | string | json:"name" form:"name" |  |
| `Key` | string | json:"key" form:"key" |  |

#### `ListDictReq`

```go
type ListDictReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Type` | string | json:"type" form:"type" |  |
| `ParentId` | int | json:"parentId" form:"parentId" |  |
| `Name` | string | json:"name" form:"name" |  |
| `Page` | int | json:"page" form:"page" |  |
| `PageSize` | int | json:"pageSize" form:"pageSize" |  |

#### `CreatePostReq`

```go
type CreatePostReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `PostCode` | string | json:"post_code" form:"post_code" binding:"required" |  |
| `PostName` | string | json:"post_name" form:"post_name" binding:"required" |  |
| `DeptId` | int64 | json:"dept_id" form:"dept_id" binding:"required" |  |

#### `GetPostReq`

```go
type GetPostReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | int64 | json:"id" form:"id" |  |
| `PostCode` | string | json:"post_code" form:"post_code" |  |
| `PostName` | string | json:"post_name" form:"post_name" |  |

#### `ListPostReq`

```go
type ListPostReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `DeptId` | int64 | json:"dept_id" form:"dept_id" |  |
| `Page` | int | json:"page" form:"page" |  |
| `PageSize` | int | json:"pageSize" form:"pageSize" |  |

#### `CreateResourceReq`

```go
type CreateResourceReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ResourceLevel` | int | json:"menuLevel" form:"menuLevel" binding:"required" | 资源等级(1主资源 2子资源) |
| `ParentId` | uint | json:"parentId" form:"parentId" |  |
| `Path` | string | json:"path" form:"path" binding:"required" |  |
| `Name` | string | json:"name" form:"name" binding:"required" |  |
| `Component` | string | json:"component" form:"component" binding:"required" |  |
| `Redirect` | string | json:"redirect" form:"redirect" |  |
| `Icon` | string | json:"icon" form:"icon" |  |
| `Title` | string | json:"title" form:"title" binding:"required" |  |
| `Sort` | int | json:"sort" form:"sort" |  |
| `Hidden` | uint8 | json:"hidden" form:"hidden" |  |
| `KeepAlive` | uint8 | json:"keepAlive" form:"keepAlive" |  |
| `AlwaysShow` | uint8 | json:"alwaysShow" form:"alwaysShow" |  |
| `Breadcrumb` | uint8 | json:"breadcrumb" form:"breadcrumb" |  |
| `Affix` | uint8 | json:"affix" form:"affix" |  |
| `ActiveResource` | string | json:"activeResource" form:"activeResource" |  |
| `Status` | int | json:"status" form:"status" |  |
| `ApiId` | *uint | json:"apiId" form:"apiId" | 资源关联的apiId |

#### `GetResourceReq`

```go
type GetResourceReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | uint | json:"id" form:"id" |  |
| `Title` | string | json:"title" form:"title" | 资源标题 |

#### `ListResourceReq`

```go
type ListResourceReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ParentId` | uint | json:"parentId" form:"parentId" |  |
| `Path` | string | json:"path" form:"path" | 资源路径 |
| `Name` | string | json:"name" form:"name" | 资源名称 |
| `Component` | string | json:"component" form:"component" | 组件路径 |
| `Title` | string | json:"title" form:"title" | 资源标题 |
| `Status` | int | json:"status" form:"status" |  |
| `Page` | int | json:"page" form:"page" |  |
| `PageSize` | int | json:"pageSize" form:"pageSize" |  |

#### `GetTreeResourceReq`

```go
type GetTreeResourceReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ParentId` | uint | json:"parentId" form:"parentId" | 父级资源id，0表示获取所有一级资源 |
| `ByRole` | bool | json:"byRole" form:"byRole" | 按角色获取资源 |

#### `CreateRoleReq`

```go
type CreateRoleReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Name` | string | json:"name" form:"name" binding:"required" |  |
| `Ident` | string | json:"ident" form:"ident" binding:"required" |  |
| `Description` | string | json:"description" form:"description" |  |

#### `GetRoleReq`

```go
type GetRoleReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | uint | json:"id" form:"id" |  |
| `Name` | string | json:"name" form:"name" |  |
| `Ident` | string | json:"ident" form:"ident" |  |

#### `ListRoleReq`

```go
type ListRoleReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Page` | int | json:"page" form:"page" |  |
| `PageSize` | int | json:"pageSize" form:"pageSize" |  |

#### `BindRoleApiReq`

```go
type BindRoleApiReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `RoleId` | int64 | json:"roleId" form:"roleId" binding:"required" |  |
| `ApiIds` | []int64 | json:"apiIds" form:"apiIds" binding:"required" |  |

#### `ListRoleApiReq`

```go
type ListRoleApiReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `RoleId` | int64 | json:"roleId" form:"roleId" binding:"required" |  |

#### `BindRoleResourceReq`

```go
type BindRoleResourceReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `RoleId` | int64 | json:"roleId" form:"roleId" binding:"required" |  |
| `ResourceIds` | []int64 | json:"resourceIds" form:"resourceIds" binding:"required" |  |

#### `ListRoleResourceReq`

```go
type ListRoleResourceReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `RoleId` | int64 | json:"roleId" form:"roleId" binding:"required" |  |

#### `RegisterReq`

```go
type RegisterReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `LoginName` | string | json:"loginName" form:"loginName" binding:"required" |  |
| `Password` | string | json:"password" form:"password" binding:"required" |  |
| `Email` | string | json:"email" form:"email" |  |
| `Mobile` | string | json:"mobile" form:"mobile" |  |
| `NickName` | string | json:"nickName" form:"nickName" |  |
| `Sex` | uint8 | json:"sex" form:"sex" |  |
| `Avatar` | string | json:"avatar" form:"avatar" |  |
| `Captcha` | *VerifyCaptchaReq | json:"captcha" form:"captcha" binding:"required" |  |

#### `LoginReq`

```go
type LoginReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `LoginName` | string | json:"loginName" form:"loginName" binding:"required" | 可以是用户名或邮箱或手机号，自动识别 |
| `Password` | string | json:"password" form:"password" binding:"required" |  |
| `Captcha` | *VerifyCaptchaReq | json:"captcha" form:"captcha" binding:"required" |  |

#### `ChangePasswordReq`

```go
type ChangePasswordReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `LoginName` | string | json:"loginName" form:"loginName" binding:"required" |  |
| `OldPassword` | string | json:"oldPassword" form:"oldPassword" binding:"required" |  |
| `NewPassword` | string | json:"newPassword" form:"newPassword" binding:"required" |  |

#### `VerifyTokenReq`

```go
type VerifyTokenReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Token` | string | json:"token" form:"token" binding:"required" |  |

#### `ListSysUserReq`

```go
type ListSysUserReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Keyword` | string | json:"keyword" form:"keyword" |  |
| `Status` | uint8 | json:"status" form:"status" |  |
| `UserType` | string | json:"userType" form:"userType" |  |
| `DeptId` | uint64 | json:"deptId" form:"deptId" |  |
| `RoleId` | uint64 | json:"roleId" form:"roleId" |  |
| `PostId` | uint64 | json:"postId" form:"postId" |  |
| `Page` | int | json:"page" form:"page" |  |
| `PageSize` | int | json:"pageSize" form:"pageSize" |  |

#### `GetSysUserReq`

```go
type GetSysUserReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | uint64 | json:"id" form:"id" |  |
| `LoginName` | string | json:"loginName" form:"loginName" |  |
| `Email` | string | json:"email" form:"email" |  |
| `Mobile` | string | json:"mobile" form:"mobile" |  |
| `NickName` | string | json:"nickName" form:"nickName" |  |

#### `GetCaptchaReq`

```go
type GetCaptchaReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Width` | int | json:"width" form:"width" binding:"required" |  |
| `Height` | int | json:"height" form:"height" binding:"required" |  |
| `Length` | int | json:"length" form:"length" |  |
| `Type` | int | json:"type" form:"type" | 0:数字 1:字母 2:算术 |

#### `VerifyCaptchaReq`

```go
type VerifyCaptchaReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | string | json:"id" form:"id" binding:"required" |  |
| `Code` | string | json:"code" form:"code" binding:"required" |  |

#### `DeleteByIdReq`

```go
type DeleteByIdReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | int64 | json:"id" form:"id" binding:"required" |  |

#### `GetByIdReq`

```go
type GetByIdReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | *int64 | json:"id" form:"id" binding:"required" |  |

#### `ChangeStatusReq`

```go
type ChangeStatusReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ID` | int64 | json:"id" form:"id" binding:"required" |  |
| `Status` | *int | json:"status" form:"status" binding:"required" |  |

#### `CreateSysUserExtReq`

```go
type CreateSysUserExtReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `UserId` | uint | json:"userId" form:"userId" binding:"required" |  |
| `ListSysUserExtReq` | ListSysUserExtReq |  |  |

#### `ListSysUserExtReq`

```go
type ListSysUserExtReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `DepartmentId` | int | json:"departmentId" form:"departmentId" |  |
| `RoleId` | int | json:"roleId" form:"roleId" |  |
| `PositionId` | int | json:"positionId" form:"positionId" |  |
| `Page` | int | json:"page" form:"page" |  |
| `PageSize` | int | json:"pageSize" form:"pageSize" |  |

#### `GetSysUserExtReq`

```go
type GetSysUserExtReq struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `UserId` | uint | json:"userId" form:"userId" binding:"required" |  |

## `github.com/maczh/mgin/v2/pkg/models/sys/vo`

```go
import "github.com/maczh/mgin/v2/pkg/models/sys/vo"
```

### 类型

#### `ListSysApiByGroupResp`

```go
type ListSysApiByGroupResp struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Group` | string | json:"group" |  |
| `Apis` | []sys.SysApi | json:"apis" |  |

#### `ListRoleApiResp`

```go
type ListRoleApiResp struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `RoleId` | int64 | json:"roleId" |  |
| `RoleName` | string | json:"roleName" |  |
| `Apis` | []ListRoleApiItem | json:"apis" |  |

#### `ListRoleApiItem`

```go
type ListRoleApiItem struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `ApiId` | int64 | json:"apiId" |  |
| `ApiPath` | string | json:"apiPath" |  |
| `Name` | string | json:"name" |  |
| `Method` | string | json:"method" |  |
| `Group` | string | json:"group" |  |

## `github.com/maczh/mgin/v2/pkg/vo`

```go
import "github.com/maczh/mgin/v2/pkg/vo"
```

### 常量

| 名称 | 类型/值 | 说明 |
|------|---------|------|
| `ERROR` | = 500 |  |
| `SUCCESS` | = 1 |  |
| `UnLogin` | = 999 |  |

### 类型

#### `Response`

```go
type Response[T any] struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Code` | int | json:"code" |  |
| `Data` | T | json:"data" |  |
| `Msg` | string | json:"msg" |  |
| `SrvTime` | time.Time | json:"srvTime" |  |

**方法**

```go
func (r Response[T]) ToAny() Response[any]
```

#### `ResponsePage`

```go
type ResponsePage[T any] struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `Code` | int | json:"code" |  |
| `Msg` | string | json:"msg" |  |
| `SrvTime` | time.Time | json:"srvTime" |  |
| `Data` | *PageData[T] | json:"data" |  |

**方法**

```go
func (r *ResponsePage[T]) ToAny() ResponsePage[any]
```

#### `PageData`

```go
type PageData[T any] struct
```

| 字段 | 类型 | tag | 说明 |
|------|------|-----|------|
| `List` | []T | json:"list" |  |
| `Total` | int64 | json:"total" |  |
| `Page` | int | json:"page" |  |
| `PageSize` | int | json:"pageSize" |  |

### 函数

#### `Error`

```go
func Error(status int, msg string) Response[any]
```

#### `ErrorPage`

```go
func ErrorPage(status int, msg string) ResponsePage[any]
```

#### `ErrorPageT`

```go
func ErrorPageT[T any](status int, msg string) ResponsePage[T]
```

#### `ErrorT`

```go
func ErrorT[T any](status int, msg string) Response[T]
```

#### `ResponsePageToT`

```go
func ResponsePageToT[T any](result ResponsePage[any]) ResponsePage[T]
```

#### `ResponseToT`

```go
func ResponseToT[T any](result Response[any]) Response[T]
```

#### `Success`

```go
func Success(data any) Response[any]
```

#### `SuccessPage`

```go
func SuccessPage(data any, page *models.ResultPage) ResponsePage[any]
```

#### `SuccessT`

```go
func SuccessT[T any](data T) Response[T]
```

#### `ToResponse`

```go
func ToResponse(r models.Result[any]) Response[any]
```

#### `ToResponsePage`

```go
func ToResponsePage(r models.Result[any]) ResponsePage[any]
```

