package db

import (
	"github.com/maczh/mgin/db/mongo"
	"github.com/maczh/mgin/db/mysql"
	"github.com/maczh/mgin/db/redis"
	"github.com/maczh/mgin/db/sqlite"
)

var Mysql = &mysql.MysqlClient{}
var Mongo = &mongo.Mongodb{}
var Redis = redis.Redis
var Sqlite = &sqlite.Sqlite{}
