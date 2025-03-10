package utils

import (
	"github.com/mitchellh/mapstructure"
	"sort"
	"strings"
)

// map转换
func MapItoS(src map[string]any) map[string]string {
	dst := make(map[string]string)
	for k, v := range src {
		dst[k] = v.(string)
	}
	return dst
}

// map转换
func MapStoI(src map[string]string) map[string]any {
	dst := make(map[string]any)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func Exists(src map[string]string, key string) bool {
	_, ok := src[key]
	return ok
}

func Existi(src map[string]any, key string) bool {
	_, ok := src[key]
	return ok
}

// 按值排序
type Pair struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type PairList []Pair

func (p PairList) Len() int           { return len(p) }
func (p PairList) Less(i, j int) bool { return p[i].Value.(float64) < p[j].Value.(float64) }
func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func SortMapByValue(src map[string]any) PairList {
	list := make(PairList, len(src))
	i := 0
	for k, v := range src {
		list[i] = Pair{k, v}
		i++
	}
	sort.Sort(list)
	return list
}

func SortMapByValueDesc(src map[string]any) PairList {
	list := make(PairList, len(src))
	i := 0
	for k, v := range src {
		list[i] = Pair{k, v}
		i++
	}
	sort.Sort(sort.Reverse(list))
	return list
}

// Map2Struct Decode takes an input structure and uses reflection to translate it to
// Map 2Struct Decode获取一个输入结构，并使用反射将其转换为
// the output structure. output must be a pointer to a map or struct.
// 输出结构输出必须是指向map或struct的指针。
func Map2Struct(input interface{}, output interface{}) error {
	cfg := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc()),
		WeaklyTypedInput: true,
		Metadata:         nil,
		Result:           &output,
	}
	if d, err := mapstructure.NewDecoder(cfg); err != nil {
		return err
	} else if err := d.Decode(input); err != nil {
		return err
	}
	return nil
}

// Get 获取map中的字段，支持嵌套结构获取，例如fieldName.subFieldName.xx
// 嵌套类型必须是map[string]interface{}
// 如果字段不存在，返回nil
func MapGet(input interface{}, fieldName string) interface{} {
	// 按照"."分割fieldName
	fields := strings.Split(fieldName, ".")
	var result interface{}
	result = input

	// 遍历每个子字段
	for _, field := range fields {
		switch v := result.(type) {
		case map[string]interface{}:
			if val, ok := v[field]; ok {
				result = val
			} else {
				return nil
			}
		case map[string]string:
			if val, ok := v[field]; ok {
				result = val
			} else {
				return nil
			}
		default:
			return nil
		}
	}
	return result
}
