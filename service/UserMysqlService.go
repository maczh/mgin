package service

import (
	"github.com/maczh/gintool/mgresult"
	"github.com/maczh/mgin/mysql"
)

func SaveUserMysql(mobile, name string, age int) mgresult.Result {
	user, err := mysql.GetUserByMobile(mobile)
	if err != nil {
		return *mgresult.Error(1001, err.Error())
	}
	if user.Mobile != "" {
		return *mgresult.Error(1002, "手机号重复")
	}
	user.Name = name
	user.Age = age
	user.Mobile = mobile
	user, err = mysql.SaveUser(user)
	if err != nil {
		return *mgresult.Error(1001, err.Error())
	}
	return *mgresult.Success(user)
}

func UpdateUserMysql(mobile, name string, age int) mgresult.Result {
	user, err := mysql.GetUserByMobile(mobile)
	if err != nil {
		return *mgresult.Error(1001, err.Error())
	}
	if user.Mobile == "" {
		return *mgresult.Error(1003, "手机号不存在")
	}
	if name != "" {
		user.Name = name
	}
	if age != 0 {
		user.Age = age
	}
	user,err = mysql.UpdateUserMysql(user)
	if err != nil {
		return *mgresult.Error(1001, err.Error())
	}
	return *mgresult.Success(user)
}

func GetUserMysqlByMobile(mobile string) mgresult.Result {
	user, err := mysql.GetUserByMobile(mobile)
	if err != nil {
		return *mgresult.Error(1001, err.Error())
	}
	if user.Mobile == "" {
		return *mgresult.Error(1003, "手机号不存在")
	}
	return *mgresult.Success(user)
}


func GetUserMysqlById(id int) mgresult.Result {
	user, err := mysql.GetUserById(id)
	if err != nil {
		return *mgresult.Error(1001, err.Error())
	}
	if user.Mobile == "" {
		return *mgresult.Error(1003, "用户编号不存在")
	}
	return *mgresult.Success(user)
}
