package model

import "gopkg.in/mgo.v2/bson"

type User struct{
	Name string `json:"name" gorm:"column:name" bson:"name"`
	Age int `json:"age" gorm:"column:age" bson:"age"`
	Mobile string `json:"mobile" gorm:"column:mobile" bson:"mobile"`
}

type UserMysql struct {
	Id int `json:"id" gorm:"column:id,primary_key,auto_increment"`
	User
}

type UserMongo struct {
	Id bson.ObjectId `json:"id" bson:"_id"`
	User
}
