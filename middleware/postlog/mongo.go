package postlog

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/db/dao"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/logsink"
	"github.com/maczh/mgin/models"
)

// MongoSinkName 内置 MongoDB 落库 handler 的注册名。
const MongoSinkName = "mongodb"

// mongoHandler 把接口日志写入 MongoDB，行为与此前的 channel 消费者保持一致。
type mongoHandler struct {
	mgodao    dao.Dao[models.PostLog]
	isMultiDB func() bool
}

func (h *mongoHandler) Name() string { return MongoSinkName }

func (h *mongoHandler) Write(entry *logsink.Entry) error {
	if config.Config.Log.RequestTableName == "" {
		return nil
	}
	if h.mgodao == nil {
		return errors.New("MongoDB日志DAO未初始化")
	}
	if config.Config.Log.DbName == "" && h.isMultiDB != nil && h.isMultiDB() {
		return fmt.Errorf("当前为多库模式，但 go.log.dbName 未配置，无法选择日志库")
	}
	return h.mgodao.Insert(entry)
}

// registerMongoHandler 在中间件首次挂载时自动注册 MongoDB handler，保持旧行为零改动。
func registerMongoHandler() {
	if config.Config.Log.RequestTableName == "" {
		return
	}
	if !strings.Contains(config.Config.Config.Used, "mongodb") {
		logs.Warn("已配置 go.log.req={}，但 go.config.used 中未包含 mongodb，接口日志不落库；如需落库请自行注册 handler 或启用 mongodb",
			config.Config.Log.RequestTableName)
		return
	}
	postlogDao := &dao.MgoDao[models.PostLog]{
		CollectionName: config.Config.Log.RequestTableName,
		Tag:            getTag,
	}
	if err := logsink.Register(&mongoHandler{mgodao: postlogDao, isMultiDB: db.Mongo.IsMultiDB}, sinkOptions()...); err != nil {
		logs.Error("注册接口日志MongoDB handler失败:{}", err.Error())
		return
	}
	logs.Info("接口日志将异步写入 MongoDB 集合 {}", config.Config.Log.RequestTableName)
}

// getTag 决定日志写入哪个库（多库模式下按配置选择）。
func getTag() string {
	if db.Mongo.IsMultiDB() {
		return config.Config.Log.DbName
	}
	return "0"
}

// shutdownTimeout 从配置读取关闭时等待队列排空的时间。
func shutdownTimeout() time.Duration {
	if config.Config.Log.Sink.Shutdown > 0 {
		return time.Duration(config.Config.Log.Sink.Shutdown) * time.Millisecond
	}
	return logsink.DefaultShutdownTimeout
}

// sinkOptions 从配置读取队列参数。
func sinkOptions() []logsink.Option {
	opts := make([]logsink.Option, 0, 2)
	if config.Config.Log.Sink.Queue > 0 {
		opts = append(opts, logsink.WithQueueSize(config.Config.Log.Sink.Queue))
	}
	if config.Config.Log.Sink.Workers > 0 {
		opts = append(opts, logsink.WithWorkers(config.Config.Log.Sink.Workers))
	}
	return opts
}
