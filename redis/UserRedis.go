package redis

import (
	"errors"
	"github.com/maczh/logs"
	"github.com/maczh/mgconfig"
	"github.com/maczh/mgin/model"
	"github.com/maczh/utils"
	"strconv"
)

const (
	REDIS_KEY_USER        = "user:id:"
	REDIS_KEY_USER_MOBILE = "user:mobile:"
)

func SaveUser(user model.UserMysql) error {
	redis := mgconfig.GetRedisConnection()
	defer mgconfig.ReturnRedisConnection(redis)
	if redis == nil {
		logs.Error("Redis连接异常")
		return errors.New("Redis连接异常")
	}
	err := redis.Set(REDIS_KEY_USER+strconv.Itoa(user.Id), utils.ToJSON(user), 0).Err()
	if err != nil {
		return err
	}
	err = redis.Set(REDIS_KEY_USER_MOBILE+user.Mobile, utils.ToJSON(user), 0).Err()
	return err
}

func UpdateUser(user model.UserMysql) error {
	return SaveUser(user)
}

func GetUserById(id int) (model.UserMysql, error) {
	var user model.UserMysql
	redis := mgconfig.GetRedisConnection()
	defer mgconfig.ReturnRedisConnection(redis)
	if redis == nil {
		logs.Error("Redis连接异常")
		return user, errors.New("Redis连接异常")
	}
	userJson := redis.Get(REDIS_KEY_USER + strconv.Itoa(id)).Val()
	if userJson == "" {
		return user, errors.New("无数据")
	}
	utils.FromJSON(userJson, &user)
	return user, nil
}

func GetUserByMobile(mobile string) (model.UserMysql, error) {
	var user model.UserMysql
	redis := mgconfig.GetRedisConnection()
	defer mgconfig.ReturnRedisConnection(redis)
	if redis == nil {
		logs.Error("Redis连接异常")
		return user, errors.New("Redis连接异常")
	}
	userJson := redis.Get(REDIS_KEY_USER_MOBILE + mobile).Val()
	if userJson == "" {
		return user, errors.New("无数据")
	}
	utils.FromJSON(userJson, &user)
	return user, nil
}
