package db

import (
	"github.com/maczh/mgin/v2/pkg/db/clickhouse"
	"github.com/maczh/mgin/v2/pkg/db/es"
	"github.com/maczh/mgin/v2/pkg/db/kafka"
	"github.com/maczh/mgin/v2/pkg/db/mongo"
	"github.com/maczh/mgin/v2/pkg/db/mysql"
	"github.com/maczh/mgin/v2/pkg/db/postgres"
	"github.com/maczh/mgin/v2/pkg/db/redis"
	"github.com/maczh/mgin/v2/pkg/db/sqlite"
)

var Mysql = &mysql.MysqlClient{}
var Mongo = &mongo.Mongodb{}
var Redis = redis.Redis
var ElasticSearch = &es.ElasticSearch{}
var Kafka = &kafka.Kafka{}
var Sqlite = &sqlite.Sqlite{}
var Clickhouse = &clickhouse.ClickhouseClient{}
var Pg = &postgres.PostgresClient{}
