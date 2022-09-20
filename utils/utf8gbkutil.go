package utils

import (
	"github.com/henrylee2cn/mahonia"
	"github.com/maczh/mgin/logs"
	"golang.org/x/text/encoding/simplifiedchinese"
	"io/ioutil"
	"strings"
)

func Utf8ToGbk(uft8Str string) string {
	return mahonia.NewEncoder("gbk").ConvertString(uft8Str)
}

func GbkToUtf8(gbkStr string) (string, error) {
	reader := simplifiedchinese.GBK.NewDecoder().Reader(strings.NewReader(gbkStr))
	buf, err := ioutil.ReadAll(reader)
	if err != nil {
		logs.Error("从GBK转换UTF8错误:{}", err.Error())
		return "", err
	} else {
		return string(buf), nil
	}

}

func ClearUtf8BOM(bomStr string) string {
	dat := []byte(bomStr)
	out := ""
	if len(dat) > 3 && (dat[0] == 0xef || dat[1] == 0xbb || dat[2] == 0xbf) {
		out = bomStr[3:]
		out = strings.ReplaceAll(out, "\r", "")
	} else {
		out = bomStr
	}
	return out
}
