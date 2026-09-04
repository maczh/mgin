package utils

import "strings"

// CheckSqlValidate 检查是否含有可能产生注入的非法字符
// 返回值为true时表示含有非法字符，同时返回的字符串值为匹配到的非法字符
func CheckSqlValidate(content string) (bool, string) {
	if content == "" {
		return false, ""
	}
	filterString := `exec|execute|insert|select|delete|update|drop|*|chr|mid|master|truncate|
		char|declare|sitename|net user|xp_cmdshell|;|+|create|
		table|from|grant|use|group_concat|column_name|
		information_schema.columns|table_schema|union|where|order|by|count|
		--|//|/|#|or 1 = 1|or '|' or|or'|'or|)or|) or|or(|or (| or|or | and|and |)and|and(|,and|and'`

	arr := strings.Split(filterString, "|")

	for _, s := range arr {
		if index := strings.Index(content, s); index > 0 {
			return true, s
		}
	}
	return false, ""
}
