package service

import (
	"github.com/maczh/mgin/errcode"
	"github.com/maczh/mgin/examples/mgin-server/model"
	"github.com/maczh/mgin/examples/mgin-server/mongo"
	"github.com/maczh/mgin/i18n"
	"github.com/maczh/mgin/models"
)

type userService struct{}

var User = &userService{}

func (s *userService) Add(user model.User) models.Result {
	u, err := mongo.Insert(user)
	if err != nil {
		return i18n.ErrorWithMsg(errcode.DB_CONNECT_ERROR, "数据库插入失败", err.Error())
	}
	return i18n.Success(u)
}

func (s *userService) Query(name string) models.Result {
	u, err := mongo.QueryUser(name)
	if err != nil {
		return i18n.ErrorWithMsg(errcode.DB_CONNECT_ERROR, "数据库查询失败", err.Error())
	}
	return i18n.Success(u)
}
