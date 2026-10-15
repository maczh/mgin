package logs

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/maczh/mgin/v2/pkg/middleware/trace"
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

func toJSON(o any) string {
	j, err := json.Marshal(o)
	if err != nil {
		return "nil"
	} else {
		js := string(j)
		js = strings.Replace(js, "\\u003c", "<", -1)
		js = strings.Replace(js, "\\u003e", ">", -1)
		js = strings.Replace(js, "\\u0026", "&", -1)
		return js
	}
}

func formatMessage(format string, v []any) string {
	for _, value := range v {
		str := ""
		if value == nil {
			value = "nil"
		}
		switch value.(type) {
		case bool:
			str = strconv.FormatBool(value.(bool))
		case float32, float64:
			str = fmt.Sprintf("%.6f", value)
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, uintptr:
			str = fmt.Sprintf("%d", value)
		case string:
			str = value.(string)
		case []byte:
			str = string(value.([]byte))
		case reflect.Value:
			str = toJSON(value)
		default:
			str = toJSON(value)
		}
		if str == "{}" {
			str = "{ }"
		}
		format = strings.Replace(format, "{}", str, 1)
	}
	return format
}

// OutPrint formats placeholders and includes the current request ID.
func OutPrint(format string, v []any) string {
	message := formatMessage(format, v)
	if id := trace.GetRequestId(); id != "" {
		message = "[" + id + "] " + message
	}
	return message
}

func Debug(format string, v ...any) { emit("debug", format, v) }
func Info(format string, v ...any)  { emit("info", format, v) }
func Warn(format string, v ...any)  { emit("warn", format, v) }
func Error(format string, v ...any) { emit("error", format, v) }
