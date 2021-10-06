package service

import (
	"github.com/maczh/gintool/mgresult"
	"github.com/maczh/mgin/redis"
)

func SaveUserRedis(mobile, name string, age,id int) mgresult.Result {
	user, err := redis.GetUserByMobile(mobile)
	if err != nil {
		return *mgresult.Error(1001, err.Error())
	}
	if user.Mobile != "" {
		return *mgresult.Error(1002, "手机号重复")
	}
	user.Id = id
	user.Name = name
	user.Age = age
	user.Mobile = mobile
	err = redis.SaveUser(user)
	if err != nil {
		return *mgresult.Error(1001, err.Error())
	}
	return *mgresult.Success(user)
}

func UpdateUserRedis(mobile, name string, age,id int) mgresult.Result {
	user, err := redis.GetUserById(id)
	if err != nil {
		return *mgresult.Error(1001, err.Error())
	}
	if user.Mobile == "" {
		return *mgresult.Error(1003, "用户编号不存在")
	}
	if mobile != "" {
		user.Mobile = mobile
	}
	if name != "" {
		user.Name = name
	}
	if age != 0 {
		user.Age = age
	}
	err = redis.UpdateUser(user)
	if err != nil {
		return *mgresult.Error(1001, err.Error())
	}
	return *mgresult.Success(user)
}

func GetUserRedisByMobile(mobile string) mgresult.Result {
	user, err := redis.GetUserByMobile(mobile)
	if err != nil {
		return *mgresult.Error(1001, err.Error())
	}
	if user.Mobile == "" {
		return *mgresult.Error(1003, "手机号不存在")
	}
	return *mgresult.Success(user)
}


func GetUserRedisById(id int) mgresult.Result {
	user, err := redis.GetUserById(id)
	if err != nil {
		return *mgresult.Error(1001, err.Error())
	}
	if user.Mobile == "" {
		return *mgresult.Error(1003, "用户编号不存在")
	}
	return *mgresult.Success(user)
}
