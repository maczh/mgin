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
	"sync"
	"time"

	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/logsink"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/middleware/trace"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/utils"
)

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

var (
	initOnce  sync.Once
	closeOnce sync.Once
)

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

// shouldSkipLog 判断是否跳过接口日志。
// 注意：Swagger.Uri 为空时必须跳过该判断，否则 strings.Contains(path, "") 恒为真会屏蔽全部日志。
func shouldSkipLog(path string) bool {
	if path == "" || path == "/" || strings.Contains(path, "/docs/") || strings.Contains(path, "/swagger/") {
		return true
	}
	if uri := config.Config.Sys.Swagger.Uri; uri != "" && strings.Contains(path, uri) {
		return true
	}
	return false
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

// RegisterHandler 挂接一个接口日志处理器，等价于 logsink.Register。
// 外部插件（mgkafka、elasticsearch 插件等）或业务代码在应用启动时调用即可接收全量接口日志。
//
//	postlog.RegisterHandler(logsink.NewHandlerFunc("kafka", func(e *logsink.Entry) error {
//	    return mgkafka.Kafka.Send(topic, utils.ToJSON(e))
//	}))
func RegisterHandler(h logsink.Handler, opts ...logsink.Option) error {
	return logsink.Register(h, opts...)
}

// Close 停止全部日志处理器并尽力排空队列，进程退出前调用（mgin.SafeExit 已内置调用）。
func Close() {
	closeOnce.Do(func() { logsink.Close(shutdownTimeout()) })
}

func RequestLogger() gin.HandlerFunc {
	initOnce.Do(func() {
		logsink.SetDropLogger(func(name string, dropped uint64) {
			logs.Warn("接口日志handler {} 队列已满，累计丢弃 {} 条，请调大 go.log.sink.queue 或排查下游性能", name, dropped)
		})
		logsink.SetErrorLogger(func(name string, err error) {
			logs.Error("接口日志handler {} 处理失败:{}", name, err.Error())
		})
		registerMongoHandler()
	})

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
			} else {
				defer r.Close()
				rBody, err := io.ReadAll(r)
				if err != nil {
					logs.Error("io.ReadAll error:", err.Error())
				} else {
					responseBody = string(rBody)
				}
			}
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

		// 文档类路径不记录接口日志
		if shouldSkipLog(c.Request.URL.Path) {
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

		// 异步投递给所有已注册的 handler（无 handler 时直接返回，零开销）
		logsink.Emit(postLog)
	}
}
