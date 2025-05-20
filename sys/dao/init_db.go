package dao

import (
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
		{Id: 1, DeptName: "行政部", ParentId: 0, Sort: 1, Status: 1},
		{Id: 2, DeptName: "技术部", ParentId: 0, Sort: 2, Status: 1},
		{Id: 3, DeptName: "市场部", ParentId: 0, Sort: 3, Status: 1},
		{Id: 4, DeptName: "销售部", ParentId: 0, Sort: 4, Status: 1},
		{Id: 5, DeptName: "客服部", ParentId: 0, Sort: 5, Status: 1},
		{Id: 6, DeptName: "运营部", ParentId: 0, Sort: 6, Status: 1},
		{Id: 7, DeptName: "财务部", ParentId: 0, Sort: 7, Status: 1},
		{Id: 8, DeptName: "人事部", ParentId: 0, Sort: 8, Status: 1},
	}
	err = SysDeptDao.MultiCreate(depts)
	if err != nil {
		logs.Error("create depts error: {}", err.Error())
	}
	// API 列表
	apiList := make([]*sys.SysApi, 0)
	apiList = append(apiList, &sys.SysApi{ID: 1, APIPath: "/api/v1/login", Method: "POST", Name: "登录", Description: "用户登录，可以用用户名、手机号、邮箱作为登录名", APIGroup: "用户模块", NeedAuth: false})
	apiList = append(apiList, &sys.SysApi{ID: 2, APIPath: "/api/v1/register", Method: "POST", Name: "注册", Description: "用户注册", APIGroup: "用户模块", NeedAuth: false})
	apiList = append(apiList, &sys.SysApi{ID: 3, APIPath: "/api/v1/captcha/get", Method: "GET", Name: "获取图片验证码", Description: "获取图片验证码", APIGroup: "用户模块", NeedAuth: false})
	apiList = append(apiList, &sys.SysApi{ID: 4, APIPath: "/api/v1/captcha/verify", Method: "POST", Name: "图片验证码验证", Description: "图片验证码验证", APIGroup: "用户模块", NeedAuth: false})
	apiList = append(apiList, &sys.SysApi{ID: 5, APIPath: "/api/v1/users/add", Method: "POST", Name: "新增用户", Description: "管理员添加一个新用户", APIGroup: "用户模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 6, APIPath: "/api/v1/users/update", Method: "POST", Name: "修改用户信息", Description: "", APIGroup: "用户模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 7, APIPath: "/api/v1/users/del", Method: "POST", Name: "删除用户", Description: "", APIGroup: "用户模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 8, APIPath: "/api/v1/users/get", Method: "GET", Name: "获取用户信息", Description: "", APIGroup: "用户模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 9, APIPath: "/api/v1/users/list", Method: "GET", Name: "分页查询", Description: "分页查询用户", APIGroup: "用户模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 10, APIPath: "/api/v1/users/logout", Method: "POST", Name: "退出登录", Description: "", APIGroup: "用户模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 11, APIPath: "/api/v1/users/pwd", Method: "POST", Name: "修改密码", Description: "", APIGroup: "用户模块", NeedAuth: true})
	apiList = append(apiList, &sys.SysApi{ID: 12, APIPath: "/api/v1/users/status", Method: "POST", Name: "修改用户状态", Description: "", APIGroup: "用户模块", NeedAuth: true})
}
