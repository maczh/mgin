package postlog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/db/dao"
	"github.com/maczh/mgin/models"
	"io/ioutil"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/middleware/trace"
	"github.com/maczh/mgin/utils"
)

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

type mongo[E any] struct {
	//insert    func(entity *E) error
	isMultiDB func() bool
	mgodao    dao.Dao[E]
}

func getTag() string {
	if db.Mongo.IsMultiDB() {
		return trace.GetHeader(config.Config.Log.DbName)
	} else {
		return "0"
	}
}

var Mgo = mongo[models.PostLog]{
	//insert:    postlogDao.Insert,
	isMultiDB: db.Mongo.IsMultiDB,
	//mgodao:    &postlogDao,
}

func (m *mongo[E]) Set(mgodao dao.Dao[E], isMultiDBFunc func() bool) {
	m.mgodao = mgodao
	m.isMultiDB = isMultiDBFunc
}

var accessChannel = make(chan string, 100)

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w bodyLogWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

func RequestLogger() gin.HandlerFunc {
	var postlogDao = dao.MgoDao[models.PostLog]{
		CollectionName: config.Config.Log.RequestTableName,
		Tag:            getTag,
	}
	Mgo.mgodao = &postlogDao

	go handleAccessChannel()

	return func(c *gin.Context) {
		bodyLogWriter := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = bodyLogWriter

		// 开始时间
		startTime := time.Now()

		data, err := c.GetRawData()
		if err != nil {
			logs.Error("GetRawData error:", err.Error())
		}
		body := string(data)

		c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(data)) // 关键点

		// 处理请求
		c.Next()

		responseBody := bodyLogWriter.body.String()

		var result any

		// 日志格式
		if strings.Contains(c.Request.RequestURI, "/docs") || c.Request.RequestURI == "/" {
			return
		}

		if responseBody != "" && responseBody[0:1] == "{" {
			err := json.Unmarshal([]byte(responseBody), &result)
			if err != nil {
				result = map[string]any{"status": -1, "msg": "解析异常:" + err.Error()}
			}
		}

		// 结束时间
		endTime := time.Now()

		// 日志格式
		var params, reqBody any
		if strings.Contains(c.ContentType(), "application/json") && body != "" {
			utils.FromJSON(body, &reqBody)
		}
		params = utils.GinParamMap(c)
		postLog := new(models.PostLog)
		//postLog.ID = bson.NewObjectId()
		postLog.Time = startTime.Format("2006-01-02 15:04:05")
		postLog.Uri = c.Request.RequestURI
		postLog.Method = c.Request.Method
		postLog.AppName = config.Config.App.Name
		postLog.RequestId = trace.GetRequestId()
		postLog.ContentType = c.ContentType()
		postLog.RequestHeader = utils.GinHeaders(c)
		ip := c.GetHeader("X-Forward-For")
		if ip == "" {
			ip = c.GetHeader("X-Real-IP")
			if ip == "" {
				ip = c.ClientIP()
			}
		}
		postLog.ClientIP = ip
		postLog.RequestParam = params
		postLog.RequestBody = reqBody
		postLog.ResponseTime = endTime.Format("2006-01-02 15:04:05")
		postLog.ResponseMap = result
		postLog.TTL = int(endTime.UnixNano()/1e6 - startTime.UnixNano()/1e6)

		accessLog := "|" + c.Request.Method + "|" + postLog.Uri + "|" + c.ClientIP() + "|" + endTime.Format("2006-01-02 15:04:05.012") + "|" + fmt.Sprintf("%vms", endTime.UnixNano()/1e6-startTime.UnixNano()/1e6)
		logs.Debug(accessLog)
		logs.Debug("请求参数:{},body: {}", params, body)
		logs.Debug("请求头:{}", postLog.RequestHeader)
		logs.Debug("接口返回:{}", result)

		if config.Config.Log.RequestTableName != "" || config.Config.Log.Kafka.Use {
			accessChannel <- utils.ToJSON(postLog)
		}
	}
}

func handleAccessChannel() {
	if config.Config.Log.LogDb == "" {
		config.Config.Log.LogDb = "mongodb"
	}
	for accessLog := range accessChannel {
		var postLog models.PostLog
		json.Unmarshal([]byte(accessLog), &postLog)
		dbName := ""
		if config.Config.Log.DbName != "" {
			dbName = postLog.RequestHeader[config.Config.Log.DbName]
		}
		//是否写入到kafka
		if config.Config.Log.Kafka.Use {
			topics := strings.Split(config.Config.Log.Kafka.Topic, ",")
			for _, topic := range topics {
				if dbName != "" {
					topic = fmt.Sprintf("%s_%s", topic, dbName)
				}
				err := db.Kafka.Send(topic, accessLog)
				if err != nil {
					logs.Error("接口日志发送到kafka的{}主题失败:{}", topic, err.Error())
				}
			}
		}
		if dbName == "" && Mgo.isMultiDB() {
			logs.Error("日志多库header配置{}错误，请求头中无此参数值", config.Config.Log.DbName)
			continue
		}
		if config.Config.Log.RequestTableName == "" {
			continue
		}
		switch config.Config.Log.LogDb {
		case "mongodb":
			//conn, err := db.Mongo.GetConnection(dbName)
			//if err != nil {
			//	logs.Error("MongoDB连接失败:{}", err.Error())
			//	continue
			//}
			//err = conn.C(config.Config.Log.RequestTableName).insert(postLog)
			//if err != nil {
			//	logs.Error("MongoDB写入错误:" + err.Error())
			//}
			//db.Mongo.ReturnConnection(conn)
			err := Mgo.mgodao.Insert(&postLog)
			if err != nil {
				logs.Error("MongoDB写入错误:" + err.Error())
			}
		case "elasticsearch":
			doc := make(map[string]any)
			utils.FromJSON(utils.ToJSON(postLog), &doc)
			resp, err := db.ElasticSearch.AddDocument(strings.ToLower(config.Config.App.Project), strings.ToLower(config.Config.Log.RequestTableName), doc, []string{})
			if err != nil {
				logs.Error("ElasticSearch写入日志失败:{}", err.Error())
				continue
			}
			logs.Debug("日志写入ElasticSearch返回:{}", resp)
		}
	}
	return
}
