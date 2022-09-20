package utils

import (
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"github.com/maczh/mgin/logs"
	"io/ioutil"
)

func Sha1(src string) string {
	h := sha1.New()
	h.Write([]byte(src))
	bs := h.Sum(nil)
	return fmt.Sprintf("%x", bs)
}

func Sha256(src string) string {
	sha256Hash := sha256.New()
	s_data := []byte(src)
	sha256Hash.Write(s_data)
	hashed := sha256Hash.Sum(nil)
	return fmt.Sprintf("%x", hashed)
}

func FileSha256(filename string) (string, error) {
	src, err := ioutil.ReadFile(filename)
	if err != nil {
		logs.Error("文件读取错误:{}", err.Error())
		return "", err
	}
	sha256Hash := sha256.New()
	sha256Hash.Write(src)
	hashed := sha256Hash.Sum(nil)
	return fmt.Sprintf("%x", hashed), nil
}
