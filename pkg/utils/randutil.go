package utils

import (
	"github.com/gofrs/uuid"
	"math/rand"
	"time"
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

func GenerateRandString(source string, l int) string {
	bytes := []byte(source)
	result := []byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < l; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}
	return string(result)
}

func GetUUIDString() string {
	u, _ := uuid.NewV4()
	return u.String()
}
