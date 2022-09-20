package trace

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/cache"
	"github.com/maczh/mgin/utils"
	"math/rand"
	"time"
)

func PutRequestId(c *gin.Context) {
	requestId := c.GetHeader("X-Request-Id")
	if requestId == "" {
		requestId = getRandomHexString(16)
	}
	routineId := utils.GetGoroutineID()
	cache.OnGetCache("RequestId").Add(routineId, requestId, 5*time.Minute)
}

func GetRequestId() string {
	requestId, found := cache.OnGetCache("RequestId").Value(utils.GetGoroutineID())
	if found {
		return requestId.(string)
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
