package utils

import "sort"

//map转换
func MapItoS(src map[string]interface{}) map[string]string {
	dst := make(map[string]string)
	for k, v := range src {
		dst[k] = v.(string)
	}
	return dst
}

//map转换
func MapStoI(src map[string]string) map[string]interface{} {
	dst := make(map[string]interface{})
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func Exists(src map[string]string, key string) bool {
	_, ok := src[key]
	return ok
}

func Existi(src map[string]interface{}, key string) bool {
	_, ok := src[key]
	return ok
}

//按值排序
type Pair struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

type PairList []Pair

func (p PairList) Len() int           { return len(p) }
func (p PairList) Less(i, j int) bool { return p[i].Value.(float64) < p[j].Value.(float64) }
func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func SortMapByValue(src map[string]interface{}) PairList {
	list := make(PairList, len(src))
	i := 0
	for k, v := range src {
		list[i] = Pair{k, v}
		i++
	}
	sort.Sort(list)
	return list
}

func SortMapByValueDesc(src map[string]interface{}) PairList {
	list := make(PairList, len(src))
	i := 0
	for k, v := range src {
		list[i] = Pair{k, v}
		i++
	}
	sort.Sort(sort.Reverse(list))
	return list
}
