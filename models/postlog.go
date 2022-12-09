package models

import (
	"gopkg.in/mgo.v2/bson"
)

type PostLog struct {
	ID            bson.ObjectId     `bson:"_id"`
	Time          string            `json:"time" bson:"time"`
	RequestId     string            `json:"requestId" bson:"requestId"`
	ResponseTime  string            `json:"responseTime" bson:"responseTime"`
	TTL           int               `json:"ttl" bson:"ttl"`
	AppName       string            `json:"appName" bson:"appName"`
	Apiname       string            `json:"apiName" bson:"apiName"`
	Method        string            `json:"method" bson:"method"`
	ContentType   string            `json:"contentType" bson:"contentType"`
	Uri           string            `json:"uri" bson:"uri"`
	ClientIP      string            `json:"clientIP" bson:"clientIP"`
	RequestHeader map[string]string `json:"requestHeader" bson:"requestHeader"`
	RequestParam  any               `json:"requestParam" bson:"requestParam"`
	ResponseStr   string            `json:"responseStr" bson:"responseStr"`
	ResponseMap   any               `json:"responseMap" bson:"responseMap"`
}
