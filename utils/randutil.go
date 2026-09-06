package utils

import (
	"github.com/gofrs/uuid"
	"math/rand"
)

func GetRandomString(l int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyz"
	return GenerateRandString(str, l)
}

func GetRandomCaseString(l int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!@#$%&_="
	return GenerateRandString(str, l)
}

func GetRandomHexString(l int) string {
	str := "0123456789abcdef"
	return GenerateRandString(str, l)
}

func GetRandomIntString(l int) string {
	str := "0123456789"
	return GenerateRandString(str, l)
}

// randSource 全局随机源，避免每次调用都新建 Source（math/rand 全局函数已内置并发安全锁）
func GenerateRandString(source string, l int) string {
	if l <= 0 || source == "" {
		return ""
	}
	bytes := []byte(source)
	result := make([]byte, 0, l)
	for i := 0; i < l; i++ {
		result = append(result, bytes[rand.Intn(len(bytes))])
	}
	return string(result)
}

func GetUUIDString() string {
	u, _ := uuid.NewV4()
	return u.String()
}
