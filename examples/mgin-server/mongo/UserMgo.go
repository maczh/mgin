package mongo

import (
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/examples/mgin-server/model"
	"github.com/maczh/mgin/logs"
	"gopkg.in/mgo.v2/bson"
)

func Insert(user model.User) (model.User, error) {
	mgo, err := db.Mongo.GetConnection()
	if err != nil {
		logs.Error("MongoDB connection fail: {}", err.Error())
		return model.User{}, err
	}
	user.Id = bson.NewObjectId()
	err = mgo.C("User").Insert(user)
	if err != nil {
		logs.Error("MongoDB insert fail: {}", err.Error())
		return model.User{}, err
	}
	return user, nil
}

func QueryUser(name string) ([]model.User, error) {
	var users []model.User
	mgo, err := db.Mongo.GetConnection()
	if err != nil {
		logs.Error("MongoDB connection fail: {}", err.Error())
		return users, err
	}
	err = mgo.C("User").Find(bson.M{"name": name}).All(&users)
	return users, err
}
