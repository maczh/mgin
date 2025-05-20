package service

import (
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/mojocn/base64Captcha"
	"math/rand"
	"time"
)

type captchaService struct{}

// 获取验证码图片
func (s captchaService) GetCaptcha(req request.GetCaptchaReq) (string, string) {
	// 初始化随机数种子
	rand.Seed(time.Now().UnixNano())
	var driver base64Captcha.Driver
	switch req.Type {
	case 0:
		driver = &base64Captcha.DriverDigit{
			Height:   req.Height,
			Width:    req.Width,
			Length:   req.Length,
			MaxSkew:  0.7,
			DotCount: 80,
		}
	case 1:
		driver = &base64Captcha.DriverString{
			Height:          req.Height,
			Width:           req.Width,
			Length:          req.Length,
			Source:          "1234567890qwertyuiopasdfghjklzxcvbnmQWERTYUIOPASDFGHJKLZXCVBNM",
			NoiseCount:      0,
			ShowLineOptions: base64Captcha.OptionShowHollowLine | base64Captcha.OptionShowSlimeLine | base64Captcha.OptionShowSineLine,
		}
	case 2:
		driver = &base64Captcha.DriverMath{
			Height:          req.Height,
			Width:           req.Width,
			NoiseCount:      0,
			ShowLineOptions: base64Captcha.OptionShowSlimeLine | base64Captcha.OptionShowSineLine,
		}
	}
	cp := base64Captcha.NewCaptcha(driver, base64Captcha.DefaultMemStore)

	// 生成验证码
	id, b64s, _, err := cp.Generate()
	if err != nil {
		logs.Error("generate captcha error: {}", err.Error())
		return "", ""
	}

	return id, b64s
}

// 验证验证码
func (s captchaService) VerifyCaptcha(req request.VerifyCaptchaReq) bool {
	return base64Captcha.DefaultMemStore.Verify(req.Id, req.Code, true)
}
