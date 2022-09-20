package utils

import (
	"crypto/md5"
	"fmt"
	"github.com/emirpasic/gods/maps/treemap"
	"io"
	"os"
	"strings"
)

func MD5Encode(content string) (md string) {
	h := md5.New()
	_, _ = io.WriteString(h, content)
	md = fmt.Sprintf("%x", h.Sum(nil))
	return
}

//计算文件的MD5值
func FileMD5(filename string) (string, error) {
	f, err := os.Open(filename) //打开文件
	if nil != err {
		fmt.Println(err)
		return "", err
	}
	defer f.Close()

	h := md5.New()         //创建 md5 句柄
	_, err = io.Copy(h, f) //将文件内容拷贝到 md5 句柄中
	if nil != err {
		fmt.Println(err)
		return "", err
	}
	md := h.Sum(nil)                //计算 MD5 值，返回 []byte
	md5str := fmt.Sprintf("%x", md) //将 []byte 转为 string
	return md5str, nil
}

func MapMD5(m map[string]string) string {
	sortmap := treemap.NewWithStringComparator()
	for k, v := range m {
		sortmap.Put(k, v)
	}
	signtext := ""
	sortmap.Each(func(key interface{}, value interface{}) {
		if key.(string) != "sign" && m[key.(string)] != "" {
			signtext = signtext + key.(string) + "=" + value.(string) + "&"
		}
	})
	signtext = strings.TrimRight(signtext, "&")
	return MD5Encode(signtext)
}
