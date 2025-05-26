package dao

import (
	"github.com/maczh/mgin/casbin"
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
		&sys.SysResource{},
		&sys.SysApi{},
		&sys.SysDept{},
		&sys.SysDict{},
		&sys.SysPost{},
		&sys.SysRoleApi{},
		&sys.SysRoleResource{},
		&sys.SysConfig{},
	)
	if err != nil {
		logs.Error("migrate models to db error: {}", err.Error())
		return
	}
	//初始化数据
	initData(conn)
}

func initData(mysql *gorm.DB) {
	//初始化管理员账号
	admin := sys.SysUser{ID: 1, LoginName: "admin", Password: utils.MD5Encode("admin"), Sex: 1, Status: 1, Email: "admin@mgin.org", Mobile: "13800138000", NickName: "超级管理员"}
	err := mysql.Create(&admin).Error
	if err != nil {
		logs.Error("create admin user error: {}", err.Error())
	}
	//初始化管理员角色
	roles := []*sys.SysRole{
		{ID: 1, RoleName: "超级管理员", RoleIdent: "supervisor", Description: "全部权限"},
		{ID: 2, RoleName: "管理员", RoleIdent: "admin", Description: "具备系统管理权限"},
		{ID: 3, RoleName: "普通用户", RoleIdent: "user", Description: "具备普通用户权限"},
	}
	for _, v := range roles {
		err = SysRoleDao.Create(v)
		if err != nil {
			logs.Error("create role error: {}", err.Error())
		}
	}
	//初始化部门
	depts := []*sys.SysDept{
		{ID: 1, Name: "行政部", ParentId: 0, Sort: 1, Status: 1},
		{ID: 2, Name: "技术部", ParentId: 0, Sort: 2, Status: 1},
		{ID: 3, Name: "市场部", ParentId: 0, Sort: 3, Status: 1},
		{ID: 4, Name: "销售部", ParentId: 0, Sort: 4, Status: 1},
		{ID: 5, Name: "客服部", ParentId: 0, Sort: 5, Status: 1},
		{ID: 6, Name: "运营部", ParentId: 0, Sort: 6, Status: 1},
		{ID: 7, Name: "财务部", ParentId: 0, Sort: 7, Status: 1},
		{ID: 8, Name: "人事部", ParentId: 0, Sort: 8, Status: 1},
	}
	for _, v := range depts {
		err = SysDeptDao.Create(v)
		if err != nil {
			logs.Error("create dept error: {}", err.Error())
		}
	}

	//初始化岗位
	posts := []*sys.SysPost{
		{ID: 1, PostName: "总经理", PostCode: "001", Sort: 1, Status: 1},
		{ID: 2, PostName: "副总经理", PostCode: "002", Sort: 2, Status: 1},
		{ID: 3, PostName: "部门经理", PostCode: "003", Sort: 3, Status: 1},
		{ID: 4, PostName: "职员", PostCode: "004", Sort: 4, Status: 1},
	}
	for _, v := range posts {
		err = SysPostDao.Create(v)
		if err != nil {
			logs.Error("create post error: {}", err.Error())
		}
	}
	// 初始化用户扩展属性
	userExt := sys.SysUserExt{ID: 1, UserId: 1, DepartmentId: 2, PositionId: 2, RoleId: 1}
	err = mysql.Create(&userExt).Error
	if err != nil {
		logs.Error("create user ext error: {}", err.Error())
	}
	// API 列表
	apiList := make([]*sys.SysApi, 0)
	apiList = append(apiList, &sys.SysApi{ID: 1, APIPath: config.Config.Sys.BaseUri + "/login", Method: "POST", Name: "登录", Description: "用户登录，可以用用户名、手机号、邮箱作为登录名", APIGroup: "用户模块", Enabled: 1, NeedLog: 1, NeedAuth: 0})
	apiList = append(apiList, &sys.SysApi{ID: 2, APIPath: config.Config.Sys.BaseUri + "/register", Method: "POST", Name: "注册", Description: "用户注册", APIGroup: "用户模块", Enabled: 1, NeedLog: 1, NeedAuth: 0})
	apiList = append(apiList, &sys.SysApi{ID: 3, APIPath: config.Config.Sys.BaseUri + "/sys/captcha/get", Method: "GET", Name: "获取图片验证码", Description: "获取图片验证码", APIGroup: "用户模块", Enabled: 1, NeedLog: 1, NeedAuth: 0})
	apiList = append(apiList, &sys.SysApi{ID: 4, APIPath: config.Config.Sys.BaseUri + "/sys/captcha/verify", Method: "POST", Name: "图片验证码验证", Description: "图片验证码验证", APIGroup: "用户模块", Enabled: 1, NeedLog: 1, NeedAuth: 0})
	apiList = append(apiList, &sys.SysApi{ID: 5, APIPath: config.Config.Sys.BaseUri + "/sys/users/add", Method: "POST", Name: "新增用户", Description: "管理员添加一个新用户", APIGroup: "用户模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 6, APIPath: config.Config.Sys.BaseUri + "/sys/users/update", Method: "POST", Name: "修改用户信息", Description: "", APIGroup: "用户模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 7, APIPath: config.Config.Sys.BaseUri + "/sys/users/del", Method: "POST", Name: "删除用户", Description: "", APIGroup: "用户模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 8, APIPath: config.Config.Sys.BaseUri + "/sys/users/get", Method: "GET", Name: "获取用户信息", Description: "", APIGroup: "用户模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 9, APIPath: config.Config.Sys.BaseUri + "/sys/users/list", Method: "GET", Name: "分页查询用户", Description: "分页查询用户", APIGroup: "用户模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 10, APIPath: config.Config.Sys.BaseUri + "/sys/users/logout", Method: "POST", Name: "退出登录", Description: "", APIGroup: "用户模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 11, APIPath: config.Config.Sys.BaseUri + "/sys/users/pwd", Method: "POST", Name: "修改密码", Description: "", APIGroup: "用户模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 12, APIPath: config.Config.Sys.BaseUri + "/sys/users/status", Method: "POST", Name: "修改用户状态", Description: "", APIGroup: "用户模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 13, APIPath: config.Config.Sys.BaseUri + "/token", Method: "POST", Name: "JWT Token验证", Description: "token 验证", APIGroup: "用户模块", Enabled: 1, NeedLog: 1, NeedAuth: 0})
	apiList = append(apiList, &sys.SysApi{ID: 14, APIPath: config.Config.Sys.BaseUri + "/sys/api/add", Method: "POST", Name: "新增API", Description: "管理员添加一个新API", APIGroup: "API模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 15, APIPath: config.Config.Sys.BaseUri + "/sys/api/update", Method: "POST", Name: "修改API信息", Description: "", APIGroup: "API模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 16, APIPath: config.Config.Sys.BaseUri + "/sys/api/del", Method: "POST", Name: "删除API", Description: "", APIGroup: "API模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 17, APIPath: config.Config.Sys.BaseUri + "/sys/api/get", Method: "GET", Name: "获取API信息", Description: "", APIGroup: "API模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 18, APIPath: config.Config.Sys.BaseUri + "/sys/api/get/uri", Method: "GET", Name: "按URI路径获取API信息", Description: "", APIGroup: "API模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 19, APIPath: config.Config.Sys.BaseUri + "/sys/api/list", Method: "GET", Name: "分页查询API接口", Description: "分页查询API", APIGroup: "API模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 20, APIPath: config.Config.Sys.BaseUri + "/sys/dept/add", Method: "POST", Name: "新增部门", Description: "管理员添加一个新部门", APIGroup: "部门模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 21, APIPath: config.Config.Sys.BaseUri + "/sys/dept/update", Method: "POST", Name: "修改部门信息", Description: "", APIGroup: "部门模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 22, APIPath: config.Config.Sys.BaseUri + "/sys/dept/del", Method: "POST", Name: "删除部门", Description: "", APIGroup: "部门模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 23, APIPath: config.Config.Sys.BaseUri + "/sys/dept/get", Method: "GET", Name: "获取部门信息", Description: "", APIGroup: "部门模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 24, APIPath: config.Config.Sys.BaseUri + "/sys/dept/tree", Method: "GET", Name: "获取部门树", Description: "", APIGroup: "部门模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 25, APIPath: config.Config.Sys.BaseUri + "/sys/dept/list", Method: "GET", Name: "分页查询部门", Description: "分页查询部门", APIGroup: "部门模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 26, APIPath: config.Config.Sys.BaseUri + "/sys/dict/add", Method: "POST", Name: "新增字典", Description: "管理员添加一个新字典", APIGroup: "字典模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 27, APIPath: config.Config.Sys.BaseUri + "/sys/dict/update", Method: "POST", Name: "修改字典信息", Description: "", APIGroup: "字典模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 28, APIPath: config.Config.Sys.BaseUri + "/sys/dict/del", Method: "POST", Name: "删除字典", Description: "", APIGroup: "字典模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 29, APIPath: config.Config.Sys.BaseUri + "/sys/dict/get", Method: "GET", Name: "获取字典信息", Description: "", APIGroup: "字典模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 30, APIPath: config.Config.Sys.BaseUri + "/sys/dict/list", Method: "GET", Name: "分页查询字典", Description: "分页查询字典", APIGroup: "字典模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 31, APIPath: config.Config.Sys.BaseUri + "/sys/role/add", Method: "POST", Name: "新增角色", Description: "管理员添加一个新角色", APIGroup: "角色模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 32, APIPath: config.Config.Sys.BaseUri + "/sys/role/update", Method: "POST", Name: "修改角色信息", Description: "", APIGroup: "角色模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 33, APIPath: config.Config.Sys.BaseUri + "/sys/role/del", Method: "POST", Name: "删除角色", Description: "", APIGroup: "角色模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 34, APIPath: config.Config.Sys.BaseUri + "/sys/role/get", Method: "GET", Name: "获取角色信息", Description: "", APIGroup: "角色模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 35, APIPath: config.Config.Sys.BaseUri + "/sys/role/list", Method: "GET", Name: "分页查询角色", Description: "分页查询角色", APIGroup: "角色模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 36, APIPath: config.Config.Sys.BaseUri + "/sys/user/ext/add", Method: "POST", Name: "新增用户扩展属性", Description: "管理员添加一个新用户扩展属性", APIGroup: "用户扩展属性模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 37, APIPath: config.Config.Sys.BaseUri + "/sys/user/ext/update", Method: "POST", Name: "修改用户扩展属性信息", Description: "", APIGroup: "用户扩展属性模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 38, APIPath: config.Config.Sys.BaseUri + "/sys/user/ext/del", Method: "POST", Name: "删除用户扩展属性", Description: "", APIGroup: "用户扩展属性模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 39, APIPath: config.Config.Sys.BaseUri + "/sys/user/ext/get", Method: "GET", Name: "获取用户扩展属性信息", Description: "", APIGroup: "用户扩展属性模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 40, APIPath: config.Config.Sys.BaseUri + "/sys/user/ext/list", Method: "GET", Name: "分页查询用户扩展属性", Description: "分页查询用户扩展属性", APIGroup: "用户扩展属性模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 41, APIPath: config.Config.Sys.BaseUri + "/sys/post/add", Method: "POST", Name: "新增岗位", Description: "管理员添加一个新岗位", APIGroup: "岗位模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 42, APIPath: config.Config.Sys.BaseUri + "/sys/post/update", Method: "POST", Name: "修改岗位信息", Description: "", APIGroup: "岗位模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 43, APIPath: config.Config.Sys.BaseUri + "/sys/post/del", Method: "POST", Name: "删除岗位", Description: "", APIGroup: "岗位模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 44, APIPath: config.Config.Sys.BaseUri + "/sys/post/get", Method: "GET", Name: "获取岗位信息", Description: "", APIGroup: "岗位模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 45, APIPath: config.Config.Sys.BaseUri + "/sys/post/list", Method: "GET", Name: "分页查询岗位", Description: "分页查询岗位", APIGroup: "岗位模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 46, APIPath: config.Config.Sys.BaseUri + "/sys/menu/add", Method: "POST", Name: "新增菜单", Description: "管理员添加一个新菜单", APIGroup: "菜单模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 47, APIPath: config.Config.Sys.BaseUri + "/sys/menu/update", Method: "POST", Name: "修改菜单信息", Description: "", APIGroup: "菜单模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 48, APIPath: config.Config.Sys.BaseUri + "/sys/menu/del", Method: "POST", Name: "删除菜单", Description: "", APIGroup: "菜单模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 49, APIPath: config.Config.Sys.BaseUri + "/sys/menu/get", Method: "GET", Name: "获取菜单信息", Description: "", APIGroup: "菜单模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 50, APIPath: config.Config.Sys.BaseUri + "/sys/menu/tree", Method: "GET", Name: "获取菜单树", Description: "", APIGroup: "菜单模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 51, APIPath: config.Config.Sys.BaseUri + "/sys/menu/list", Method: "GET", Name: "分页查询菜单", Description: "分页查询菜单", APIGroup: "菜单模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 52, APIPath: config.Config.Sys.BaseUri + "/sys/role_api/bind", Method: "POST", Name: "绑定角色API接口权限", Description: "批量全量绑定", APIGroup: "权限模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 53, APIPath: config.Config.Sys.BaseUri + "/sys/role_api/list", Method: "GET", Name: "获取角色API接口权限列表", Description: "", APIGroup: "权限模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 54, APIPath: config.Config.Sys.BaseUri + "/sys/role_menu/bind", Method: "POST", Name: "绑定角色菜单权限", Description: "全量绑定角色菜单", APIGroup: "权限模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 55, APIPath: config.Config.Sys.BaseUri + "/sys/role_menu/list", Method: "GET", Name: "获取角色菜单权限列表", Description: "", APIGroup: "权限模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 56, APIPath: config.Config.Sys.BaseUri + "/sys/api/group", Method: "GET", Name: "分组查询API接口", Description: "分组查询API", APIGroup: "API模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 57, APIPath: config.Config.Sys.BaseUri + "/sys/config/add", Method: "POST", Name: "新增系统配置", Description: "管理员添加一个新系统配置", APIGroup: "系统配置模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 58, APIPath: config.Config.Sys.BaseUri + "/sys/config/update", Method: "POST", Name: "修改系统配置信息", Description: "", APIGroup: "系统配置模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 59, APIPath: config.Config.Sys.BaseUri + "/sys/config/del", Method: "POST", Name: "删除系统配置", Description: "", APIGroup: "系统配置模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 60, APIPath: config.Config.Sys.BaseUri + "/sys/config/get", Method: "GET", Name: "获取系统配置信息", Description: "", APIGroup: "系统配置模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 61, APIPath: config.Config.Sys.BaseUri + "/sys/config/list", Method: "GET", Name: "分页查询系统配置", Description: "分页查询系统配置", APIGroup: "系统配置模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 62, APIPath: config.Config.Sys.BaseUri + "/sys/config/get/multi", Method: "GET", Name: "批量获取系统配置信息", Description: "", APIGroup: "系统配置模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 63, APIPath: config.Config.Sys.BaseUri + "/sys/role_api/add", Method: "POST", Name: "增量绑定角色API接口权限", Description: "批量全量绑定", APIGroup: "权限模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})
	apiList = append(apiList, &sys.SysApi{ID: 64, APIPath: config.Config.Sys.BaseUri + "/sys/role_api/remove", Method: "POST", Name: "解绑角色API接口权限", Description: "批量全量绑定", APIGroup: "权限模块", Enabled: 1, NeedLog: 1, NeedAuth: 1})

	for _, v := range apiList {
		err = SysApiDao.Create(v)
		if err != nil {
			logs.Error("create api error: {}", err.Error())
		}
	}

	// 初始化角色API权限
	roleApiList := []*sys.SysRoleApi{
		{ID: 1, RoleId: 1, ApiId: 1},
		{ID: 2, RoleId: 1, ApiId: 2},
		{ID: 3, RoleId: 1, ApiId: 3},
		{ID: 4, RoleId: 1, ApiId: 4},
		{ID: 5, RoleId: 1, ApiId: 5},
		{ID: 6, RoleId: 1, ApiId: 6},
		{ID: 7, RoleId: 1, ApiId: 7},
		{ID: 8, RoleId: 1, ApiId: 8},
		{ID: 9, RoleId: 1, ApiId: 9},
		{ID: 10, RoleId: 1, ApiId: 10},
		{ID: 11, RoleId: 1, ApiId: 11},
		{ID: 12, RoleId: 1, ApiId: 12},
		{ID: 13, RoleId: 1, ApiId: 13},
		{ID: 14, RoleId: 1, ApiId: 14},
		{ID: 15, RoleId: 1, ApiId: 15},
		{ID: 16, RoleId: 1, ApiId: 16},
		{ID: 17, RoleId: 1, ApiId: 17},
		{ID: 18, RoleId: 1, ApiId: 18},
		{ID: 19, RoleId: 1, ApiId: 19},
		{ID: 20, RoleId: 1, ApiId: 20},
		{ID: 21, RoleId: 1, ApiId: 21},
		{ID: 22, RoleId: 1, ApiId: 22},
		{ID: 23, RoleId: 1, ApiId: 23},
		{ID: 24, RoleId: 1, ApiId: 24},
		{ID: 25, RoleId: 1, ApiId: 25},
		{ID: 26, RoleId: 1, ApiId: 26},
		{ID: 27, RoleId: 1, ApiId: 27},
		{ID: 28, RoleId: 1, ApiId: 28},
		{ID: 29, RoleId: 1, ApiId: 29},
		{ID: 30, RoleId: 1, ApiId: 30},
		{ID: 31, RoleId: 1, ApiId: 31},
		{ID: 32, RoleId: 1, ApiId: 32},
		{ID: 33, RoleId: 1, ApiId: 33},
		{ID: 34, RoleId: 1, ApiId: 34},
		{ID: 35, RoleId: 1, ApiId: 35},
		{ID: 36, RoleId: 1, ApiId: 36},
		{ID: 37, RoleId: 1, ApiId: 37},
		{ID: 38, RoleId: 1, ApiId: 38},
		{ID: 39, RoleId: 1, ApiId: 39},
		{ID: 40, RoleId: 1, ApiId: 40},
		{ID: 41, RoleId: 1, ApiId: 41},
		{ID: 42, RoleId: 1, ApiId: 42},
		{ID: 43, RoleId: 1, ApiId: 43},
		{ID: 44, RoleId: 1, ApiId: 44},
		{ID: 45, RoleId: 1, ApiId: 45},
		{ID: 46, RoleId: 1, ApiId: 46},
		{ID: 47, RoleId: 1, ApiId: 47},
		{ID: 48, RoleId: 1, ApiId: 48},
		{ID: 49, RoleId: 1, ApiId: 49},
		{ID: 50, RoleId: 1, ApiId: 50},
		{ID: 51, RoleId: 1, ApiId: 51},
		{ID: 52, RoleId: 1, ApiId: 52},
		{ID: 53, RoleId: 1, ApiId: 53},
		{ID: 54, RoleId: 1, ApiId: 54},
		{ID: 55, RoleId: 1, ApiId: 55},
		{ID: 56, RoleId: 2, ApiId: 1},
		{ID: 57, RoleId: 2, ApiId: 2},
		{ID: 58, RoleId: 2, ApiId: 3},
		{ID: 59, RoleId: 2, ApiId: 4},
		{ID: 60, RoleId: 2, ApiId: 5},
		{ID: 61, RoleId: 2, ApiId: 6},
		{ID: 62, RoleId: 2, ApiId: 7},
		{ID: 63, RoleId: 2, ApiId: 8},
		{ID: 64, RoleId: 2, ApiId: 9},
		{ID: 65, RoleId: 2, ApiId: 10},
		{ID: 66, RoleId: 2, ApiId: 11},
		{ID: 67, RoleId: 2, ApiId: 12},
		{ID: 68, RoleId: 2, ApiId: 13},
		{ID: 69, RoleId: 2, ApiId: 14},
		{ID: 70, RoleId: 2, ApiId: 15},
		{ID: 71, RoleId: 2, ApiId: 16},
		{ID: 72, RoleId: 2, ApiId: 17},
		{ID: 73, RoleId: 2, ApiId: 18},
		{ID: 74, RoleId: 2, ApiId: 19},
		{ID: 75, RoleId: 2, ApiId: 20},
		{ID: 76, RoleId: 2, ApiId: 21},
		{ID: 77, RoleId: 2, ApiId: 22},
		{ID: 78, RoleId: 2, ApiId: 23},
		{ID: 79, RoleId: 2, ApiId: 24},
		{ID: 80, RoleId: 2, ApiId: 25},
		{ID: 81, RoleId: 2, ApiId: 26},
		{ID: 82, RoleId: 2, ApiId: 27},
		{ID: 83, RoleId: 2, ApiId: 28},
		{ID: 84, RoleId: 2, ApiId: 29},
		{ID: 85, RoleId: 2, ApiId: 30},
		{ID: 86, RoleId: 2, ApiId: 31},
		{ID: 87, RoleId: 2, ApiId: 32},
		{ID: 88, RoleId: 2, ApiId: 33},
		{ID: 89, RoleId: 2, ApiId: 34},
		{ID: 90, RoleId: 2, ApiId: 35},
		{ID: 91, RoleId: 2, ApiId: 36},
		{ID: 92, RoleId: 2, ApiId: 37},
		{ID: 93, RoleId: 2, ApiId: 38},
		{ID: 94, RoleId: 2, ApiId: 39},
		{ID: 95, RoleId: 2, ApiId: 40},
		{ID: 96, RoleId: 2, ApiId: 41},
		{ID: 97, RoleId: 2, ApiId: 42},
		{ID: 98, RoleId: 2, ApiId: 43},
		{ID: 99, RoleId: 2, ApiId: 44},
		{ID: 100, RoleId: 2, ApiId: 45},
		{ID: 101, RoleId: 2, ApiId: 46},
		{ID: 102, RoleId: 2, ApiId: 47},
		{ID: 103, RoleId: 2, ApiId: 48},
		{ID: 104, RoleId: 2, ApiId: 49},
		{ID: 105, RoleId: 2, ApiId: 50},
		{ID: 106, RoleId: 2, ApiId: 51},
		{ID: 107, RoleId: 2, ApiId: 52},
		{ID: 108, RoleId: 2, ApiId: 53},
		{ID: 109, RoleId: 2, ApiId: 54},
		{ID: 110, RoleId: 2, ApiId: 55},
		{ID: 111, RoleId: 3, ApiId: 1},
		{ID: 112, RoleId: 3, ApiId: 2},
		{ID: 113, RoleId: 3, ApiId: 3},
		{ID: 114, RoleId: 3, ApiId: 4},
		{ID: 115, RoleId: 3, ApiId: 6},
		{ID: 116, RoleId: 3, ApiId: 8},
		{ID: 117, RoleId: 3, ApiId: 9},
		{ID: 118, RoleId: 3, ApiId: 10},
		{ID: 119, RoleId: 3, ApiId: 11},
		{ID: 120, RoleId: 3, ApiId: 13},
		{ID: 121, RoleId: 3, ApiId: 17},
		{ID: 122, RoleId: 3, ApiId: 18},
		{ID: 123, RoleId: 3, ApiId: 19},
		{ID: 124, RoleId: 3, ApiId: 23},
		{ID: 125, RoleId: 3, ApiId: 24},
		{ID: 126, RoleId: 3, ApiId: 25},
		{ID: 127, RoleId: 3, ApiId: 26},
		{ID: 128, RoleId: 3, ApiId: 27},
		{ID: 129, RoleId: 3, ApiId: 28},
		{ID: 130, RoleId: 3, ApiId: 29},
		{ID: 131, RoleId: 3, ApiId: 30},
		{ID: 132, RoleId: 3, ApiId: 34},
		{ID: 133, RoleId: 3, ApiId: 35},
		{ID: 134, RoleId: 3, ApiId: 39},
		{ID: 135, RoleId: 3, ApiId: 40},
		{ID: 136, RoleId: 3, ApiId: 44},
		{ID: 137, RoleId: 3, ApiId: 45},
		{ID: 138, RoleId: 3, ApiId: 49},
		{ID: 139, RoleId: 3, ApiId: 50},
		{ID: 140, RoleId: 3, ApiId: 51},
		{ID: 141, RoleId: 3, ApiId: 53},
		{ID: 142, RoleId: 3, ApiId: 55},
		{ID: 143, RoleId: 1, ApiId: 56},
		{ID: 144, RoleId: 2, ApiId: 56},
		{ID: 145, RoleId: 3, ApiId: 56},
		{ID: 146, RoleId: 1, ApiId: 57},
		{ID: 147, RoleId: 1, ApiId: 58},
		{ID: 148, RoleId: 1, ApiId: 59},
		{ID: 149, RoleId: 1, ApiId: 60},
		{ID: 150, RoleId: 1, ApiId: 61},
		{ID: 151, RoleId: 1, ApiId: 62},
		{ID: 152, RoleId: 2, ApiId: 57},
		{ID: 153, RoleId: 2, ApiId: 58},
		{ID: 154, RoleId: 2, ApiId: 59},
		{ID: 155, RoleId: 2, ApiId: 60},
		{ID: 156, RoleId: 2, ApiId: 61},
		{ID: 157, RoleId: 2, ApiId: 62},
		{ID: 158, RoleId: 3, ApiId: 57},
		{ID: 159, RoleId: 3, ApiId: 58},
		{ID: 160, RoleId: 3, ApiId: 59},
		{ID: 161, RoleId: 3, ApiId: 60},
		{ID: 162, RoleId: 3, ApiId: 61},
		{ID: 163, RoleId: 3, ApiId: 62},
		{ID: 164, RoleId: 1, ApiId: 63},
		{ID: 165, RoleId: 1, ApiId: 64},
		{ID: 166, RoleId: 2, ApiId: 63},
		{ID: 167, RoleId: 2, ApiId: 64},
	}
	for _, v := range roleApiList {
		err = SysRoleApiDao.Create(v)
		if err != nil {
			logs.Error("create role api error: {}", err.Error())
		}
	}
	//清除角色接口权限缓存
	redis, _ := db.Redis.GetConnection()
	keys := redis.Keys("sys:role:api:*").Val()
	for _, key := range keys {
		redis.Del(key)
	}

	//初始化菜单资源
	var sysMenuList = []sys.SysResource{
		{ID: 1, Type: 1, MenuLevel: 0, Hidden: 0, ParentId: 0, Path: "dashboard", Name: "dashboard", Component: "view/dashboard/index.vue", Sort: 1, Title: "仪表盘", Icon: "odometer"},
		{ID: 2, Type: 1, MenuLevel: 0, Hidden: 0, ParentId: 0, Path: "about", Name: "about", Component: "view/about/index.vue", Sort: 9, Title: "关于我们", Icon: "info-filled"},
		{ID: 3, Type: 1, MenuLevel: 0, Hidden: 0, ParentId: 0, Path: "admin", Name: "superAdmin", Component: "view/superAdmin/index.vue", Sort: 3, Title: "超级管理员", Icon: "user"},
		{ID: 4, Type: 1, MenuLevel: 0, Hidden: 0, ParentId: 0, Path: "person", Name: "person", Component: "view/person/person.vue", Sort: 4, Title: "个人信息", Icon: "message"},
		{ID: 5, Type: 1, MenuLevel: 0, Hidden: 0, ParentId: 0, Path: "example", Name: "example", Component: "view/example/index.vue", Sort: 7, Title: "示例文件", Icon: "management"},
		{ID: 6, Type: 1, MenuLevel: 0, Hidden: 0, ParentId: 0, Path: "systemTools", Name: "systemTools", Component: "view/systemTools/index.vue", Sort: 5, Title: "系统工具", Icon: "tools"},
		{ID: 7, Type: 1, MenuLevel: 0, Hidden: 0, ParentId: 0, Path: "https://www.gin-vue-admin.com", Name: "https://www.gin-vue-admin.com", Component: "/", Sort: 0, Title: "官方网站", Icon: "customer-gva"},
		{ID: 8, Type: 1, MenuLevel: 0, Hidden: 0, ParentId: 0, Path: "state", Name: "state", Component: "view/system/state.vue", Sort: 8, Title: "服务器状态", Icon: "cloudy"},
		{ID: 9, Type: 1, MenuLevel: 0, Hidden: 0, ParentId: 0, Path: "plugin", Name: "plugin", Component: "view/routerHolder.vue", Sort: 6, Title: "插件系统", Icon: "cherry"},
		// superAdmin子菜单
		{ID: 10, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 3, Path: "authority", Name: "authority", Component: "view/superAdmin/authority/authority.vue", Sort: 1, Title: "角色管理", Icon: "avatar"},
		{ID: 11, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 3, Path: "menu", Name: "menu", Component: "view/superAdmin/menu/menu.vue", Sort: 2, Title: "菜单管理", Icon: "tickets", KeepAlive: 1},
		{ID: 12, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 3, Path: "api", Name: "api", Component: "view/superAdmin/api/api.vue", Sort: 3, Title: "api管理", Icon: "platform", KeepAlive: 1},
		{ID: 13, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 3, Path: "user", Name: "user", Component: "view/superAdmin/user/user.vue", Sort: 4, Title: "用户管理", Icon: "coordinate"},
		{ID: 14, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 3, Path: "dictionary", Name: "dictionary", Component: "view/superAdmin/dictionary/sysDictionary.vue", Sort: 5, Title: "字典管理", Icon: "notebook"},
		{ID: 15, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 3, Path: "operation", Name: "operation", Component: "view/superAdmin/operation/sysOperationRecord.vue", Sort: 6, Title: "操作历史", Icon: "pie-chart"},
		{ID: 16, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 3, Path: "sysParams", Name: "sysParams", Component: "view/superAdmin/params/sysParams.vue", Sort: 7, Title: "参数管理", Icon: "compass"},

		// example子菜单
		{ID: 17, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 5, Path: "upload", Name: "upload", Component: "view/example/upload/upload.vue", Sort: 5, Title: "媒体库（上传下载）", Icon: "upload"},
		{ID: 18, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 5, Path: "breakpoint", Name: "breakpoint", Component: "view/example/breakpoint/breakpoint.vue", Sort: 6, Title: "断点续传", Icon: "upload-filled"},
		{ID: 19, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 5, Path: "customer", Name: "customer", Component: "view/example/customer/customer.vue", Sort: 7, Title: "客户列表（资源示例）", Icon: "avatar"},

		// systemTools子菜单
		{ID: 20, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 6, Path: "autoCode", Name: "autoCode", Component: "view/systemTools/autoCode/index.vue", Sort: 1, Title: "代码生成器", Icon: "cpu", KeepAlive: 1},
		{ID: 21, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 6, Path: "formCreate", Name: "formCreate", Component: "view/systemTools/formCreate/index.vue", Sort: 3, Title: "表单生成器", Icon: "magic-stick", KeepAlive: 1},
		{ID: 22, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 6, Path: "system", Name: "system", Component: "view/systemTools/system/system.vue", Sort: 4, Title: "系统配置", Icon: "operation"},
		{ID: 23, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 6, Path: "autoCodeAdmin", Name: "autoCodeAdmin", Component: "view/systemTools/autoCodeAdmin/index.vue", Sort: 2, Title: "自动化代码管理", Icon: "magic-stick"},
		{ID: 24, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 6, Path: "autoCodeEdit/:id", Name: "autoCodeEdit", Component: "view/systemTools/autoCode/index.vue", Sort: 0, Title: "自动化代码-${id}", Icon: "magic-stick"},
		{ID: 25, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 6, Path: "autoPkg", Name: "autoPkg", Component: "view/systemTools/autoPkg/autoPkg.vue", Sort: 0, Title: "模板配置", Icon: "folder"},
		{ID: 26, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 6, Path: "exportTemplate", Name: "exportTemplate", Component: "view/systemTools/exportTemplate/exportTemplate.vue", Sort: 5, Title: "导出模板", Icon: "reading"},
		{ID: 27, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 6, Path: "picture", Name: "picture", Component: "view/systemTools/autoCode/picture.vue", Sort: 6, Title: "AI页面绘制", Icon: "picture-filled"},
		{ID: 28, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 6, Path: "mcpTool", Name: "mcpTool", Component: "view/systemTools/autoCode/mcp.vue", Sort: 7, Title: "Mcp Tools模板", Icon: "magnet"},
		{ID: 29, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 6, Path: "mcpTest", Name: "mcpTest", Component: "view/systemTools/autoCode/mcpTest.vue", Sort: 7, Title: "Mcp Tools测试", Icon: "partly-cloudy"},

		{ID: 30, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 9, Path: "https://plugin.gin-vue-admin.com/", Name: "https://plugin.gin-vue-admin.com/", Component: "https://plugin.gin-vue-admin.com/", Sort: 0, Title: "插件市场", Icon: "shop"},
		{ID: 31, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 9, Path: "installPlugin", Name: "installPlugin", Component: "view/systemTools/installPlugin/index.vue", Sort: 1, Title: "插件安装", Icon: "box"},
		{ID: 32, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 9, Path: "pubPlug", Name: "pubPlug", Component: "view/systemTools/pubPlug/pubPlug.vue", Sort: 3, Title: "打包插件", Icon: "files"},
		{ID: 33, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 9, Path: "plugin-email", Name: "plugin-email", Component: "plugin/email/view/index.vue", Sort: 4, Title: "邮件插件", Icon: "message"},
		{ID: 34, Type: 1, MenuLevel: 1, Hidden: 0, ParentId: 9, Path: "anInfo", Name: "anInfo", Component: "plugin/announcement/view/info.vue", Sort: 5, Title: "公告管理[示例]", Icon: "scaleToOriginal"},
	}
	// 删除管理员角色菜单
	SysRoleResourceDao.Where("role_id in (1,2,3) AND menu_id < ?", 35).Delete(new(sys.SysRoleResource))
	// 重新插入初始菜单
	for _, menu := range sysMenuList {
		err = SysResourceDao.Create(&menu)
		if err != nil {
			logs.Error("初始化菜单失败: {}", err.Error())
		}
		// 重新插入角色菜单
		err = SysRoleResourceDao.Create(&sys.SysRoleResource{ResourceId: menu.ID, RoleId: 1})
		if err != nil {
			logs.Error("插入初始菜单角色失败: {}", err.Error())
		}
		err = SysRoleResourceDao.Create(&sys.SysRoleResource{ResourceId: menu.ID, RoleId: 2})
		if err != nil {
			logs.Error("插入初始菜单角色失败: {}", err.Error())
		}
	}
	// 普通用户菜单初始化
	userRoleMenus := []*sys.SysRoleResource{
		{RoleId: 3, ResourceId: 1},
		{RoleId: 3, ResourceId: 2},
		{RoleId: 3, ResourceId: 4},
		{RoleId: 3, ResourceId: 5},
		{RoleId: 3, ResourceId: 7},
		{RoleId: 3, ResourceId: 8},
		{RoleId: 3, ResourceId: 9},
		{RoleId: 3, ResourceId: 17},
		{RoleId: 3, ResourceId: 18},
		{RoleId: 3, ResourceId: 19},
		{RoleId: 3, ResourceId: 30},
		{RoleId: 3, ResourceId: 33},
		{RoleId: 3, ResourceId: 34},
	}
	err = SysRoleResourceDao.MultiCreate(userRoleMenus)
	if err != nil {
		logs.Error("初始化普通用户菜单失败: {}", err.Error())
	}

	// 初始化casbin
	unAuthApiList, _ := SysApiDao.All(sys.SysApi{NeedAuth: 0})
	for _, api := range unAuthApiList {
		casbin.Casbin.UnAuthPath = append(casbin.Casbin.UnAuthPath, casbin.CasbinInfo{
			Path:   api.APIPath,
			Method: api.Method,
		})
	}
	// 初始化角色API接口权限，重新从数据库中读取所有角色API接口并初始化casbin
	roleApis, _ := SysRoleApiDao.All(sys.SysRoleApi{})
	casbinApiRules := make(map[uint][]casbin.CasbinInfo)
	for _, v := range roleApis {
		// 初始化角色API接口权限缓存
		if config.Config.Sys.Casbin {
			if casbinInfos, ok := casbinApiRules[v.RoleId]; !ok {
				casbinInfos = make([]casbin.CasbinInfo, 0)
				casbinApiRules[v.RoleId] = casbinInfos
			}
			api, _ := SysApiDao.One(sys.SysApi{ID: v.ApiId})
			casbinApiRules[v.RoleId] = append(casbinApiRules[v.RoleId], casbin.CasbinInfo{Path: api.APIPath, Method: api.Method})
		}
	}
	if config.Config.Sys.Casbin {
		for roleId, casbinInfos := range casbinApiRules {
			casbin.Casbin.UpdateCasbin(roleId, casbinInfos)
		}
		err := casbin.Casbin.GetEnforcer().LoadPolicy()
		if err != nil {
			logs.Error("load casbin policy error: {}", err.Error())
			return
		}
	}
}
