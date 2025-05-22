package dao

import (
	"github.com/maczh/mgin/db/dao"
	"github.com/maczh/mgin/models/sys"
)

var (
	SysUserDao     = dao.MySQLDao[sys.SysUser]{}
	SysRoleDao     = dao.MySQLDao[sys.SysRole]{}
	SysMenuDao     = dao.MySQLDao[sys.SysMenu]{}
	SysApiDao      = dao.MySQLDao[sys.SysApi]{}
	SysDeptDao     = dao.MySQLDao[sys.SysDept]{}
	SysDictDao     = dao.MySQLDao[sys.SysDict]{}
	SysPostDao     = dao.MySQLDao[sys.SysPost]{}
	SysRoleApiDao  = dao.MySQLDao[sys.SysRoleApi]{}
	SysRoleMenuDao = dao.MySQLDao[sys.SysRoleMenu]{}
	SysUserExtDao  = dao.MySQLDao[sys.SysUserExt]{}
	SysConfigDao   = dao.MySQLDao[sys.SysConfig]{}
)
