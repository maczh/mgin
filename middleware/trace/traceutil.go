package trace

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/cache"
	"math/rand"
	"runtime"
	"strconv"
	"time"
)

func PutRequestId(c *gin.Context) {
	headers := getHeaders(c)
	requestId := headers["X-Request-Id"]
	if requestId == "" {
		headers["X-Request-Id"] = getRandomHexString(16)
	}
	routineId := GetGoroutineID()
	clientIp := c.ClientIP()
	if c.GetHeader("X-Real-IP") != "" {
		clientIp = c.GetHeader("X-Real-IP")
	}
	if c.GetHeader("X-Forwarded-For") != "" {
		clientIp = c.GetHeader("X-Forwarded-For")
	}
	headers["X-Real-IP"] = clientIp
	if headers["X-User-Agent"] == "" {
		headers["X-User-Agent"] = headers["User-Agent"]
	}
	cache.OnGetCache("Header").Add(routineId, headers, 5*time.Minute)
}

func GetRequestId() string {
	return GetHeader("X-Request-Id")
}

func GetClientIp() string {
	return GetHeader("X-Real-IP")
}

func GetUserAgent() string {
	return GetHeader("X-User-Agent")
}

func GetHeader(header string) string {
	headers := GetHeaders()
	return headers[header]
}

func SetHeader(key, value string) {
	headers := GetHeaders()
	headers[key] = value
	cache.OnGetCache("Header").Add(GetGoroutineID(), headers, 5*time.Minute)
}

func GetHeaders() map[string]string {
	headers, found := cache.OnGetCache("Header").Value(GetGoroutineID())
	if found {
		h := headers.(map[string]string)
		headersMap := make(map[string]string)
		for k, v := range h {
			headersMap[k] = v
		}
		return headersMap
	} else {
		return map[string]string{}
	}
}

func generateRandString(source string, l int) string {
	bytes := []byte(source)
	result := []byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < l; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}
	return string(result)
}

func getRandomHexString(l int) string {
	str := "0123456789abcdef"
	return generateRandString(str, l)
}

func GetGoroutineID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}

func getHeaders(c *gin.Context) map[string]string {
	headers := make(map[string]string)
	for k, v := range c.Request.Header {
		headers[k] = v[0]
	}
	return headers
}

// 从其他协程克隆headers到当前协程的缓存
func CopyPreHeaderToCurRoutine(preRoutineId uint64) {
	headers, found := cache.OnGetCache("Header").Value(preRoutineId)
	if found {
		cache.OnGetCache("Header").Add(GetGoroutineID(), headers, 5*time.Minute)
	}
}
