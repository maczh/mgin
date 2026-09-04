package utils

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

func HmacSHA1(key string, data string) []byte {
	mac := hmac.New(sha1.New, []byte(key))
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func HmacSHA256(key string, data string) []byte {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func HmacSHA1Hex(key string, data string) string {
	return hex.EncodeToString(HmacSHA1(key, data))
}

func HmacSHA1Base64(key string, data string) string {
	return base64.StdEncoding.EncodeToString(HmacSHA1(key, data))
}

func HmacSHA256Hex(key string, data string) string {
	return hex.EncodeToString(HmacSHA256(key, data))
}

func HmacSHA256Base64(key string, data string) string {
	return base64.StdEncoding.EncodeToString(HmacSHA256(key, data))
}
