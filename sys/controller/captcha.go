package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/service"
)

type captchaController struct{}

// GetCaptcha is a method of the captchaController struct that returns a models.Result[any] type
func (c *captchaController) GetCaptcha(g *gin.Context) models.Result[any] {
	var req request.GetCaptchaReq
	if err := g.ShouldBind(&req); err != nil { // 绑定请求参数
		return models.ErrorT[any](500, "参数绑定失败")
	}
	id, imgBase64 := service.Captcha.GetCaptcha(req)
	if id == "" || imgBase64 == "" {
		return models.ErrorT[any](500, "获取验证码失败")
	}
	return models.Success[any](map[string]string{
		"id":        id,
		"imgBase64": imgBase64,
	})
}

// VerifyCaptcha is a method of the captchaController struct that returns a models.Result[any] type
func (c *captchaController) VerifyCaptcha(g *gin.Context) models.Result[any] {
	var req request.VerifyCaptchaReq
	if err := g.ShouldBind(&req); err != nil { // 绑定请求参数
		return models.ErrorT[any](500, "参数绑定失败")
	}
	if service.Captcha.VerifyCaptcha(req) { // 验证验证码
		return models.Success[any](nil)
	}
	return models.ErrorT[any](500, "验证码错误")
}
