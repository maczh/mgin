package controller

import (
	"github.com/maczh/gintool/mgresult"
	"github.com/maczh/mgin/service"
	"github.com/maczh/utils"
	"strconv"
)

// SaveUserMysql	godoc
// @Summary		保存用户信息
// @Description	保存用户信息
// @Tags	mysql
// @Accept	x-www-form-urlencoded
// @Produce json
// @Param	mobile formData string true "手机号"
// @Param	name formData string true "用户名"
// @Param	age formData int true "年龄"
// @Success 200 {string} string	"ok"
// @Router	/user/mysql/save [post][get]
func SaveUserMysql(params map[string]string) mgresult.Result {
	if !utils.Exists(params, "mobile") {
		return *mgresult.Error(-1, "手机号不可为空")
	}
	age, _ := strconv.Atoi(params["age"])
	return service.SaveUserMysql(params["mobile"], params["name"], age)
}

// UpdateUserMysql	godoc
// @Summary		更新用户信息
// @Description	更新用户信息
// @Tags	mysql
// @Accept	x-www-form-urlencoded
// @Produce json
// @Param	mobile formData string true "手机号"
// @Param	name formData string false "用户名"
// @Param	age formData int false "年龄"
// @Success 200 {string} string	"ok"
// @Router	/user/mysql/update [post][get]
func UpdateUserMysql(params map[string]string) mgresult.Result {
	if !utils.Exists(params, "mobile") {
		return *mgresult.Error(-1, "手机号不可为空")
	}
	age, _ := strconv.Atoi(params["age"])
	return service.UpdateUserMysql(params["mobile"], params["name"], age)
}

// GetUserMysqlByMobile	godoc
// @Summary		按手机号查询用户信息
// @Description	按手机号查询用户信息
// @Tags	mysql
// @Accept	x-www-form-urlencoded
// @Produce json
// @Param	mobile formData string true "手机号"
// @Success 200 {string} string	"ok"
// @Router	/user/mysql/get/mobile [post][get]
func GetUserMysqlByMobile(params map[string]string) mgresult.Result {
	if !utils.Exists(params, "mobile") {
		return *mgresult.Error(-1, "手机号不可为空")
	}
	return service.GetUserMysqlByMobile(params["mobile"])
}

// GetUserMysqlById	godoc
// @Summary		按用户编号查询用户信息
// @Description	按用户编号查询用户信息
// @Tags	mysql
// @Accept	x-www-form-urlencoded
// @Produce json
// @Param	id formData int true "编号编号"
// @Success 200 {string} string	"ok"
// @Router	/user/mysql/get/id [post][get]
func GetUserMysqlById(params map[string]string) mgresult.Result {
	if !utils.Exists(params, "id") {
		return *mgresult.Error(-1, "用户编号不可为空")
	}
	id, _ := strconv.Atoi(params["id"])
	return service.GetUserMysqlById(id)
}
