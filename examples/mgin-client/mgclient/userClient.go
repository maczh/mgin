package mgclient

import (
	"github.com/maczh/mgin/client"
	"github.com/maczh/mgin/examples/mgin-server/model"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/utils"
)

const (
	ServiceExample = "mgin-server"
	UriUserAdd     = "/api/v1/user/add"
	UriUserQuery   = "/api/v1/user/get"
)

type user struct{}

var User = &user{}

func (u *user) Add(userInfo model.User) models.Result {
	resp, err := client.Nacos.Call(ServiceExample, UriUserAdd, "POST", userInfo)
	if err != nil {
		return models.Error(-1, err.Error())
	}
	var result models.Result
	utils.FromJSON(resp, &result)
	return result
}

func (u *user) Get(param map[string]string) models.Result {
	resp, err := client.Nacos.Call(ServiceExample, UriUserQuery, "GET", nil, param)
	if err != nil {
		return models.Error(-1, err.Error())
	}
	var result models.Result
	utils.FromJSON(resp, &result)
	return result
}
