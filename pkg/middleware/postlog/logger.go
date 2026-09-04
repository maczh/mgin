package postlog

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"path"
	"strings"
	"time"

	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/maczh/mgin/v2/pkg/db"
	"github.com/maczh/mgin/v2/pkg/db/dao"
	"github.com/maczh/mgin/v2/pkg/models"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/v2/pkg/logs"
	"github.com/maczh/mgin/v2/pkg/middleware/trace"
	"github.com/maczh/mgin/v2/pkg/utils"
)

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

type mongo[E any] struct {
	//insert    func(entity *E) error
	isMultiDB func() bool
	// v2：mgodao 仅用于 Insert，dao.Dao[E] 扩展后要求 SQL 风格的方法集，
	// 与 MgoDao（mongo 风格）签名不一致。改用只含 Insert 的内联小接口解耦。
	mgodao insertOnly[E]
}

// insertOnly 是 mongo[E].mgodao 实际需要的最小接口，避免与 dao.Dao[E] 强耦合。
// 这样 postlog 仍可接受任何实现 Insert(*E) error 的实现（包括 MgoDao、MySQLDao 等）。
type insertOnly[E any] interface {
	Insert(entity *E) error
}

func getTag() string {
	if db.Mongo.IsMultiDB() {
		return config.Config.Log.DbName
	} else {
		return "0"
	}
}

var Mgo = mongo[models.PostLog]{
	//insert:    postlogDao.Insert,
	isMultiDB: db.Mongo.IsMultiDB,
	//mgodao:    &postlogDao,
}

func (m *mongo[E]) Set(mgodao insertOnly[E], isMultiDBFunc func() bool) {
	m.mgodao = mgodao
	m.isMultiDB = isMultiDBFunc
}

var accessChannel = make(chan string, 100)

var fileResponseFormats = map[string]string{
	"application/zip":    "zip",
	"application/msword": "doc",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": "docx",
	"application/vnd.ms-excel": "xls",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         "xlsx",
	"application/vnd.ms-powerpoint":                                             "ppt",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": "pptx",
	"application/pdf":    "pdf",
	"application/gzip":   "gz",
	"application/x-gzip": "gz",
	"application/x-tar":  "tar",
	"text/csv":           "csv",
	"text/plain":         "txt",
}

var fileResponseExtensions = map[string]bool{
	"zip": true, "doc": true, "docx": true, "xls": true, "xlsx": true,
	"ppt": true, "pptx": true, "pdf": true, "gz": true, "tar": true,
	"csv": true, "txt": true,
}

func fileResponseSummary(contentType, contentDisposition, requestPath string, size int) string {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	format := fileResponseFormats[strings.ToLower(mediaType)]

	if format == "" {
		if _, params, err := mime.ParseMediaType(contentDisposition); err == nil {
			format = fileExtension(params["filename"])
		}
	}
	if format == "" {
		format = fileExtension(requestPath)
	}
	if !fileResponseExtensions[format] {
		return ""
	}

	return fmt.Sprintf("输出%s格式文件，大小%.2fMB", format, float64(size)/(1024*1024))
}

func fileExtension(filename string) string {
	extension := strings.TrimPrefix(strings.ToLower(path.Ext(filename)), ".")
	if fileResponseExtensions[extension] {
		return extension
	}
	return ""
}

func getResponseLogMode() string {
	mode := strings.ToLower(config.Config.Log.Get)
	if mode != "line" && mode != "off" {
		return "full"
	}
	return mode
}

func oneLineResponse(response string) string {
	response = strings.Join(strings.Fields(response), " ")
	responseRunes := []rune(response)
	if len(responseRunes) <= 180 {
		return response
	}
	return string(responseRunes[:176]) + " ..."
}

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
		logs.Debug("请求 {} {}", c.Request.Method, c.Request.RequestURI)
		params := utils.GinParamMap(c)
		if c.ContentType() == gin.MIMEJSON {
			logs.Debug("请求参数:{}", body)
		} else {
			logs.Debug("请求参数:{}", params)
		}
		headers := utils.GinHeaders(c)
		logs.Debug("请求头:{}", headers)

		// 处理请求
		c.Next()

		responseBody := bodyLogWriter.body.String()
		fileSummary := fileResponseSummary(bodyLogWriter.Header().Get("Content-Type"), bodyLogWriter.Header().Get("Content-Disposition"), c.Request.URL.Path, bodyLogWriter.body.Len())
		if fileSummary != "" {
			responseBody = fileSummary
		}
		//如果gzip压缩，需要解压缩
		if fileSummary == "" && strings.Contains(bodyLogWriter.Header().Get("Content-Encoding"), "gzip") {
			r, err := gzip.NewReader(bytes.NewBufferString(responseBody))
			if err != nil {
				logs.Error("gzip.NewReader error:", err.Error())
			}
			defer r.Close()
			rBody, err := io.ReadAll(r)
			if err != nil {
				logs.Error("io.ReadAll error:", err.Error())
			}
			responseBody = string(rBody)
		}
		responseDatabaseBody := responseBody
		responseLogBody := responseBody
		if c.Request.Method == "GET" {
			switch getResponseLogMode() {
			case "line":
				responseLogBody = oneLineResponse(responseBody)
			case "off":
				responseLogBody = ""
				responseDatabaseBody = ""
			}
		}
		var result any

		// 日志格式
		if strings.Contains(c.Request.URL.Path, "/docs/") || strings.Contains(c.Request.URL.Path, "/swagger/") || strings.Contains(c.Request.URL.Path, config.Config.Sys.Swagger.Uri) || c.Request.URL.Path == "/" {
			return
		}

		if responseDatabaseBody != "" && responseDatabaseBody[0:1] == "{" {
			err := json.Unmarshal([]byte(responseDatabaseBody), &result)
			if err != nil {
				result = map[string]any{"status": -1, "msg": "解析异常:" + err.Error()}
			}
		}

		// 结束时间
		endTime := time.Now()

		// 日志格式
		var reqBody any
		if strings.Contains(c.ContentType(), "application/json") && body != "" {
			utils.FromJSON(body, &reqBody)
		}
		postLog := new(models.PostLog)
		//postLog.ID = bson.NewObjectId()
		postLog.Time = startTime.Format("2006-01-02 15:04:05")
		postLog.Uri = c.Request.URL.Path
		postLog.Method = c.Request.Method
		postLog.AppName = config.Config.App.Name
		postLog.RequestId = trace.GetRequestId()
		postLog.ContentType = c.ContentType()
		postLog.RequestHeader = headers
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
		postLog.ResponseStr = responseDatabaseBody
		postLog.TTL = int(endTime.UnixNano()/1e6 - startTime.UnixNano()/1e6)

		accessLog := "|" + c.Request.Method + "|" + postLog.Uri + "|" + c.ClientIP() + "|" + endTime.Format("2006-01-02 15:04:05.012") + "|" + fmt.Sprintf("%vms", endTime.UnixNano()/1e6-startTime.UnixNano()/1e6)
		logs.Debug(accessLog)
		if responseLogBody != "" {
			logs.Debug("接口返回:{}", responseLogBody)
		}

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
			dbName = config.Config.Log.DbName
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
}
