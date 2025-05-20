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
	admin := sys.SysUser{Id: 1, LoginName: "admin", Password: utils.MD5Encode("admin"), Sex: 1, Status: 1}
	err := db.Create(&admin).Error
	if err != nil {
		logs.Error("create admin user error: {}", err.Error())
	}
	//初始化管理员角色
	roles := []*sys.SysRole{
		{ID: 1, RoleName: "超级管理员", RoleIdent: "supervisor", Description: "全部权限"},
		{ID: 2, RoleName: "管理员", RoleIdent: "admin", Description: "具备系统管理权限"},
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

	err = SysApiDao.MultiCreate(apiList)
	if err != nil {
		logs.Error("create api error: {}", err.Error())
	}
}
