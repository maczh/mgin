package utils

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	space = " "
)

// IsEmpty returns true if the string is empty
func IsEmpty(text string) bool {
	return len(text) == 0
}

// IsNotEmpty returns true if the string is not empty
func IsNotEmpty(text string) bool {
	return !IsEmpty(text)
}

// IsBlank returns true if the string is blank (all whitespace)
func IsBlank(text string) bool {
	return len(strings.TrimSpace(text)) == 0
}

// IsNotBlank returns true if the string is not blank
func IsNotBlank(text string) bool {
	return !IsBlank(text)
}

func Left(src string, size int) string {
	return src[:size]
}

func Right(src string, size int) string {
	return src[len(src)-size:]
}

// Left justifies the text to the left
func LeftJustin(text string, size int) string {
	spaces := size - Length(text)
	if spaces <= 0 {
		return text
	}

	var buffer bytes.Buffer
	buffer.WriteString(text)

	for i := 0; i < spaces; i++ {
		buffer.WriteString(space)
	}
	return buffer.String()
}

// Right justifies the text to the right
func RightJustin(text string, size int) string {
	spaces := size - Length(text)
	if spaces <= 0 {
		return text
	}

	var buffer bytes.Buffer
	for i := 0; i < spaces; i++ {
		buffer.WriteString(space)
	}

	buffer.WriteString(text)
	return buffer.String()
}

// Center justifies the text in the center
func CenterJustin(text string, size int) string {
	left := RightJustin(text, (Length(text)+size)/2)
	return LeftJustin(left, size)
}

// Length counts the input while respecting UTF8 encoding and combined characters
func Length(text string) int {
	textRunes := []rune(text)
	textRunesLength := len(textRunes)

	sum, i, j := 0, 0, 0
	for i < textRunesLength && j < textRunesLength {
		j = i + 1
		for j < textRunesLength && IsMark(textRunes[j]) {
			j++
		}
		sum++
		i = j
	}
	return sum
}

// IsMark determines whether the rune is a marker
func IsMark(r rune) bool {
	return unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Mc, r)
}

// AddStringBytes 拼接字符串, 返回 bytes from bytes.Join()
func AddStringBytes(s ...string) []byte {
	switch len(s) {
	case 0:
		return []byte{}
	case 1:
		return []byte(s[0])
	}

	n := 0
	for _, v := range s {
		n += len(v)
	}

	b := make([]byte, n)
	bp := copy(b, s[0])
	for _, v := range s[1:] {
		bp += copy(b[bp:], v)
	}

	return b
}

// AddString 拼接字符串
func AddString(s ...string) string {
	return string(AddStringBytes(s...))
}

// IsNumeric returns true if the given character is a numeric, otherwise false.
func IsNumeric(c byte) bool {
	return c >= '0' && c <= '9'
}

// IsAlphabet char
func IsAlphabet(char uint8) bool {
	// A 65 -> Z 90
	if char >= 'A' && char <= 'Z' {
		return true
	}

	// a 97 -> z 122
	if char >= 'a' && char <= 'z' {
		return true
	}

	return false
}

// IsAlphaNum reports whether the byte is an ASCII letter, number, or underscore
func IsAlphaNum(c uint8) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// StrPos alias of the strings.Index
func StrPos(s, sub string) int {
	return strings.Index(s, sub)
}

// BytePos alias of the strings.IndexByte
func BytePos(s string, bt byte) int {
	return strings.IndexByte(s, bt)
}

// RunePos alias of the strings.IndexRune
func RunePos(s string, ru rune) int {
	return strings.IndexRune(s, ru)
}

// IsStartOf alias of the strings.HasPrefix
func IsStartOf(s, sub string) bool {
	return strings.HasPrefix(s, sub)
}

// IsEndOf alias of the strings.HasSuffix
func IsEndOf(s, sub string) bool {
	return strings.HasSuffix(s, sub)
}

// Utf8Len of the string
func Utf8len(s string) int {
	return utf8.RuneCount([]byte(s))
}

// ValidUtf8String check
func ValidUtf8String(s string) bool {
	return utf8.ValidString(s)
}

var spaceTable = [256]int8{0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

// IsSpace returns true if the given character is a space, otherwise false.
func IsSpace(c byte) bool {
	return spaceTable[c] == 1
}

// IsSpaceRune returns true if the given rune is a space, otherwise false.
func IsSpaceRune(r rune) bool {
	return r <= 256 && IsSpace(byte(r)) || unicode.IsSpace(r)
}

// IsBlankBytes returns true if the given []byte is all space characters.
func IsBlankBytes(bs []byte) bool {
	for _, b := range bs {
		if !IsSpace(b) {
			return false
		}
	}
	return true
}

func Lowercase(s string) string {
	return strings.ToLower(s)
}

// Uppercase alias of the strings.ToUpper()
func Uppercase(s string) string {
	return strings.ToUpper(s)
}

// UpperWord Change the first character of each word to uppercase
func UpperWord(s string) string {
	if len(s) == 0 {
		return s
	}

	if len(s) == 1 {
		return strings.ToUpper(s)
	}

	inWord := true
	buf := make([]byte, 0, len(s))

	i := 0
	rs := []rune(s)
	if runeIsLowerChar(rs[i]) {
		buf = append(buf, []byte(string(unicode.ToUpper(rs[i])))...)
	} else {
		buf = append(buf, []byte(string(rs[i]))...)
	}

	for j := i + 1; j < len(rs); j++ {
		if !runeIsWord(rs[i]) && runeIsWord(rs[j]) {
			inWord = false
		}

		if runeIsLowerChar(rs[j]) && !inWord {
			buf = append(buf, []byte(string(unicode.ToUpper(rs[j])))...)
			inWord = true
		} else {
			buf = append(buf, []byte(string(rs[j]))...)
		}

		if runeIsWord(rs[j]) {
			inWord = true
		}

		i++
	}

	return string(buf)
}

// LowerFirst lower first char
func LowerFirst(s string) string {
	if len(s) == 0 {
		return s
	}

	rs := []rune(s)
	f := rs[0]
	if 'A' <= f && f <= 'Z' {
		return string(unicode.ToLower(f)) + string(rs[1:])
	}

	return s
}

// UpperFirst upper first char
func UpperFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	rs := []rune(s)
	f := rs[0]
	if 'a' <= f && f <= 'z' {
		return string(unicode.ToUpper(f)) + string(rs[1:])
	}

	return s
}

func runeIsWord(c rune) bool {
	return runeIsLowerChar(c) || runeIsUpperChar(c)
}

func runeIsLowerChar(c rune) bool {
	return 'a' <= c && c <= 'z'
}

func runeIsUpperChar(c rune) bool {
	return 'A' <= c && c <= 'Z'
}

func ReplacePunctuationWithSpace(src string) string {
	reg1 := regexp.MustCompile(`[\f\t\n\r\v\-\^\$\.\*+\?{}()\/\[\]\|]`)
	reg2 := regexp.MustCompile(`[\s\p{Zs}]{2,}`)
	txt := reg2.ReplaceAll(reg1.ReplaceAll([]byte(src), []byte(" ")), []byte(" "))
	return string(txt)
}

func AddSpaceBetweenCharsAndNumbers(src string) string {
	var result []rune
	isNumber := false
	isChinese := false
	reg, _ := regexp.Compile("[^\\w\u4e00-\u9fa5]")
	src = reg.ReplaceAllString(src, " ")
	for i, s := range []rune(src) {
		if s >= '0' && s <= '9' {
			if !isNumber && i > 0 {
				result = append(result, ' ')
			}
			isNumber = true
		} else if isNumber {
			result = append(result, ' ')
			isNumber = false
		}
		if (unicode.Is(unicode.Han, s) && !isChinese) || (!unicode.Is(unicode.Han, s) && isChinese) {
			result = append(result, ' ')
			isChinese = !isChinese
		}
		result = append(result, s)
	}
	reg = regexp.MustCompile(`[\s\p{Zs}]{2,}`)
	return strings.TrimSpace(string(reg.ReplaceAll([]byte(string(result)), []byte(" "))))
}

func ReplacePunctuation(src, replaceWith string) string {
	reg, _ := regexp.Compile("[^\\w\u4e00-\u9fa5]*")
	return reg.ReplaceAllString(src, replaceWith)
}

func AnyToString(i any) (string, error) {
	switch s := i.(type) {
	case string:
		return s, nil
	case bool:
		return strconv.FormatBool(s), nil
	case float64:
		return strconv.FormatFloat(s, 'f', -1, 64), nil
	case float32:
		return strconv.FormatFloat(float64(s), 'f', -1, 32), nil
	case int:
		return strconv.Itoa(s), nil
	case int64:
		return strconv.FormatInt(s, 10), nil
	case int32:
		return strconv.Itoa(int(s)), nil
	case int16:
		return strconv.FormatInt(int64(s), 10), nil
	case int8:
		return strconv.FormatInt(int64(s), 10), nil
	case uint:
		return strconv.FormatUint(uint64(s), 10), nil
	case uint64:
		return strconv.FormatUint(uint64(s), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(s), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(s), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(s), 10), nil
	case []byte:
		return string(s), nil
	case template.HTML:
		return string(s), nil
	case template.URL:
		return string(s), nil
	case template.JS:
		return string(s), nil
	case template.CSS:
		return string(s), nil
	case template.HTMLAttr:
		return string(s), nil
	case nil:
		return "", nil
	case fmt.Stringer:
		return s.String(), nil
	case error:
		return s.Error(), nil
	default:
		return ToJSON(i), nil
	}
}

func StringToAny[T any](src string) (T, error) {
	var t T
	switch any(t).(type) {
	case bool:
		v, err := strconv.ParseBool(src)
		if err != nil {
			return t, err
		}
		t = any(v).(T)
	case int:
		v, err := strconv.Atoi(src)
		if err != nil {
			return t, err
		}
		t = any(v).(T)
	case int8:
		v, err := strconv.ParseInt(src, 10, 8)
		if err != nil {
			return t, err
		}
		t = any(int8(v)).(T)
	case int16:
		v, err := strconv.ParseInt(src, 10, 16)
		if err != nil {
			return t, err
		}
		t = any(int16(v)).(T)
	case int32:
		v, err := strconv.ParseInt(src, 10, 32)
		if err != nil {
			return t, err
		}
		t = any(int32(v)).(T)
	case int64:
		v, err := strconv.ParseInt(src, 10, 64)
		if err != nil {
			return t, err
		}
		t = any(v).(T)
	case uint:
		v, err := strconv.ParseInt(src, 10, 64)
		if err != nil {
			return t, err
		}
		t = any(uint(v)).(T)
	case uint8:
		v, err := strconv.ParseInt(src, 10, 8)
		if err != nil {
			return t, err
		}
		t = any(uint8(v)).(T)
	case uint16:
		v, err := strconv.ParseInt(src, 10, 16)
		if err != nil {
			return t, err
		}
		t = any(uint16(v)).(T)
	case uint32:
		v, err := strconv.ParseInt(src, 10, 32)
		if err != nil {
			return t, err
		}
		t = any(uint32(v)).(T)
	case uint64:
		v, err := strconv.ParseInt(src, 10, 64)
		if err != nil {
			return t, err
		}
		t = any(uint64(v)).(T)
	case float32:
		v, err := strconv.ParseFloat(src, 32)
		if err != nil {
			return t, err
		}
		t = any(float32(v)).(T)
	case float64:
		v, err := strconv.ParseFloat(src, 64)
		if err != nil {
			return t, err
		}
		t = any(v).(T)
	case string:
		t = any(src).(T)
	default:
		FromJSON(src, &t)
	}
	return t, nil
}
