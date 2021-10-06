package service

import (
	"github.com/maczh/gintool/mgresult"
	"github.com/maczh/mgin/mongo"
)

func SaveUserMongo(mobile, name string, age int) mgresult.Result {
	user, err := mongo.GetUserByMobile(mobile)
	if err != nil {
		return *mgresult.Error(1001, err.Error())
	}
	if user.Mobile != "" {
		return *mgresult.Error(1002, "手机号重复")
	}
	user.Name = name
	user.Age = age
	user.Mobile = mobile
	user, err = mongo.InsertUser(user)
	if err != nil {
		return *mgresult.Error(1001, err.Error())
	}
	return *mgresult.Success(user)
}

func UpdateUserMongo(mobile, name string, age int) mgresult.Result {
	user, err := mongo.GetUserByMobile(mobile)
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
	err = mongo.UpdateUser(user)
	if err != nil {
		return *mgresult.Error(1001, err.Error())
	}
	return *mgresult.Success(user)
}

func GetUserMongoByMobile(mobile string) mgresult.Result {
	user, err := mongo.GetUserByMobile(mobile)
	if err != nil {
		return *mgresult.Error(1001, err.Error())
	}
	if user.Mobile == "" {
		return *mgresult.Error(1003, "手机号不存在")
	}
	return *mgresult.Success(user)
}
