package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/examples/mgin-server/model"
	"github.com/maczh/mgin/examples/mgin-server/service"
	"github.com/maczh/mgin/i18n"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models"
)

type userController struct{}

var User = &userController{}

func (u *userController) Insert(c *gin.Context) models.Result {
	var user model.User
	err := c.BindJSON(&user)
	if err != nil {
		logs.Error("Post Data json error: {}", err.Error())
		return i18n.ParamError("user")
	}
	return service.User.Add(user)
}

func (u *userController) Query(params map[string]string) models.Result {
	if rs := i18n.CheckParametersLost(params, "name"); rs.Status != 1 {
		return rs
	}
	return service.User.Query(params["name"])
}
