package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/examples/mgin-client/mgclient"
	"github.com/maczh/mgin/examples/mgin-server/model"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models"
)

type userController struct{}

var User = &userController{}

func (u *userController) Add(c *gin.Context) models.Result {
	var user model.User
	err := c.BindJSON(&user)
	if err != nil {
		logs.Error("Post Data json error: {}", err.Error())
		return models.Error(-1, err.Error())
	}
	return mgclient.User.Add(user)
}

func (u *userController) Query(params map[string]string) models.Result {
	return mgclient.User.Get(params)
}
