package mongo

import (
	"errors"
	"github.com/maczh/mgconfig"
	"github.com/maczh/mgin/model"
	"gopkg.in/mgo.v2/bson"
)

const COLLECTION_USER  = "user"

func InsertUser(user model.UserMongo) (model.UserMongo,error) {
	user.Id = bson.NewObjectId()
	mongo := mgconfig.GetMongoConnection()
	defer mgconfig.ReturnMongoConnection(mongo)
	if mongo == nil {
		return model.UserMongo{}, errors.New("MongoDB连接异常")
	}
	err := mongo.C(COLLECTION_USER).Insert(&user)
	if err != nil {
		return model.UserMongo{}, err
	}
	return user, nil
}


func UpdateUser(user model.UserMongo) error {
	user.Id = bson.NewObjectId()
	mongo := mgconfig.GetMongoConnection()
	defer mgconfig.ReturnMongoConnection(mongo)
	if mongo == nil {
		return errors.New("MongoDB连接异常")
	}
	err := mongo.C(COLLECTION_USER).UpdateId(user.Id,&user)
	if err != nil {
		return err
	}
	return nil
}


func GetUserByMobile(mobile string) (model.UserMongo,error) {
	mongo := mgconfig.GetMongoConnection()
	defer mgconfig.ReturnMongoConnection(mongo)
	if mongo == nil {
		return model.UserMongo{}, errors.New("MongoDB连接异常")
	}
	var user model.UserMongo
	err := mongo.C(COLLECTION_USER).Find(&bson.M{"mobile":mobile}).One(&user)
	if err != nil {
		return model.UserMongo{}, err
	}
	return user, nil
}

