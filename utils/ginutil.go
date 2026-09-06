package utils

import (
	"github.com/gin-gonic/gin"
	"strings"
)

// firstValue 安全取切片首元素，避免输入异常时 index out of range
func firstValue(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return v[0]
}

// GinParamMap 获取请求参数，转成Map
func GinParamMap(c *gin.Context) map[string]string {
	params := make(map[string]string)
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return params
	}
	if c.Request.Method == "GET" {
		for k, v := range c.Request.URL.Query() {
			params[k] = firstValue(v)
		}
		return params
	} else if c.Request.Method == "POST" {
		if strings.Contains(c.ContentType(), "x-www-form-urlencoded") {
			_ = c.Request.ParseForm()
			for k, v := range c.Request.PostForm {
				params[k] = firstValue(v)
			}
			for k, v := range c.Request.URL.Query() {
				params[k] = firstValue(v)
			}
		} else if strings.Contains(c.ContentType(), "multipart/form-data") {
			// 解析失败（boundary 非法/body 超限/连接中断）时 MultipartForm 为 nil，必须判空
			if err := c.Request.ParseMultipartForm(100 << 20); err == nil && c.Request.MultipartForm != nil {
				for k, v := range c.Request.MultipartForm.Value {
					params[k] = firstValue(v)
				}
			}
			for k, v := range c.Request.URL.Query() {
				params[k] = firstValue(v)
			}
		}
	}
	return params
}

func GinHeaders(c *gin.Context) map[string]string {
	headers := make(map[string]string)
	if c == nil || c.Request == nil {
		return headers
	}
	for k, v := range c.Request.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	return headers
}
