package xlang

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/cache"
	"runtime"
	"strconv"
	"time"
)

func RequestLanguage() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := c.GetHeader("X-Lang")
		if lang == "" {
			lang = "zh-cn"
		}
		routineId := getGoroutineID()
		cache.OnGetCache("Lang").Add(routineId, lang, 5*time.Minute)
	}
}

func GetCurrentLanguage() string {
	lang, found := cache.OnGetCache("Lang").Value(getGoroutineID())
	if found {
		if s, ok := lang.(string); ok {
			return s
		}
	}
	return "zh-cn"
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
