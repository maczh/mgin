package xlang

import (
	"bytes"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/v2/pkg/cache"
	"github.com/maczh/mgin/v2/pkg/config"
)

func RequestLanguage() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := c.GetHeader("X-Lang")
		if lang == "" {
			lang = defaultLanguage()
		}
		routineId := getGoroutineID()
		cache.OnGetCache("Lang").Add(routineId, lang, 5*time.Minute)
		defer cache.OnGetCache("Lang").Delete(routineId)
		c.Next()
	}
}

func GetCurrentLanguage() string {
	lang, found := cache.OnGetCache("Lang").Value(getGoroutineID())
	if found {
		if s, ok := lang.(string); ok {
			return s
		}
	}
	return defaultLanguage()
}

func getGoroutineID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	if i := bytes.IndexByte(b, ' '); i >= 0 {
		b = b[:i]
	} else {
		return 0
	}
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}

func defaultLanguage() string {
	language := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(config.Config.GetConfigString("go.i18n.default")), "_", "-"))
	if language == "" {
		return "zh-cn"
	}
	return language
}
