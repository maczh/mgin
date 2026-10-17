package i18n

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/maczh/mgin/errcode"
	"github.com/maczh/mgin/middleware/xlang"
	"github.com/maczh/mgin/models"
)

// GetString resolves a message from the built-in dictionary with language fallback.
func GetString(messageID, language string) string {
	s, err := GetStore()
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	text, err := s.Translate(ctx, messageID, language)
	if err != nil {
		return ""
	}
	return text
}

// GetXLangString is kept for source compatibility and uses the local dictionary.
// Deprecated: use GetString.
func GetXLangString(messageID, language string) string { return GetString(messageID, language) }

func Error(code int, messageId string) models.Result[any] {
	return models.Error(code, String(messageId))
}

func ErrorT[T any](code int, messageId string) models.Result[T] {
	return models.ErrorT[T](code, String(messageId))
}

func ErrorWithMsg(code int, messageId, msg string) models.Result[any] {
	return models.Error(code, fmt.Sprintf("%s:%s", String(messageId), msg))
}

func ErrorWithMsgT[T any](code int, messageId, msg string) models.Result[T] {
	return models.ErrorT[T](code, fmt.Sprintf("%s:%s", String(messageId), msg))
}
func Success[T any](data T) models.Result[T] {
	return models.SuccessWithMsg[T](String("success"), data)
}

func SuccessWithPage[T any](data T, count, index, size, total int) models.Result[T] {
	return models.Result[T]{
		Status: 1,
		Msg:    String("success"),
		Data:   data,
		Page: &models.ResultPage{
			Count: count,
			Index: index,
			Size:  size,
			Total: total,
		},
	}
}

// String 将messageId根据当前协程X-Lang参数转换成当前语言字符串
func String(messageId string) string {
	lang := xlang.GetCurrentLanguage()
	s := GetString(messageId, lang)
	if s != "" {
		return s
	}
	return messageId
}

// Format 格式化数据，messageId对应的内容为带{}的模板
func Format(messageId string, args ...any) string {
	format := String(messageId)
	for _, value := range args {
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
			j, _ := json.Marshal(value)
			str = string(j)
		default:
			j, _ := json.Marshal(value)
			str = string(j)
		}
		format = strings.Replace(format, "{}", str, 1)
	}
	return format
}

func ParamLostError(param string) models.Result[any] {
	return models.Error(errcode.REQUEST_PARAMETER_LOST, Format("参数不可为空", param))
}

func ParamError(param string) models.Result[any] {
	return models.Error(errcode.REQUEST_PARAMETER_LOST, Format("参数错误", param))
}

func CheckParametersLost(params map[string]string, paramNames ...string) models.Result[any] {
	for _, param := range paramNames {
		v := params[param]
		if v == "" {
			return ParamLostError(param)
		}
	}
	return Success[any](nil)
}
