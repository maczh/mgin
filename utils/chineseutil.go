package utils

import (
	"unicode"
)

func SubChineseString(str string, begin, length int) string {
	// 将字符串的转换成[]rune
	rs := []rune(str)
	lth := len(rs)

	// 简单的越界判断
	if begin < 0 {
		begin = 0
	}
	if begin >= lth {
		begin = lth
	}
	end := begin + length
	if end > lth {
		end = lth
	}
	// 返回子串
	if length >= 0 {
		return string(rs[begin:end])
	} else {
		return string(rs[begin:])
	}
}

func ChineseLength(str string) int {
	return len([]rune(str))
}

func IsChinese(str string) bool {
	for _, r := range str {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
	//isChinese := regexp.MustCompile("^[\u4e00-\u9fa5]")
	//return isChinese.MatchString(str)
}

//全角转半角
func DBCtoSBC(s string) string {
	retstr := ""
	for _, i := range s {
		inside_code := i
		if inside_code == 12288 {
			inside_code = 32
		} else {
			inside_code -= 65248
		}
		if inside_code < 32 || inside_code > 126 {
			retstr += string(i)
		} else {
			retstr += string(inside_code)
		}
	}
	return retstr
}
