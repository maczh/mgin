package db

import (
	"github.com/maczh/mgin/v2/pkg/db/mongo"
	"github.com/maczh/mgin/v2/pkg/db/mysql"
	"github.com/maczh/mgin/v2/pkg/db/redis"
	"github.com/maczh/mgin/v2/pkg/db/sqlite"
)

var Mysql = &mysql.MysqlClient{}
var Mongo = &mongo.Mongodb{}
var Redis = redis.Redis
var Sqlite = &sqlite.Sqlite{}
