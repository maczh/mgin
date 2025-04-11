package utils

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/maczh/mgin/config"
	"time"
)

// GenerateToken 生成JWT token
func GenerateToken(claims jwt.MapClaims) (string, error) {
	// 设置过期时间
	claims["exp"] = time.Now().Add(time.Hour * 24).Unix()

	// 创建token对象
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 生成签名
	tokenStr, err := token.SignedString([]byte(config.Config.Jwt.Secret))
	if err != nil {
		return "", err
	}

	return tokenStr, nil
}

// ValidateToken 验证JWT token
func ValidateToken(tokenStr string) (*jwt.Token, error) {
	return jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.Config.Jwt.Secret), nil
	})
}
