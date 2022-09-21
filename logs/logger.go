package logs

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/middleware/trace"
	"reflect"
	"strconv"
	"strings"
)

type Logger struct {
	PrinterType string
	Location    string
}

type LogInstance struct {
	LogType    string
	Message    string
	LoggerInit Logger
}

var logger GoLogger
var logLevel = "debug"

func initConfig() {
	l := config.Config.Logger.Out
	if l != "" {
		loggers := strings.Split(l, ",")
		logger = GetLogger(loggers...)
	} else {
		logger = GetLogger()
	}
	level := config.Config.Logger.Level
	if level != "" {
		logLevel = level
	}
}

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func toJSON(o interface{}) string {
	j, err := json.Marshal(o)
	if err != nil {
		return "{}"
	} else {
		js := string(j)
		js = strings.Replace(js, "\\u003c", "<", -1)
		js = strings.Replace(js, "\\u003e", ">", -1)
		js = strings.Replace(js, "\\u0026", "&", -1)
		return js
	}
}

func OutPrint(format string, v []interface{}) string {
	for _, value := range v {
		str := ""
		switch value.(type) {
		case bool:
			str = strconv.FormatBool(value.(bool))
		case float32, float64:
			str = strconv.FormatFloat(value.(float64), 'f', 6, 32)
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, uintptr:
			str = strconv.Itoa(value.(int))
		case string:
			str = value.(string)
		case []byte:
			str = string(value.([]byte))
		case reflect.Value:
			str = toJSON(value)
		default:
			str = toJSON(value)
		}
		format = strings.Replace(format, "{}", str, 1)
	}
	traceId := trace.GetRequestId()
	if traceId != "" {
		format = "[" + traceId + "] " + format
	}
	return format
}

func Debug(format string, v ...interface{}) {
	initConfig()
	switch logLevel {
	case "debug":
		logger.Debug(OutPrint(format, v))
	}
}

func Info(format string, v ...interface{}) {
	initConfig()
	switch logLevel {
	case "debug", "info":
		logger.Info(OutPrint(format, v))
	}
}
func Warn(format string, v ...interface{}) {
	initConfig()
	switch logLevel {
	case "debug", "info", "warn":
		logger.Warn(OutPrint(format, v))
	}
}
func Error(format string, v ...interface{}) {
	initConfig()
	switch logLevel {
	case "debug", "info", "warn", "error":
		logger.Error(OutPrint(format, v))
	}
}
