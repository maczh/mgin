package mysql

import (
	"errors"
	"github.com/maczh/logs"
	"github.com/maczh/mgconfig"
	"github.com/maczh/mgin/model"
)

const TABLE_USER = "user"

func GetUserById(id int) (*model.UserMysql,error) {
	var user model.UserMysql
	db := mgconfig.GetMysqlConnection()
	defer mgconfig.ReturnMysqlConnection(db)
	if db == nil {
		logs.Error("MySQL数据库连接异常")
		return nil, errors.New("MySQL数据库连接异常")
	}
	db.Table(TABLE_USER).Where("id = ?", id).First(&user)
	if user.Id == 0 {
		return nil,errors.New("无数据")
	} else {
		return &user,nil
	}
}

func GetUserByMobile(mobile string) (*model.UserMysql,error) {
	var user model.UserMysql
	db := mgconfig.GetMysqlConnection()
	defer mgconfig.ReturnMysqlConnection(db)
	if db == nil {
		logs.Error("MySQL数据库连接异常")
		return nil, errors.New("MySQL数据库连接异常")
	}
	db.Table(TABLE_USER).Where("mobile = ?", mobile).First(&user)
	if user.Id == 0 {
		return nil,errors.New("无数据")
	} else {
		return &user,nil
	}
}

func SaveUser(user *model.UserMysql) (*model.UserMysql, error) {
	db := mgconfig.GetMysqlConnection()
	defer mgconfig.ReturnMysqlConnection(db)
	if db == nil {
		logs.Error("MySQL数据库连接异常")
		return user, errors.New("MySQL数据库连接异常")
	}
	tx := db.Begin()
	err := tx.Table(TABLE_USER).Create(user).Error
	if err != nil {
		logs.Error("插入数据错误:{}", err.Error())
		tx.Rollback()
		return nil, err
	}
	tx.Commit()
	return user, nil
}

func UpdateUserMysql(user *model.UserMysql) (*model.UserMysql, error) {
	db := mgconfig.GetMysqlConnection()
	defer mgconfig.ReturnMysqlConnection(db)
	if db == nil {
		logs.Error("MySQL数据库连接异常")
		return user, errors.New("MySQL数据库连接异常")
	}
	tx := db.Begin()
	err := tx.Table(TABLE_USER).Save(user).Error
	if err != nil {
		logs.Error("更新数据错误:{}", err.Error())
		tx.Rollback()
		return nil, err
	}
	tx.Commit()
	return user, nil
}
