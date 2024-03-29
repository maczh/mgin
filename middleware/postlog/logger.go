package postlog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/models"
	"io/ioutil"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/middleware/trace"
	"github.com/maczh/mgin/utils"
	"gopkg.in/mgo.v2/bson"
)

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
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

		var req map[string]interface{}
		var result map[string]interface{}

		// 日志格式
		if strings.Contains(c.Request.RequestURI, "/docs") || c.Request.RequestURI == "/" {
			return
		}

		if responseBody != "" && responseBody[0:1] == "{" {
			err := json.Unmarshal([]byte(responseBody), &result)
			if err != nil {
				result = map[string]interface{}{"status": -1, "msg": "解析异常:" + err.Error()}
			}
		}

		// 结束时间
		endTime := time.Now()

		// 日志格式
		var params interface{}
		if strings.Contains(c.ContentType(), "application/json") && body != "" {
			utils.FromJSON(body, &req)
			params = req
		} else if strings.Contains(c.ContentType(), "x-www-form-urlencoded") || strings.Contains(c.ContentType(), "multipart/form-data") {
			params = utils.GinParamMap(c)
		} else if c.Request.Method != "GET" && c.Request.Method != "DELETE" {
			return
		}
		postLog := new(models.PostLog)
		postLog.ID = bson.NewObjectId()
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
		postLog.ResponseTime = endTime.Format("2006-01-02 15:04:05")
		postLog.ResponseMap = result
		postLog.TTL = int(endTime.UnixNano()/1e6 - startTime.UnixNano()/1e6)

		accessLog := "|" + c.Request.Method + "|" + postLog.Uri + "|" + c.ClientIP() + "|" + endTime.Format("2006-01-02 15:04:05.012") + "|" + fmt.Sprintf("%vms", endTime.UnixNano()/1e6-startTime.UnixNano()/1e6)
		logs.Debug(accessLog)
		logs.Debug("请求参数:{}", params)
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
		if dbName == "" && db.Mongo.IsMultiDB() {
			logs.Error("日志多库header配置{}错误，请求头中无此参数值", config.Config.Log.DbName)
			continue
		}
		if config.Config.Log.RequestTableName == "" {
			continue
		}
		switch config.Config.Log.LogDb {
		case "mongodb":
			conn, err := db.Mongo.GetConnection(dbName)
			if err != nil {
				logs.Error("MongoDB连接失败:{}", err.Error())
				continue
			}
			err = conn.C(config.Config.Log.RequestTableName).Insert(postLog)
			if err != nil {
				logs.Error("MongoDB写入错误:" + err.Error())
			}
			db.Mongo.ReturnConnection(conn)
		case "elasticsearch":
			doc := make(map[string]interface{})
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
