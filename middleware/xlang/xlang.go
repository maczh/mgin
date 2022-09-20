package xlang

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/cache"
	"github.com/maczh/mgin/utils"
	"time"
)

func RequestLanguage() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := c.GetHeader("X-Lang")
		if lang == "" {
			lang = "zh-cn"
		}
		routineId := utils.GetGoroutineID()
		cache.OnGetCache("Lang").Add(routineId, lang, 5*time.Minute)
	}
}

func GetCurrentLanguage() string {
	lang, found := cache.OnGetCache("Lang").Value(utils.GetGoroutineID())
	if found {
		return lang.(string)
	} else {
		return "zh-cn"
	}
}
