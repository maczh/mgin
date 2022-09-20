package models

import (
	"gopkg.in/mgo.v2/bson"
)

type PostLog struct {
	ID           bson.ObjectId          `bson:"_id"`
	Time         string                 `json:"time" bson:"time"`
	RequestId    string                 `json:"requestId" bson:"requestId"`
	Responsetime string                 `json:"responsetime" bson:"responsetime"`
	TTL          int                    `json:"ttl" bson:"ttl"`
	Apiname      string                 `json:"apiName" bson:"apiName"`
	Method       string                 `json:"method" bson:"method"`
	ContentType  string                 `json:"contentType" bson:"contentType"`
	Uri          string                 `json:"uri" bson:"uri"`
	Requestparam interface{}            `json:"requestparam" bson:"requestparam"`
	Responsestr  string                 `json:"responsestr" bson:"responsestr"`
	Responsemap  map[string]interface{} `json:"responsemap" bson:"responsemap"`
}
