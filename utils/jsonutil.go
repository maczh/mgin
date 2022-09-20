package utils

import (
	"bytes"
	j "encoding/json"
	jsoniter "github.com/json-iterator/go"
	"github.com/maczh/mgin/logs"
	"strings"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func ToJSON(o interface{}) string {
	j, err := json.Marshal(o)
	if err != nil {
		return "{}"
	} else {
		js := string(j)
		js = strings.Replace(js, "\\u003c", "<", -1)
		js = strings.Replace(js, "\\u003e", ">", -1)
		js = strings.Replace(js, "\\u0026", "&", -1)
		return js
	}
}

func FromJSON(j string, o interface{}) *interface{} {
	err := json.Unmarshal([]byte(j), &o)
	if err != nil {
		logs.Error("数据转换错误:{}", err.Error())
		return nil
	} else {
		return &o
	}
}

// JSONPrettyPrint pretty print raw json string to indent string
func JSONPretty(in, prefix, indent string) string {
	var out bytes.Buffer
	if err := j.Indent(&out, []byte(in), prefix, indent); err != nil {
		return in
	}
	return out.String()
}

// CompactJSON compact json input with insignificant space characters elided
func CompactJSON(in string) string {
	var out bytes.Buffer
	if err := j.Compact(&out, []byte(in)); err != nil {
		return in
	}
	return out.String()
}
