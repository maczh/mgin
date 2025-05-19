package dao

import (
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models/sys"
	"gorm.io/gorm"
)

func initDB() {
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

}
