package i18n

import (
	"encoding/json"
	"fmt"
	"github.com/maczh/mgin/cache"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/errcode"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/middleware/xlang"
	"github.com/maczh/mgin/models"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var appName, defaultLanguage string

func Init() {
	appName = config.Config.GetConfigString("go.xlang.appName")
	if appName == "" {
		appName = config.Config.App.Name
	}
	defaultLanguage = config.Config.GetConfigString("go.xlang.default")
	if defaultLanguage == "" {
		defaultLanguage = "zh-cn"
	}
	version, err := GetAppXLangVersion(appName)
	if err != nil {
		logs.Error(err.Error())
		return
	}
	initCache(version)
	//设置定时任务自动检查
	ticker := time.NewTicker(time.Minute * 5)
	go func() {
		for _ = range ticker.C {
			refreshXLangCache()
		}
	}()
}

func initCache(version string) {
	cache.OnGetCache("x-lang").Add("version", version, 0)
	//从公共应用加载公共常用多语言数据
	langs, err := GetAppXLangStringsAll("default")
	if err != nil {
		logs.Error(err.Error())
		return
	}
	for k, v := range langs {
		cache.OnGetCache("x-lang").Add(k, v, 0)
	}

	//加载本程序所有多语言字符串数据
	langs, err = GetAppXLangStringsAll(appName)
	if err != nil {
		logs.Error(err.Error())
		return
	}
	for k, v := range langs {
		cache.OnGetCache("x-lang").Add(k, v, 0)
	}
}

func GetXLangString(stringId, lang string) string {
	key := fmt.Sprintf("%s:%s", stringId, lang)
	str, ok := cache.OnGetCache("x-lang").Value(key)
	if ok {
		return str.(string)
	}
	key = fmt.Sprintf("%s:%s", stringId, defaultLanguage)
	str, ok = cache.OnGetCache("x-lang").Value(key)
	if ok {
		return str.(string)
	}
	return ""
}

func refreshXLangCache() {
	version, err := GetAppXLangVersion(appName)
	if err != nil {
		logs.Error(err.Error())
		return
	}
	oldVersion, ok := cache.OnGetCache("x-lang").Value("version")
	if ok {
		if oldVersion != version {
			initCache(version)
		}
	}
}

func Error(code int, messageId string) models.Result[any] {
	return models.Error(code, String(messageId))
}

func ErrorWithMsg(code int, messageId, msg string) models.Result[any] {
	return models.Error(code, fmt.Sprintf("%s:%s", String(messageId), msg))
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
	s := GetXLangString(messageId, lang)
	if s != "" {
		return s
	}
	return messageId
}

// Format 格式化数据，messageId对应的内容为带{}的模板
func Format(messageId string, args ...interface{}) string {
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
