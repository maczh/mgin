package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/service"
)

type captchaController struct{}

// GetCaptcha 获取验证码
// @Summary 获取验证码
// @Description 获取图片验证码，返回验证码 ID 和 Base64 编码的图片
// @Tags 验证码
// @Accept  json
// @Produce  json
// @Param   body body request.GetCaptchaReq true "获取验证码请求参数"
// @Success 200 {object} models.Result[map[string]string] "成功返回验证码信息"
// @Failure 500 {object} models.Result[any] "参数绑定失败或获取验证码失败"
// @Router /captcha/get [post]
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

// VerifyCaptcha 验证验证码
// @Summary 验证验证码
// @Description 验证用户输入的验证码是否正确
// @Tags 验证码
// @Accept  json
// @Produce  json
// @Param   body body request.VerifyCaptchaReq true "验证验证码请求参数"
// @Success 200 {object} models.Result[any] "验证码验证成功"
// @Failure 500 {object} models.Result[any] "参数绑定失败或验证码错误"
// @Router /captcha/verify [post]
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
