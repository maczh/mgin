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
	requestId := c.GetHeader("X-Request-Id")
	if requestId == "" {
		requestId = getRandomHexString(16)
	}
	routineId := getGoroutineID()
	cache.OnGetCache("RequestId").Add(routineId, requestId, 5*time.Minute)
	clientIp := c.ClientIP()
	if c.GetHeader("X-Real-IP") != "" {
		clientIp = c.GetHeader("X-Real-IP")
	}
	if c.GetHeader("X-Forwarded-For") != "" {
		clientIp = c.GetHeader("X-Forwarded-For")
	}
	cache.OnGetCache("ClientIP").Add(routineId, clientIp, time.Minute)
	userAgent := c.GetHeader("X-User-Agent")
	if userAgent == "" {
		userAgent = c.GetHeader("User-Agent")
	}
	cache.OnGetCache("UserAgent").Add(requestId, userAgent, time.Minute)
}

func GetRequestId() string {
	requestId, found := cache.OnGetCache("RequestId").Value(getGoroutineID())
	if found {
		return requestId.(string)
	} else {
		return ""
	}
}

func GetClientIp() string {
	clientIp, found := cache.OnGetCache("ClientIP").Value(getGoroutineID())
	if found {
		return clientIp.(string)
	} else {
		return ""
	}
}

func GetUserAgent() string {
	userAgent, found := cache.OnGetCache("UserAgent").Value(getGoroutineID())
	if found {
		return userAgent.(string)
	} else {
		return ""
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

func getGoroutineID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}
