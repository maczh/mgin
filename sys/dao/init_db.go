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
		&sys.SysMenu{},
		&sys.SysApi{},
		&sys.SysDept{},
		&sys.SysDict{},
		&sys.SysPost{},
		&sys.SysRoleApi{},
		&sys.SysRoleMenu{},
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
	admin := sys.SysUser{Id: 1, LoginName: "admin", Password: utils.MD5Encode("admin"), Sex: 1, Status: 1, Email: "admin@mgin.org", Mobile: "13800138000", NickName: "超级管理员"}
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
		{Id: 1, Name: "行政部", ParentId: 0, Sort: 1, Status: 1},
		{Id: 2, Name: "技术部", ParentId: 0, Sort: 2, Status: 1},
		{Id: 3, Name: "市场部", ParentId: 0, Sort: 3, Status: 1},
		{Id: 4, Name: "销售部", ParentId: 0, Sort: 4, Status: 1},
		{Id: 5, Name: "客服部", ParentId: 0, Sort: 5, Status: 1},
		{Id: 6, Name: "运营部", ParentId: 0, Sort: 6, Status: 1},
		{Id: 7, Name: "财务部", ParentId: 0, Sort: 7, Status: 1},
		{Id: 8, Name: "人事部", ParentId: 0, Sort: 8, Status: 1},
	}
	for _, v := range depts {
		err = SysDeptDao.Create(v)
		if err != nil {
			logs.Error("create dept error: {}", err.Error())
		}
	}

	//初始化岗位
	posts := []*sys.SysPost{
		{Id: 1, PostName: "总经理", PostCode: "001", Sort: 1, Status: 1},
		{Id: 2, PostName: "副总经理", PostCode: "002", Sort: 2, Status: 1},
		{Id: 3, PostName: "部门经理", PostCode: "003", Sort: 3, Status: 1},
		{Id: 4, PostName: "职员", PostCode: "004", Sort: 4, Status: 1},
	}
	for _, v := range posts {
		err = SysPostDao.Create(v)
		if err != nil {
			logs.Error("create post error: {}", err.Error())
		}
	}
	// 初始化用户扩展属性
	userExt := sys.SysUserExt{Id: 1, UserId: 1, DepartmentId: 2, PositionId: 2, RoleId: 1}
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

	for _, v := range apiList {
		err = SysApiDao.Create(v)
		if err != nil {
			logs.Error("create api error: {}", err.Error())
		}
		if v.NeedAuth == 0 {
			casbin.Casbin.UnAuthPath = append(casbin.Casbin.UnAuthPath, casbin.CasbinInfo{
				Path:   v.APIPath,
				Method: v.Method,
			})
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
	}
	casbinApiRules := make(map[uint][]casbin.CasbinInfo)
	for _, v := range roleApiList {
		err = SysRoleApiDao.Create(v)
		if err != nil {
			logs.Error("create role api error: {}", err.Error())
		}
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
	//清除角色接口权限缓存
	redis, _ := db.Redis.GetConnection()
	keys := redis.Keys("sys:role:api:*").Val()
	for _, key := range keys {
		redis.Del(key)
	}

	//初始化菜单资源
	var sysMenuList = []sys.SysMenu{
		{ID: 1, Path: "system", Name: "", Component: "", Icon: "system", Title: "系统管理", Sort: 0},
		{ID: 2, Path: "menu", Name: "", Component: "system/menu/list", Icon: "nested", Title: "菜单管理", Sort: 0},
		{ID: 3, Path: "role", Name: "", Component: "system/role/list", Icon: "peoples", Title: "系统配置管理", Sort: 0},
		{ID: 4, Path: "user", Name: "", Component: "system/user/list", Icon: "user", Title: "用户管理", Sort: 0},
		{ID: 5, Path: "information", Name: "", Component: "system/information/list", Icon: "color", Title: "平台定制", Sort: 0},
		{ID: 6, Path: "log", Name: "", Component: "", Icon: "log", Title: "日志记录", Sort: 0},
		{ID: 7, Path: "logLogin", Name: "", Component: "system/log/logLogin/list", Icon: "", Title: "登录日志", Sort: 0},
		{ID: 8, Path: "operate", Name: "", Component: "system/log/operate/list", Icon: "", Title: "操作日志", Sort: 0},
		{ID: 9, Path: "exception", Name: "", Component: "system/log/exception/list", Icon: "", Title: "异常日志", Sort: 0},
		{ID: 10, Path: "dict", Name: "", Component: "system/dict/list", Icon: "dict", Title: "参数设置", Sort: 0},
		{ID: 11, Path: "iot", Name: "", Component: "", Icon: "monitor", Title: "物联网中台", Sort: 0},
		{ID: 12, Path: "factory", Name: "", Component: "iot/factory/list", Icon: "", Title: "厂家管理", Sort: 0},
		{ID: 13, Path: "product", Name: "", Component: "iot/product/list", Icon: "", Title: "产品管理", Sort: 0},
		{ID: 14, Path: "category", Name: "", Component: "iot/category/list", Icon: "", Title: "品类管理", Sort: 0},
		{ID: 15, Path: "equipment", Name: "", Component: "iot/equipment/list", Icon: "", Title: "设备管理", Sort: 0},
		{ID: 16, Path: "monitor", Name: "", Component: "", Icon: "build", Title: "系统监控", Sort: 0},
		{ID: 17, Path: "redis", Name: "", Component: "system/monitor/redis/list", Icon: "chart", Title: "redis监控", Sort: 0},
		{ID: 18, Path: "jvm", Name: "", Component: "system/monitor/jvm/list", Icon: "swagger", Title: "jvm信息", Sort: 0},
		{ID: 19, Path: "server", Name: "", Component: "system/monitor/server/list", Icon: "chart", Title: "服务器信息", Sort: 0},
		{ID: 20, Path: "tomcat", Name: "", Component: "system/monitor/tomcat/list", Icon: "build", Title: "tomcat信息", Sort: 0},
		{ID: 21, Path: "organization", Name: "", Component: "system/organization/list", Icon: "cascader", Title: "组织管理", Sort: 0},
		{ID: 22, Path: "instrument", Name: "", Component: "iot/instrument/list", Icon: "", Title: "仪表盘", Sort: 0},
		{ID: 23, Path: "", Name: "", Component: "", Icon: "", Title: "菜单新增/编辑", Sort: 0},
		{ID: 24, Path: "", Name: "", Component: "", Icon: "", Title: "菜单删除", Sort: 0},
		{ID: 25, Path: "protocol", Name: "", Component: "iot/protocol/list", Icon: "", Title: "协议管理", Sort: 0},
		{ID: 26, Path: "variable", Name: "", Component: "iot/variable/list", Icon: "", Title: "变量库管理", Sort: 0},
		{ID: 27, Path: "data", Name: "", Component: "", Icon: "server", Title: "数据报表", Sort: 0},
		{ID: 28, Path: "history", Name: "", Component: "data/history/list", Icon: "", Title: "历史数据", Sort: 0},
		{ID: 29, Path: "real", Name: "", Component: "data/real/list", Icon: "", Title: "实时数据", Sort: 0},
		{ID: 30, Path: "http://192.168.110.208:6377/iot-xxl-job/jobinfo", Name: "", Component: "", Icon: "job", Title: "定时任务(外链)", Sort: 0},
		{ID: 31, Path: "", Name: "", Component: "", Icon: "", Title: "用户新增/编辑", Sort: 0},
		{ID: 32, Path: "", Name: "", Component: "", Icon: "", Title: "用户删除", Sort: 0},
		{ID: 33, Path: "", Name: "", Component: "", Icon: "", Title: "用户查询", Sort: 0},
		{ID: 34, Path: "", Name: "", Component: "", Icon: "", Title: "菜单查询", Sort: 0},
		{ID: 35, Path: "", Name: "", Component: "", Icon: "", Title: "角色查询", Sort: 0},
		{ID: 36, Path: "", Name: "", Component: "", Icon: "", Title: "角色新增/编辑", Sort: 0},
		{ID: 37, Path: "", Name: "", Component: "", Icon: "", Title: "角色删除", Sort: 0},
		{ID: 38, Path: "", Name: "", Component: "", Icon: "", Title: "平台定制查询", Sort: 0},
		{ID: 39, Path: "", Name: "", Component: "", Icon: "", Title: "平台定制新增/编辑", Sort: 0},
		{ID: 40, Path: "", Name: "", Component: "", Icon: "", Title: "平台定制删除", Sort: 0},
		{ID: 41, Path: "", Name: "", Component: "", Icon: "", Title: "登录日志查询", Sort: 0},
		{ID: 42, Path: "", Name: "", Component: "", Icon: "", Title: "操作日志查询", Sort: 0},
		{ID: 43, Path: "", Name: "", Component: "", Icon: "", Title: "异常日志查询", Sort: 0},
		{ID: 44, Path: "", Name: "", Component: "", Icon: "", Title: "参数设置查询", Sort: 0},
		{ID: 45, Path: "", Name: "", Component: "", Icon: "", Title: "参数设置新增/编辑", Sort: 0},
		{ID: 46, Path: "", Name: "", Component: "", Icon: "", Title: "参数设置删除", Sort: 0},
		{ID: 47, Path: "", Name: "", Component: "", Icon: "", Title: "组织查询", Sort: 0},
		{ID: 48, Path: "", Name: "", Component: "", Icon: "", Title: "组织新增/编辑", Sort: 0},
		{ID: 49, Path: "", Name: "", Component: "", Icon: "", Title: "组织删除", Sort: 0},
		{ID: 50, Path: "", Name: "", Component: "", Icon: "", Title: "参数设置刷新", Sort: 0},
		{ID: 51, Path: "", Name: "", Component: "", Icon: "", Title: "角色菜单授权", Sort: 0},
		{ID: 52, Path: "", Name: "", Component: "", Icon: "", Title: "查看用户", Sort: 0},
		{ID: 53, Path: "", Name: "", Component: "", Icon: "", Title: "展示全部组织", Sort: 0},
		{ID: 54, Path: "gateway", Name: "", Component: "iot/gateway/list", Icon: "", Title: "通信设备管理", Sort: 0},
		{ID: 55, Path: "", Name: "", Component: "", Icon: "", Title: "菜单保存", Sort: 0},
		{ID: 56, Path: "", Name: "", Component: "", Icon: "", Title: "用户保存", Sort: 0},
		{ID: 57, Path: "", Name: "", Component: "", Icon: "", Title: "角色保存", Sort: 0},
		{ID: 58, Path: "", Name: "", Component: "", Icon: "", Title: "平台定制保存", Sort: 0},
		{ID: 59, Path: "", Name: "", Component: "", Icon: "", Title: "参数设置保存", Sort: 0},
		{ID: 60, Path: "", Name: "", Component: "", Icon: "", Title: "组织保存", Sort: 0},
		{ID: 61, Path: "", Name: "", Component: "", Icon: "", Title: "角色菜单授权保存", Sort: 0},
		{ID: 62, Path: "http://192.168.110.208:6565/api/druid/index.html", Name: "", Component: "", Icon: "chart", Title: "SQL监控", Sort: 0},
		{ID: 63, Path: "", Name: "", Component: "", Icon: "", Title: "重置密码", Sort: 0},
		{ID: 64, Path: "group", Name: "", Component: "system/monitor/job/group/list", Icon: "", Title: "执行器管理", Sort: 0},
		{ID: 65, Path: "info", Name: "", Component: "system/monitor/job/info/list", Icon: "", Title: "任务管理", Sort: 0},
		{ID: 66, Path: "httpTrace", Name: "", Component: "system/monitor/httpTrace/list", Icon: "chart", Title: "请求追踪", Sort: 0},
		{ID: 67, Path: "storage", Name: "", Component: "data/storage/list", Icon: "", Title: "存储策略", Sort: 0},
		{ID: 68, Path: "announcement", Name: "", Component: "system/announcement/list", Icon: "message", Title: "系统通告", Sort: 0},
		{ID: 69, Path: "announcementSend", Name: "", Component: "system/announcementSend/list", Icon: "message", Title: "我的信息", Sort: 0},
		{ID: 70, Path: "warnLevel", Name: "", Component: "iot/warnLevel/list", Icon: "", Title: "告警等级管理", Sort: 0},
		{ID: 71, Path: "warnPolicy", Name: "", Component: "iot/warnPolicy/list", Icon: "", Title: "告警策略", Sort: 0},
		{ID: 72, Path: "warnInfo", Name: "", Component: "data/warnInfo/list", Icon: "", Title: "告警信息", Sort: 0},
		{ID: 73, Path: "", Name: "", Component: "", Icon: "", Title: "我的信息查询", Sort: 0},
		{ID: 74, Path: "", Name: "", Component: "", Icon: "", Title: "我的信息查看", Sort: 0},
		{ID: 75, Path: "", Name: "", Component: "", Icon: "", Title: "我的信息已阅", Sort: 0},
		{ID: 76, Path: "", Name: "", Component: "", Icon: "", Title: "我的信息删除", Sort: 0},
		{ID: 77, Path: "", Name: "", Component: "", Icon: "", Title: "系统通告查询", Sort: 0},
		{ID: 78, Path: "", Name: "", Component: "", Icon: "", Title: "系统通告新增/编辑", Sort: 0},
		{ID: 79, Path: "", Name: "", Component: "", Icon: "", Title: "系统通告保存", Sort: 0},
		{ID: 80, Path: "", Name: "", Component: "", Icon: "", Title: "系统通告发布/撤销", Sort: 0},
		{ID: 81, Path: "", Name: "", Component: "", Icon: "", Title: "系统通告删除", Sort: 0},
		{ID: 82, Path: "", Name: "", Component: "", Icon: "", Title: "厂家管理查询", Sort: 0},
		{ID: 83, Path: "", Name: "", Component: "", Icon: "", Title: "厂家管理新增、编辑", Sort: 0},
		{ID: 84, Path: "", Name: "", Component: "", Icon: "", Title: "厂家管理保存", Sort: 0},
		{ID: 85, Path: "", Name: "", Component: "", Icon: "", Title: "厂家管理删除", Sort: 0},
		{ID: 86, Path: "", Name: "", Component: "", Icon: "", Title: "通信设备查询", Sort: 0},
		{ID: 87, Path: "", Name: "", Component: "", Icon: "", Title: "通信设备新增/编辑", Sort: 0},
		{ID: 88, Path: "", Name: "", Component: "", Icon: "", Title: "通信设备保存", Sort: 0},
		{ID: 89, Path: "", Name: "", Component: "", Icon: "", Title: "通信设备删除", Sort: 0},
		{ID: 90, Path: "cameraMonitor", Name: "", Component: "camera/cameraMonitor/list", Icon: "", Title: "视频监控", Sort: 0},
		{ID: 91, Path: "camera", Name: "", Component: "", Icon: "table", Title: "视频汇聚中台", Sort: 0},
		{ID: 92, Path: "cameraManage", Name: "", Component: "camera/cameraManage/list", Icon: "", Title: "视频管理", Sort: 0},
		{ID: 93, Path: "config", Name: "", Component: "system/config/list", Icon: "", Title: "系统配置", Sort: 0},
		{ID: 94, Path: "warnSendRecord", Name: "", Component: "iot/warnSendRecord/list", Icon: "", Title: "告警推送记录（预留）", Sort: 0},
		{ID: 95, Path: "formworkVariable", Name: "", Component: "iot/formworkVariable/list", Icon: "", Title: "变量库模板管理", Sort: 0},
		{ID: 96, Path: "messageData", Name: "", Component: "data/messageData/list", Icon: "", Title: "报文日志", Sort: 0},
		{ID: 97, Path: "", Name: "", Component: "", Icon: "", Title: "统计", Sort: 0},
		{ID: 98, Path: "", Name: "", Component: "", Icon: "", Title: "产品查询", Sort: 0},
		{ID: 99, Path: "", Name: "", Component: "", Icon: "", Title: "产品新增/编辑", Sort: 0},
		{ID: 100, Path: "", Name: "", Component: "", Icon: "", Title: "产品保存", Sort: 0},
		{ID: 101, Path: "", Name: "", Component: "", Icon: "", Title: "产品删除", Sort: 0},
		{ID: 102, Path: "", Name: "", Component: "", Icon: "", Title: "设备查询", Sort: 0},
		{ID: 103, Path: "", Name: "", Component: "", Icon: "", Title: "设备新增/编辑", Sort: 0},
		{ID: 104, Path: "", Name: "", Component: "", Icon: "", Title: "设备保存", Sort: 0},
		{ID: 105, Path: "", Name: "", Component: "", Icon: "", Title: "设备删除", Sort: 0},
		{ID: 106, Path: "", Name: "", Component: "", Icon: "", Title: "品类查询", Sort: 0},
		{ID: 107, Path: "", Name: "", Component: "", Icon: "", Title: "品类新增/修改", Sort: 0},
		{ID: 108, Path: "", Name: "", Component: "", Icon: "", Title: "品类删除", Sort: 0},
		{ID: 109, Path: "", Name: "", Component: "", Icon: "", Title: "协议查询", Sort: 0},
		{ID: 110, Path: "", Name: "", Component: "", Icon: "", Title: "协议新增/编辑", Sort: 0},
		{ID: 111, Path: "", Name: "", Component: "", Icon: "", Title: "协议保存", Sort: 0},
		{ID: 112, Path: "", Name: "", Component: "", Icon: "", Title: "协议删除", Sort: 0},
		{ID: 113, Path: "", Name: "", Component: "", Icon: "", Title: "告警等级查询", Sort: 0},
		{ID: 114, Path: "", Name: "", Component: "", Icon: "", Title: "告警等级新增/修改", Sort: 0},
		{ID: 115, Path: "", Name: "", Component: "", Icon: "", Title: "告警等级保存", Sort: 0},
		{ID: 116, Path: "", Name: "", Component: "", Icon: "", Title: "告警等级删除", Sort: 0},
		{ID: 117, Path: "", Name: "", Component: "", Icon: "", Title: "变量库查询", Sort: 0},
		{ID: 118, Path: "", Name: "", Component: "", Icon: "", Title: "变量库新增/编辑", Sort: 0},
		{ID: 119, Path: "", Name: "", Component: "", Icon: "", Title: "变量库保存", Sort: 0},
		{ID: 120, Path: "", Name: "", Component: "", Icon: "", Title: "变量库删除", Sort: 0},
		{ID: 121, Path: "", Name: "", Component: "", Icon: "", Title: "变量库模板查询", Sort: 0},
		{ID: 122, Path: "", Name: "", Component: "", Icon: "", Title: "变量库模板新增/修改", Sort: 0},
		{ID: 123, Path: "", Name: "", Component: "", Icon: "", Title: "变量库模板保存", Sort: 0},
		{ID: 124, Path: "", Name: "", Component: "", Icon: "", Title: "变量库模板删除", Sort: 0},
		{ID: 125, Path: "", Name: "", Component: "", Icon: "", Title: "存储策略查询", Sort: 0},
		{ID: 126, Path: "", Name: "", Component: "", Icon: "", Title: "存储策略新增/修改", Sort: 0},
		{ID: 127, Path: "", Name: "", Component: "", Icon: "", Title: "存储策略保存", Sort: 0},
		{ID: 128, Path: "", Name: "", Component: "", Icon: "", Title: "存储策略删除", Sort: 0},
		{ID: 129, Path: "", Name: "", Component: "", Icon: "", Title: "实时数据查询", Sort: 0},
		{ID: 130, Path: "", Name: "", Component: "", Icon: "", Title: "告警策略查询", Sort: 0},
		{ID: 131, Path: "", Name: "", Component: "", Icon: "", Title: "告警策略新增/编辑", Sort: 0},
		{ID: 132, Path: "", Name: "", Component: "", Icon: "", Title: "告警策略保存", Sort: 0},
		{ID: 133, Path: "", Name: "", Component: "", Icon: "", Title: "告警策略删除", Sort: 0},
		{ID: 134, Path: "", Name: "", Component: "", Icon: "", Title: "历史数据查询", Sort: 0},
		{ID: 135, Path: "", Name: "", Component: "", Icon: "", Title: "告警信息查询", Sort: 0},
		{ID: 136, Path: "", Name: "", Component: "", Icon: "", Title: "摄像头查询", Sort: 0},
		{ID: 137, Path: "", Name: "", Component: "", Icon: "", Title: "摄像头新增/修改", Sort: 0},
		{ID: 138, Path: "", Name: "", Component: "", Icon: "", Title: "摄像头保存", Sort: 0},
		{ID: 139, Path: "", Name: "", Component: "", Icon: "", Title: "摄像头删除", Sort: 0},
		{ID: 140, Path: "", Name: "", Component: "", Icon: "", Title: "摄像头同步在线状态", Sort: 0},
		{ID: 141, Path: "", Name: "", Component: "", Icon: "", Title: "摄像头直播流", Sort: 0},
		{ID: 142, Path: "", Name: "", Component: "", Icon: "", Title: "摄像头回放流", Sort: 0},
		{ID: 143, Path: "", Name: "", Component: "", Icon: "", Title: "报文日志查询", Sort: 0},
		{ID: 144, Path: "", Name: "", Component: "", Icon: "", Title: "告警推送记录查询", Sort: 0},
		{ID: 145, Path: "", Name: "", Component: "", Icon: "", Title: "摄像头控制", Sort: 0},
		{ID: 146, Path: "door", Name: "", Component: "data/door/list", Icon: "", Title: "门禁记录", Sort: 0},
		{ID: 147, Path: "", Name: "", Component: "", Icon: "", Title: "告警信息处理", Sort: 0},
		{ID: 148, Path: "", Name: "", Component: "", Icon: "", Title: "告警信息保存", Sort: 0},
		{ID: 149, Path: "", Name: "", Component: "", Icon: "", Title: "任务新增/编辑/详情", Sort: 0},
		{ID: 150, Path: "", Name: "", Component: "", Icon: "", Title: "任务日志", Sort: 0},
		{ID: 151, Path: "", Name: "", Component: "", Icon: "", Title: "任务执行", Sort: 0},
		{ID: 152, Path: "", Name: "", Component: "", Icon: "", Title: "任务删除", Sort: 0},
		{ID: 153, Path: "", Name: "", Component: "", Icon: "", Title: "任务保存", Sort: 0},
		{ID: 154, Path: "", Name: "", Component: "", Icon: "", Title: "执行器新增/编辑", Sort: 0},
		{ID: 155, Path: "", Name: "", Component: "", Icon: "", Title: "任务查询", Sort: 0},
		{ID: 156, Path: "", Name: "", Component: "", Icon: "", Title: "执行器查询", Sort: 0},
		{ID: 157, Path: "", Name: "", Component: "", Icon: "", Title: "执行器删除", Sort: 0},
		{ID: 158, Path: "", Name: "", Component: "", Icon: "", Title: "执行器保存", Sort: 0},
		{ID: 159, Path: "", Name: "", Component: "", Icon: "", Title: "门禁记录查询", Sort: 0},
		{ID: 160, Path: "", Name: "", Component: "", Icon: "", Title: "品类保存", Sort: 0},
		{ID: 161, Path: "", Name: "", Component: "", Icon: "", Title: "通信设备导出", Sort: 0},
		{ID: 162, Path: "", Name: "", Component: "", Icon: "", Title: "变量库导出", Sort: 0},
		{ID: 163, Path: "", Name: "", Component: "", Icon: "", Title: "设备导出", Sort: 0},
		{ID: 164, Path: "pushSystem", Name: "", Component: "iot/pushSystem/list", Icon: "", Title: "开放平台系统推送配置", Sort: 0},
		{ID: 165, Path: "", Name: "", Component: "", Icon: "", Title: "开放平台系统配置查询", Sort: 0},
		{ID: 166, Path: "", Name: "", Component: "", Icon: "", Title: "开放平台系统配置新增/编辑", Sort: 0},
		{ID: 167, Path: "", Name: "", Component: "", Icon: "", Title: "开放平台系统配置保存", Sort: 0},
		{ID: 168, Path: "", Name: "", Component: "", Icon: "", Title: "开放平台系统配置删除", Sort: 0},
		{ID: 169, Path: "iotDoor", Name: "", Component: "iot/door/iotDoor/list", Icon: "", Title: "门禁管理", Sort: 0},
		{ID: 170, Path: "apiConfig", Name: "", Component: "iot/apiConfig/list", Icon: "", Title: "视频链路管理", Sort: 0},
		{ID: 171, Path: "doors", Name: "", Component: "", Icon: "eye", Title: "门禁中台", Sort: 0},
		{ID: 172, Path: "iotEmployee", Name: "", Component: "iot/door/iotEmployee/list", Icon: "", Title: "门禁人员管理", Sort: 0},
		{ID: 173, Path: "policyConfig", Name: "", Component: "", Icon: "config", Title: "物联网策略配置", Sort: 0},
		{ID: 174, Path: "openUrl", Name: "", Component: "iot/openUrl/list", Icon: "link", Title: "开放平台接口管理", Sort: 0},
		{ID: 175, Path: "", Name: "", Component: "", Icon: "", Title: "开放平台接口查询", Sort: 0},
		{ID: 176, Path: "", Name: "", Component: "", Icon: "", Title: "开放平台接口新增/编辑", Sort: 0},
		{ID: 177, Path: "", Name: "", Component: "", Icon: "", Title: "开放平台接口保存", Sort: 0},
		{ID: 178, Path: "", Name: "", Component: "", Icon: "", Title: "开放平台接口删除", Sort: 0},
		{ID: 179, Path: "pushSystemLog", Name: "", Component: "data/pushSystemLog/list", Icon: "", Title: "开放平台日志", Sort: 0},
		{ID: 180, Path: "deviceControl", Name: "", Component: "iot/deviceControl/list", Icon: "", Title: "设备监控详情", Sort: 0},
		{ID: 181, Path: "iotDoorProtocol", Name: "", Component: "iot/door/iotDoorProtocol/list", Icon: "", Title: "门禁协议管理", Sort: 0},
		{ID: 182, Path: "monitor2", Name: "", Component: "camera/cameraMonitor2/live", Icon: "", Title: "视频监控（国标平台）", Sort: 0},
		{ID: 183, Path: "linkPolicy", Name: "", Component: "", Icon: "", Title: "联动策略", Sort: 0},
		{ID: 184, Path: "lpconfigure", Name: "", Component: "iot/linkPolicy/list", Icon: "", Title: "联动策略配置", Sort: 0},
		{ID: 185, Path: "", Name: "", Component: "", Icon: "", Title: "联动策略配置查询", Sort: 0},
		{ID: 186, Path: "", Name: "", Component: "", Icon: "", Title: "联动策略配置新增/修改", Sort: 0},
		{ID: 187, Path: "", Name: "", Component: "", Icon: "", Title: "联动策略配置保存", Sort: 0},
		{ID: 188, Path: "", Name: "", Component: "", Icon: "", Title: "联动策略配置删除", Sort: 0},
		{ID: 189, Path: "linkLog", Name: "", Component: "iot/linkLog/list", Icon: "", Title: "联动策略日志", Sort: 0},
		{ID: 190, Path: "", Name: "", Component: "", Icon: "", Title: "联动策略日志查询", Sort: 0},
		{ID: 191, Path: "linkButton", Name: "", Component: "iot/linkButton/list", Icon: "", Title: "联动策略主题", Sort: 0},
		{ID: 192, Path: "", Name: "", Component: "", Icon: "", Title: "联动策略主题查询", Sort: 0},
		{ID: 193, Path: "", Name: "", Component: "", Icon: "", Title: "联动策略主题新增/修改", Sort: 0},
		{ID: 194, Path: "", Name: "", Component: "", Icon: "", Title: "联动策略主题保存", Sort: 0},
		{ID: 195, Path: "", Name: "", Component: "", Icon: "", Title: "联动策略主题删除", Sort: 0},
		{ID: 196, Path: "", Name: "", Component: "", Icon: "", Title: "联动策略主题执行", Sort: 0},
		{ID: 197, Path: "http://192.168.110.208:6377/iot-xxl-job/joblog", Name: "", Component: "", Icon: "", Title: "定时任务日志(外链)", Sort: 0},
		{ID: 198, Path: "loginUser", Name: "", Component: "system/loginUser/list", Icon: "", Title: "在线用户管理", Sort: 0},
		{ID: 199, Path: "pwdLog", Name: "", Component: "system/log/pwd/list", Icon: "", Title: "改密日志", Sort: 0},
		{ID: 200, Path: "Audit", Name: "", Component: "", Icon: "lock", Title: "安全审计", Sort: 0},
		{ID: 201, Path: "openPlatform", Name: "", Component: "", Icon: "international", Title: "开放平台", Sort: 0},
		{ID: 202, Path: "iotCard", Name: "", Component: "", Icon: "documentation", Title: "物联卡", Sort: 0},
		{ID: 203, Path: "tool", Name: "", Component: "", Icon: "", Title: "系统工具", Sort: 0},
		{ID: 204, Path: "redis", Name: "", Component: "system/tool/redis/list", Icon: "", Title: "redis工具", Sort: 0},
		{ID: 205, Path: "log4j2", Name: "", Component: "system/tool/log4j2/list", Icon: "", Title: "实时日志工具", Sort: 0},
		{ID: 206, Path: "timPolicy", Name: "", Component: "", Icon: "", Title: "定时策略", Sort: 0},
		{ID: 207, Path: "TpConfig", Name: "", Component: "iot/timPolicy/list", Icon: "", Title: "定时策略配置", Sort: 0},
		{ID: 208, Path: "timLog", Name: "", Component: "iot/timLog/list", Icon: "", Title: "定时策略日志", Sort: 0},
		{ID: 209, Path: "noticePolicy", Name: "", Component: "", Icon: "", Title: "通知策略", Sort: 0},
		{ID: 210, Path: "", Name: "", Component: "", Icon: "", Title: "通知策略查询", Sort: 0},
		{ID: 211, Path: "", Name: "", Component: "", Icon: "", Title: "通知策略新增/编辑", Sort: 0},
		{ID: 212, Path: "", Name: "", Component: "", Icon: "", Title: "通知策略保存", Sort: 0},
		{ID: 213, Path: "", Name: "", Component: "", Icon: "", Title: "通知策略删除", Sort: 0},
		{ID: 214, Path: "viewCenter", Name: "", Component: "", Icon: "eye-open", Title: "视图中心", Sort: 0},
		{ID: 215, Path: "mapView", Name: "", Component: "viewCenter/mapView/list", Icon: "", Title: "地图视图", Sort: 0},
		{ID: 216, Path: "config", Name: "", Component: "iot/noticePolicy/list", Icon: "", Title: "通知策略配置", Sort: 0},
		{ID: 217, Path: "log", Name: "", Component: "iot/noticePolicyLog/list", Icon: "", Title: "通知策略日志", Sort: 0},
		{ID: 218, Path: "", Name: "", Component: "", Icon: "", Title: "通知策略日志查询", Sort: 0},
		{ID: 219, Path: "", Name: "", Component: "", Icon: "", Title: "组织管理查询", Sort: 0},
		{ID: 220, Path: "dataSource", Name: "", Component: "largeScreen/dataSource/list", Icon: "server", Title: "数据源管理", Sort: 0},
		{ID: 221, Path: "largeScreen", Name: "", Component: "", Icon: "textarea", Title: "大屏管理", Sort: 0},
		{ID: 222, Path: "component", Name: "", Component: "largeScreen/component/list", Icon: "visual", Title: "组件管理", Sort: 0},
		{ID: 223, Path: "componentCategory", Name: "", Component: "largeScreen/componentCategory/list", Icon: "redis", Title: "组件分类管理", Sort: 0},
		{ID: 224, Path: "", Name: "", Component: "", Icon: "", Title: "查询", Sort: 0},
		{ID: 225, Path: "report", Name: "", Component: "iot/report/list", Icon: "", Title: "报表管理", Sort: 0},
		{ID: 226, Path: "", Name: "", Component: "", Icon: "", Title: "查询", Sort: 0},
		{ID: 227, Path: "echarts", Name: "", Component: "iot/report/echarts", Icon: "", Title: "图表", Sort: 0},
		{ID: 228, Path: "platform", Name: "", Component: "iot/cardPlatform/list", Icon: "", Title: "平台接入", Sort: 0},
		{ID: 229, Path: "cardManage", Name: "", Component: "iot/card/list", Icon: "", Title: "物联卡管理", Sort: 0},
		{ID: 230, Path: "home", Name: "", Component: "iot/card/home", Icon: "", Title: "物联卡首页", Sort: 0},
		{ID: 231, Path: "warnType", Name: "", Component: "iot/warnType/list", Icon: "", Title: "告警类型管理", Sort: 0},
		{ID: 232, Path: "", Name: "", Component: "", Icon: "", Title: "告警类型查询", Sort: 0},
		{ID: 233, Path: "", Name: "", Component: "", Icon: "", Title: "告警类型新增/修改", Sort: 0},
		{ID: 234, Path: "", Name: "", Component: "", Icon: "", Title: "告警类型并保存", Sort: 0},
		{ID: 235, Path: "", Name: "", Component: "", Icon: "", Title: "告警类型删除", Sort: 0},
	}
	for _, menu := range sysMenuList {
		err = SysMenuDao.Create(&menu)
		if err != nil {
			logs.Error("初始化菜单失败: {}", err.Error())
		}
	}

}
