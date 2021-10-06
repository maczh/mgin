package controller

import (
	"github.com/maczh/gintool/mgresult"
	"github.com/maczh/mgin/service"
	"github.com/maczh/utils"
	"strconv"
)

// SaveUserRedis	godoc
// @Summary		保存用户信息
// @Description	保存用户信息
// @Tags	redis
// @Accept	x-www-form-urlencoded
// @Produce json
// @Param	id formData int true "用户编号"
// @Param	mobile formData string true "手机号"
// @Param	name formData string true "用户名"
// @Param	age formData int true "年龄"
// @Success 200 {string} string	"ok"
// @Router	/user/redis/save [post][get]
func SaveUserRedis(params map[string]string) mgresult.Result {
	if !utils.Exists(params, "id") {
		return *mgresult.Error(-1, "用户编号不可为空")
	}
	if !utils.Exists(params, "mobile") {
		return *mgresult.Error(-1, "手机号不可为空")
	}
	id, _ := strconv.Atoi(params["id"])
	age, _ := strconv.Atoi(params["age"])
	return service.SaveUserRedis(params["mobile"], params["name"], age, id)
}

// UpdateUserRedis	godoc
// @Summary		更新用户信息
// @Description	更新用户信息
// @Tags	redis
// @Accept	x-www-form-urlencoded
// @Produce json
// @Param	id formData int true "用户编号"
// @Param	mobile formData string false "手机号"
// @Param	name formData string false "用户名"
// @Param	age formData int false "年龄"
// @Success 200 {string} string	"ok"
// @Router	/user/redis/update [post][get]
func UpdateUserRedis(params map[string]string) mgresult.Result {
	if !utils.Exists(params, "id") {
		return *mgresult.Error(-1, "用户编号不可为空")
	}
	id, _ := strconv.Atoi(params["id"])
	age, _ := strconv.Atoi(params["age"])
	return service.UpdateUserRedis(params["mobile"], params["name"], age, id)
}

// GetUserRedisByMobile	godoc
// @Summary		按手机号查询用户信息
// @Description	按手机号查询用户信息
// @Tags	redis
// @Accept	x-www-form-urlencoded
// @Produce json
// @Param	mobile formData string true "手机号"
// @Success 200 {string} string	"ok"
// @Router	/user/redis/get/mobile [post][get]
func GetUserRedisByMobile(params map[string]string) mgresult.Result {
	if !utils.Exists(params, "mobile") {
		return *mgresult.Error(-1, "手机号不可为空")
	}
	return service.GetUserRedisByMobile(params["mobile"])
}

// GetUserRedisById	godoc
// @Summary		按用户编号查询用户信息
// @Description	按用户编号查询用户信息
// @Tags	redis
// @Accept	x-www-form-urlencoded
// @Produce json
// @Param	id formData int true "编号编号"
// @Success 200 {string} string	"ok"
// @Router	/user/redis/get/id [post][get]
func GetUserRedisById(params map[string]string) mgresult.Result {
	if !utils.Exists(params, "id") {
		return *mgresult.Error(-1, "用户编号不可为空")
	}
	id, _ := strconv.Atoi(params["id"])
	return service.GetUserRedisById(id)
}
