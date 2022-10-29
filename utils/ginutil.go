package utils

import "github.com/gin-gonic/gin"

//获取请求参数，转成Map
func GinParamMap(c *gin.Context) map[string]string {
	params := make(map[string]string)
	if c.Request.Method == "GET" {
		for k, v := range c.Request.URL.Query() {
			params[k] = v[0]
		}
		return params
	} else {
		c.Request.ParseForm()
		for k, v := range c.Request.PostForm {
			params[k] = v[0]
		}
		for k, v := range c.Request.URL.Query() {
			params[k] = v[0]
		}
		return params
	}
}

func GinHeaders(c *gin.Context) map[string]string {
	headers := make(map[string]string)
	for k, v := range c.Request.Header {
		headers[k] = v[0]
	}
	return headers
}
