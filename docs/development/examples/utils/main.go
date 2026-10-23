// Package main 演示常用工具函数（无外部依赖）。
// 运行：go run ./utils
package main

import (
	"fmt"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/maczh/mgin/v2/pkg/utils"
)

func main() {
	fmt.Println("md5:", utils.MD5Encode("hello"))
	fmt.Println("sha256:", utils.Sha256("hello"))
	fmt.Println("base64:", utils.Base64Encode("hello"))

	enc, err := utils.AESBase64Encrypt("secret", "0123456789abcdef", []byte("abcdefghijklmnop"))
	fmt.Println("aes encrypt err:", err)
	if err == nil {
		dec, err := utils.AESBase64Decrypt(enc, "0123456789abcdef", []byte("abcdefghijklmnop"))
		fmt.Println("aes decrypt:", dec, "err:", err)
	}

	token, err := utils.GenerateToken(jwt.MapClaims{"userId": 1001, "roleId": 2})
	fmt.Println("jwt err:", err, "token prefix:", prefix(token, 20))

	parsed, err := utils.ValidateToken(token)
	if err != nil {
		fmt.Println("jwt validate err:", err)
	} else if claims, ok := parsed.Claims.(jwt.MapClaims); ok {
		fmt.Println("jwt userId:", claims["userId"])
	}

	fmt.Println("uuid:", utils.NewUUIDString())
	fmt.Println("random:", utils.GetRandomString(8))
	fmt.Println("is intranet 10.0.0.1:", utils.IsIntranetIP("10.0.0.1"))
}

func prefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
