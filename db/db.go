package db

import (
	"github.com/maczh/mgin/db/es"
	"github.com/maczh/mgin/db/kafka"
	"github.com/maczh/mgin/db/mongo"
	"github.com/maczh/mgin/db/mysql"
	"github.com/maczh/mgin/db/redis"
)

var Mysql = &mysql.MysqlClient{}
var Mongo = &mongo.Mongodb{}
var Redis = &redis.RedisClient{}
var ElasticSearch = &es.ElasticSearch{}
var Kafka = &kafka.Kafka{}
