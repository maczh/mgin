package dao

import (
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/utils"
	"gorm.io/gorm"
)

func InitDB() {
	conn, err := db.Mysql.GetConnection()
	if err != nil {
		logs.Error("get mysql connection error: {}", err.Error())
		return
	}
	// 迁移模型到数据库
	err = conn.AutoMigrate(
		&sys.SysUser{},
		&sys.SysUserExt{},
		&sys.SysRole{},
		&sys.SysMenu{},
		&sys.SysApi{},
		&sys.SysDept{},
		&sys.SysDict{},
		&sys.SysPost{},
		&sys.SysRoleApi{},
		&sys.SysRoleMenu{},
	)
	if err != nil {
		logs.Error("migrate models to db error: {}", err.Error())
		return
	}
	//初始化数据
	initData(conn)
}

func initData(db *gorm.DB) {
	//初始化管理员账号
	admin := sys.SysUser{Id: 1, LoginName: "admin", Password: utils.MD5Encode("admin"), Sex: 1, Status: 1, Email: "admin@mgin.org", Mobile: "13800138000", NickName: "超级管理员"}
	err := db.Create(&admin).Error
	if err != nil {
		logs.Error("create admin user error: {}", err.Error())
	}
	//初始化管理员角色
	roles := []*sys.SysRole{
		{ID: 1, RoleName: "超级管理员", RoleIdent: "supervisor", Description: "全部权限"},
		{ID: 2, RoleName: "管理员", RoleIdent: "admin", Description: "具备系统管理权限"},
		{ID: 3, RoleName: "普通用户", RoleIdent: "user", Description: "具备普通用户权限"},
	}
	err = SysRoleDao.MultiCreate(roles)
	if err != nil {
		logs.Error("create roles error: {}", err.Error())
	}
	//初始化部门
	depts := []*sys.SysDept{
		{Id: 1, Name: "行政部", ParentId: 0, Sort: 1, Status: 1},
		{Id: 2, Name: "技术部", ParentId: 0, Sort: 2, Status: 1},
		{Id: 3, Name: "市场部", ParentId: 0, Sort: 3, Status: 1},
		{Id: 4, Name: "销售部", ParentId: 0, Sort: 4, Status: 1},
		{Id: 5, Name: "客服部", ParentId: 0, Sort: 5, Status: 1},
		{Id: 6, Name: "运营部", ParentId: 0, Sort: 6, Status: 1},
		{Id: 7, Name: "财务部", ParentId: 0, Sort: 7, Status: 1},
		{Id: 8, Name: "人事部", ParentId: 0, Sort: 8, Status: 1},
	}
	err = SysDeptDao.MultiCreate(depts)
	if err != nil {
		logs.Error("create depts error: {}", err.Error())
	}
	//初始化岗位
	posts := []*sys.SysPost{
		{Id: 1, PostName: "总经理", PostCode: "001", Sort: 1, Status: 1},
		{Id: 2, PostName: "副总经理", PostCode: "002", Sort: 2, Status: 1},
		{Id: 3, PostName: "部门经理", PostCode: "003", Sort: 3, Status: 1},
		{Id: 4, PostName: "职员", PostCode: "004", Sort: 4, Status: 1},
	}
	err = SysPostDao.MultiCreate(posts)
	if err != nil {
		logs.Error("create posts error: {}", err.Error())
	}
	// 初始化用户扩展属性
	userExt := sys.SysUserExt{Id: 1, UserId: 1, DepartmentId: 2, PositionId: 2}
	err = db.Create(&userExt).Error
	if err != nil {
		logs.Error("create user ext error: {}", err.Error())
	}
	// API 列表
	apiList := make([]*sys.SysApi, 0)
	apiList = append(apiList, &sys.SysApi{ID: 1, APIPath: config.Config.Sys.BaseUri + "/login", Method: "POST", Name: "登录", Description: "用户登录，可以用用户名、手机号、邮箱作为登录名", APIGroup: "用户模块", NeedAuth: false})
	apiList = append(apiList, &sys.SysApi{ID: 2, APIPath: config.Config.Sys.BaseUri + "/register", Method: "POST", Name: "注册", Description: "用户注册", APIGroup: "用户模块", NeedAuth: false})
	apiList = append(apiList, &sys.SysApi{ID: 3, APIPath: config.Config.Sys.BaseUri + "/captcha/get", Method: "GET", Name: "获取图片验证码", Description: "获取图片验证码", APIGroup: "用户模块", NeedAuth: false})
	apiList = append(apiList, &sys.SysApi{ID: 4, APIPath: config.Config.Sys.BaseUri + "/captcha/verify", Method: "POST", Name: "图片验证码验证", Description: "图片验证码验证", APIGroup: "用户模块", NeedAuth: false})
	apiList = append(apiList, &sys.SysApi{ID: 5, APIPath: config.Config.Sys.BaseUri + "/users/add", Method: "POST", Name: "新增用户", Description: "管理员添加一个新用户", APIGroup: "用户模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 6, APIPath: config.Config.Sys.BaseUri + "/users/update", Method: "POST", Name: "修改用户信息", Description: "", APIGroup: "用户模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 7, APIPath: config.Config.Sys.BaseUri + "/users/del", Method: "POST", Name: "删除用户", Description: "", APIGroup: "用户模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 8, APIPath: config.Config.Sys.BaseUri + "/users/get", Method: "GET", Name: "获取用户信息", Description: "", APIGroup: "用户模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 9, APIPath: config.Config.Sys.BaseUri + "/users/list", Method: "GET", Name: "分页查询用户", Description: "分页查询用户", APIGroup: "用户模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 10, APIPath: config.Config.Sys.BaseUri + "/users/logout", Method: "POST", Name: "退出登录", Description: "", APIGroup: "用户模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 11, APIPath: config.Config.Sys.BaseUri + "/users/pwd", Method: "POST", Name: "修改密码", Description: "", APIGroup: "用户模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 12, APIPath: config.Config.Sys.BaseUri + "/users/status", Method: "POST", Name: "修改用户状态", Description: "", APIGroup: "用户模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 13, APIPath: config.Config.Sys.BaseUri + "/token", Method: "POST", Name: "JWT Token验证", Description: "token 验证", APIGroup: "用户模块", NeedAuth: false})
	apiList = append(apiList, &sys.SysApi{ID: 14, APIPath: config.Config.Sys.BaseUri + "/sys_api/add", Method: "POST", Name: "新增API", Description: "管理员添加一个新API", APIGroup: "API模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 15, APIPath: config.Config.Sys.BaseUri + "/sys_api/update", Method: "POST", Name: "修改API信息", Description: "", APIGroup: "API模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 16, APIPath: config.Config.Sys.BaseUri + "/sys_api/del", Method: "POST", Name: "删除API", Description: "", APIGroup: "API模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 17, APIPath: config.Config.Sys.BaseUri + "/sys_api/get", Method: "GET", Name: "获取API信息", Description: "", APIGroup: "API模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 18, APIPath: config.Config.Sys.BaseUri + "/sys_api/get/uri", Method: "GET", Name: "按URI路径获取API信息", Description: "", APIGroup: "API模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 19, APIPath: config.Config.Sys.BaseUri + "/sys_api/list", Method: "GET", Name: "分页查询API接口", Description: "分页查询API", APIGroup: "API模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 20, APIPath: config.Config.Sys.BaseUri + "/dept/add", Method: "POST", Name: "新增部门", Description: "管理员添加一个新部门", APIGroup: "部门模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 21, APIPath: config.Config.Sys.BaseUri + "/dept/update", Method: "POST", Name: "修改部门信息", Description: "", APIGroup: "部门模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 22, APIPath: config.Config.Sys.BaseUri + "/dept/del", Method: "POST", Name: "删除部门", Description: "", APIGroup: "部门模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 23, APIPath: config.Config.Sys.BaseUri + "/dept/get", Method: "GET", Name: "获取部门信息", Description: "", APIGroup: "部门模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 24, APIPath: config.Config.Sys.BaseUri + "/dept/tree", Method: "GET", Name: "获取部门树", Description: "", APIGroup: "部门模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 25, APIPath: config.Config.Sys.BaseUri + "/dept/list", Method: "GET", Name: "分页查询部门", Description: "分页查询部门", APIGroup: "部门模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 26, APIPath: config.Config.Sys.BaseUri + "/dict/add", Method: "POST", Name: "新增字典", Description: "管理员添加一个新字典", APIGroup: "字典模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 27, APIPath: config.Config.Sys.BaseUri + "/dict/update", Method: "POST", Name: "修改字典信息", Description: "", APIGroup: "字典模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 28, APIPath: config.Config.Sys.BaseUri + "/dict/del", Method: "POST", Name: "删除字典", Description: "", APIGroup: "字典模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 29, APIPath: config.Config.Sys.BaseUri + "/dict/get", Method: "GET", Name: "获取字典信息", Description: "", APIGroup: "字典模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 30, APIPath: config.Config.Sys.BaseUri + "/dict/list", Method: "GET", Name: "分页查询字典", Description: "分页查询字典", APIGroup: "字典模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 31, APIPath: config.Config.Sys.BaseUri + "/role/add", Method: "POST", Name: "新增角色", Description: "管理员添加一个新角色", APIGroup: "角色模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 32, APIPath: config.Config.Sys.BaseUri + "/role/update", Method: "POST", Name: "修改角色信息", Description: "", APIGroup: "角色模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 33, APIPath: config.Config.Sys.BaseUri + "/role/del", Method: "POST", Name: "删除角色", Description: "", APIGroup: "角色模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 34, APIPath: config.Config.Sys.BaseUri + "/role/get", Method: "GET", Name: "获取角色信息", Description: "", APIGroup: "角色模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 35, APIPath: config.Config.Sys.BaseUri + "/role/list", Method: "GET", Name: "分页查询角色", Description: "分页查询角色", APIGroup: "角色模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 36, APIPath: config.Config.Sys.BaseUri + "/user/ext/add", Method: "POST", Name: "新增用户扩展属性", Description: "管理员添加一个新用户扩展属性", APIGroup: "用户扩展属性模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 37, APIPath: config.Config.Sys.BaseUri + "/user/ext/update", Method: "POST", Name: "修改用户扩展属性信息", Description: "", APIGroup: "用户扩展属性模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 38, APIPath: config.Config.Sys.BaseUri + "/user/ext/del", Method: "POST", Name: "删除用户扩展属性", Description: "", APIGroup: "用户扩展属性模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 39, APIPath: config.Config.Sys.BaseUri + "/user/ext/get", Method: "GET", Name: "获取用户扩展属性信息", Description: "", APIGroup: "用户扩展属性模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 40, APIPath: config.Config.Sys.BaseUri + "/user/ext/list", Method: "GET", Name: "分页查询用户扩展属性", Description: "分页查询用户扩展属性", APIGroup: "用户扩展属性模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 41, APIPath: config.Config.Sys.BaseUri + "/post/add", Method: "POST", Name: "新增岗位", Description: "管理员添加一个新岗位", APIGroup: "岗位模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 42, APIPath: config.Config.Sys.BaseUri + "/post/update", Method: "POST", Name: "修改岗位信息", Description: "", APIGroup: "岗位模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 43, APIPath: config.Config.Sys.BaseUri + "/post/del", Method: "POST", Name: "删除岗位", Description: "", APIGroup: "岗位模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 44, APIPath: config.Config.Sys.BaseUri + "/post/get", Method: "GET", Name: "获取岗位信息", Description: "", APIGroup: "岗位模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 45, APIPath: config.Config.Sys.BaseUri + "/post/list", Method: "GET", Name: "分页查询岗位", Description: "分页查询岗位", APIGroup: "岗位模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 46, APIPath: config.Config.Sys.BaseUri + "/menu/add", Method: "POST", Name: "新增菜单", Description: "管理员添加一个新菜单", APIGroup: "菜单模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 47, APIPath: config.Config.Sys.BaseUri + "/menu/update", Method: "POST", Name: "修改菜单信息", Description: "", APIGroup: "菜单模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 48, APIPath: config.Config.Sys.BaseUri + "/menu/del", Method: "POST", Name: "删除菜单", Description: "", APIGroup: "菜单模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 49, APIPath: config.Config.Sys.BaseUri + "/menu/get", Method: "GET", Name: "获取菜单信息", Description: "", APIGroup: "菜单模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 50, APIPath: config.Config.Sys.BaseUri + "/menu/tree", Method: "GET", Name: "获取菜单树", Description: "", APIGroup: "菜单模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 51, APIPath: config.Config.Sys.BaseUri + "/menu/list", Method: "GET", Name: "分页查询菜单", Description: "分页查询菜单", APIGroup: "菜单模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 52, APIPath: config.Config.Sys.BaseUri + "/role_api/bind", Method: "POST", Name: "绑定角色API接口权限", Description: "批量全量绑定", APIGroup: "权限模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 53, APIPath: config.Config.Sys.BaseUri + "/role_api/list", Method: "GET", Name: "获取角色API接口权限列表", Description: "", APIGroup: "权限模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 54, APIPath: config.Config.Sys.BaseUri + "/role_menu/bind", Method: "POST", Name: "绑定角色菜单权限", Description: "全量绑定角色菜单", APIGroup: "权限模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 55, APIPath: config.Config.Sys.BaseUri + "/role_menu/list", Method: "GET", Name: "获取角色菜单权限列表", Description: "", APIGroup: "权限模块", NeedAuth: true})

	err = SysApiDao.MultiCreate(apiList)
	if err != nil {
		logs.Error("create api error: {}", err.Error())
	}

	// 初始化角色API权限
	roleApiList := []*sys.SysRoleApi{
		{RoleId: 1, ApiId: 1},
		{RoleId: 1, ApiId: 2},
		{RoleId: 1, ApiId: 3},
		{RoleId: 1, ApiId: 4},
		{RoleId: 1, ApiId: 5},
		{RoleId: 1, ApiId: 6},
		{RoleId: 1, ApiId: 7},
		{RoleId: 1, ApiId: 8},
		{RoleId: 1, ApiId: 9},
		{RoleId: 1, ApiId: 10},
		{RoleId: 1, ApiId: 11},
		{RoleId: 1, ApiId: 12},
		{RoleId: 1, ApiId: 13},
		{RoleId: 1, ApiId: 14},
		{RoleId: 1, ApiId: 15},
		{RoleId: 1, ApiId: 16},
		{RoleId: 1, ApiId: 17},
		{RoleId: 1, ApiId: 18},
		{RoleId: 1, ApiId: 19},
		{RoleId: 1, ApiId: 20},
		{RoleId: 1, ApiId: 21},
		{RoleId: 1, ApiId: 22},
		{RoleId: 1, ApiId: 23},
		{RoleId: 1, ApiId: 24},
		{RoleId: 1, ApiId: 25},
		{RoleId: 1, ApiId: 26},
		{RoleId: 1, ApiId: 27},
		{RoleId: 1, ApiId: 28},
		{RoleId: 1, ApiId: 29},
		{RoleId: 1, ApiId: 30},
		{RoleId: 1, ApiId: 31},
		{RoleId: 1, ApiId: 32},
		{RoleId: 1, ApiId: 33},
		{RoleId: 1, ApiId: 34},
		{RoleId: 1, ApiId: 35},
		{RoleId: 1, ApiId: 36},
		{RoleId: 1, ApiId: 37},
		{RoleId: 1, ApiId: 38},
		{RoleId: 1, ApiId: 39},
		{RoleId: 1, ApiId: 40},
		{RoleId: 1, ApiId: 41},
		{RoleId: 1, ApiId: 42},
		{RoleId: 1, ApiId: 43},
		{RoleId: 1, ApiId: 44},
		{RoleId: 1, ApiId: 45},
		{RoleId: 1, ApiId: 46},
		{RoleId: 1, ApiId: 47},
		{RoleId: 1, ApiId: 48},
		{RoleId: 1, ApiId: 49},
		{RoleId: 1, ApiId: 50},
		{RoleId: 1, ApiId: 51},
		{RoleId: 1, ApiId: 52},
		{RoleId: 1, ApiId: 53},
		{RoleId: 1, ApiId: 54},
		{RoleId: 1, ApiId: 55},
		{RoleId: 2, ApiId: 1},
		{RoleId: 2, ApiId: 2},
		{RoleId: 2, ApiId: 3},
		{RoleId: 2, ApiId: 4},
		{RoleId: 2, ApiId: 5},
		{RoleId: 2, ApiId: 6},
		{RoleId: 2, ApiId: 7},
		{RoleId: 2, ApiId: 8},
		{RoleId: 2, ApiId: 9},
		{RoleId: 2, ApiId: 10},
		{RoleId: 2, ApiId: 11},
		{RoleId: 2, ApiId: 12},
		{RoleId: 2, ApiId: 13},
		{RoleId: 2, ApiId: 14},
		{RoleId: 2, ApiId: 15},
		{RoleId: 2, ApiId: 16},
		{RoleId: 2, ApiId: 17},
		{RoleId: 2, ApiId: 18},
		{RoleId: 2, ApiId: 19},
		{RoleId: 2, ApiId: 20},
		{RoleId: 2, ApiId: 21},
		{RoleId: 2, ApiId: 22},
		{RoleId: 2, ApiId: 23},
		{RoleId: 2, ApiId: 24},
		{RoleId: 2, ApiId: 25},
		{RoleId: 2, ApiId: 26},
		{RoleId: 2, ApiId: 27},
		{RoleId: 2, ApiId: 28},
		{RoleId: 2, ApiId: 29},
		{RoleId: 2, ApiId: 30},
		{RoleId: 2, ApiId: 31},
		{RoleId: 2, ApiId: 32},
		{RoleId: 2, ApiId: 33},
		{RoleId: 2, ApiId: 34},
		{RoleId: 2, ApiId: 35},
		{RoleId: 2, ApiId: 36},
		{RoleId: 2, ApiId: 37},
		{RoleId: 2, ApiId: 38},
		{RoleId: 2, ApiId: 39},
		{RoleId: 2, ApiId: 40},
		{RoleId: 2, ApiId: 41},
		{RoleId: 2, ApiId: 42},
		{RoleId: 2, ApiId: 43},
		{RoleId: 2, ApiId: 44},
		{RoleId: 2, ApiId: 45},
		{RoleId: 2, ApiId: 46},
		{RoleId: 2, ApiId: 47},
		{RoleId: 2, ApiId: 48},
		{RoleId: 2, ApiId: 49},
		{RoleId: 2, ApiId: 50},
		{RoleId: 2, ApiId: 51},
		{RoleId: 2, ApiId: 52},
		{RoleId: 2, ApiId: 53},
		{RoleId: 2, ApiId: 54},
		{RoleId: 2, ApiId: 55},
		{RoleId: 3, ApiId: 1},
		{RoleId: 3, ApiId: 2},
		{RoleId: 3, ApiId: 3},
		{RoleId: 3, ApiId: 4},
		{RoleId: 3, ApiId: 6},
		{RoleId: 3, ApiId: 8},
		{RoleId: 3, ApiId: 9},
		{RoleId: 3, ApiId: 10},
		{RoleId: 3, ApiId: 11},
		{RoleId: 3, ApiId: 13},
		{RoleId: 3, ApiId: 17},
		{RoleId: 3, ApiId: 18},
		{RoleId: 3, ApiId: 19},
		{RoleId: 3, ApiId: 23},
		{RoleId: 3, ApiId: 24},
		{RoleId: 3, ApiId: 25},
		{RoleId: 3, ApiId: 26},
		{RoleId: 3, ApiId: 27},
		{RoleId: 3, ApiId: 28},
		{RoleId: 3, ApiId: 29},
		{RoleId: 3, ApiId: 30},
		{RoleId: 3, ApiId: 34},
		{RoleId: 3, ApiId: 35},
		{RoleId: 3, ApiId: 39},
		{RoleId: 3, ApiId: 40},
		{RoleId: 3, ApiId: 44},
		{RoleId: 3, ApiId: 45},
		{RoleId: 3, ApiId: 49},
		{RoleId: 3, ApiId: 50},
		{RoleId: 3, ApiId: 51},
		{RoleId: 3, ApiId: 53},
		{RoleId: 3, ApiId: 55},
	}
	err = SysRoleApiDao.MultiCreate(roleApiList)
	if err != nil {
		logs.Error("create role api error: {}", err.Error())
	}
}
