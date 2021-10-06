package controller

import (
	"github.com/maczh/gintool/mgresult"
	"github.com/maczh/mgin/service"
	"github.com/maczh/utils"
	"strconv"
)

// SaveUserMongo	godoc
// @Summary		保存用户信息
// @Description	保存用户信息
// @Tags	mongodb
// @Accept	x-www-form-urlencoded
// @Produce json
// @Param	mobile formData string true "手机号"
// @Param	name formData string true "用户名"
// @Param	age formData int true "年龄"
// @Success 200 {string} string	"ok"
// @Router	/user/mongo/save [post][get]
func SaveUserMongo(params map[string]string) mgresult.Result {
	if !utils.Exists(params, "mobile") {
		return *mgresult.Error(-1, "手机号不可为空")
	}
	age, _ := strconv.Atoi(params["age"])
	return service.SaveUserMongo(params["mobile"], params["name"], age)
}

// UpdateUserMongo	godoc
// @Summary		更新用户信息
// @Description	更新用户信息
// @Tags	mongodb
// @Accept	x-www-form-urlencoded
// @Produce json
// @Param	mobile formData string true "手机号"
// @Param	name formData string false "用户名"
// @Param	age formData int false "年龄"
// @Success 200 {string} string	"ok"
// @Router	/user/mongo/update [post][get]
func UpdateUserMongo(params map[string]string) mgresult.Result {
	if !utils.Exists(params, "mobile") {
		return *mgresult.Error(-1, "手机号不可为空")
	}
	age, _ := strconv.Atoi(params["age"])
	return service.UpdateUserMongo(params["mobile"], params["name"], age)
}

// GetUserMongoByMobile	godoc
// @Summary		按手机号查询用户信息
// @Description	按手机号查询用户信息
// @Tags	mongodb
// @Accept	x-www-form-urlencoded
// @Produce json
// @Param	mobile formData string true "手机号"
// @Success 200 {string} string	"ok"
// @Router	/user/mongo/get [post][get]
func GetUserMongoByMobile(params map[string]string) mgresult.Result {
	if !utils.Exists(params, "mobile") {
		return *mgresult.Error(-1, "手机号不可为空")
	}
	return service.GetUserMongoByMobile(params["mobile"])
}
