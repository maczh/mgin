package utils

import (
	"strconv"
	"strings"
)

func StringArrayContains(src []string, dst string) bool {
	if src == nil || len(src) == 0 {
		return false
	}
	for _, str := range src {
		if str == dst {
			return true
		}
	}
	return false
}

func StringArrayDelete(src []string, index int) []string {
	dst := append(src[:index], src[index+1:]...)
	return dst
}

func IntArrayContains(src []int, dst int) bool {
	if src == nil || len(src) == 0 {
		return false
	}
	for _, s := range src {
		if s == dst {
			return true
		}
	}
	return false
}

func IntArrayDelete(src []int, index int) []int {
	dst := append(src[:index], src[index+1:]...)
	return dst
}

func Float64ArrayContains(src []float64, dst float64) bool {
	if src == nil || len(src) == 0 {
		return false
	}
	for _, s := range src {
		if s == dst {
			return true
		}
	}
	return false
}

func Float64ArrayDelete(src []float64, index int) []float64 {
	dst := append(src[:index], src[index+1:]...)
	return dst
}

func SliceContains(sl []interface{}, v interface{}) bool {
	for _, vv := range sl {
		if vv == v {
			return true
		}
	}
	return false
}

func SliceContainsInt(sl []int, v int) bool {
	for _, vv := range sl {
		if vv == v {
			return true
		}
	}
	return false
}

func SliceContainsInt64(sl []int64, v int64) bool {
	for _, vv := range sl {
		if vv == v {
			return true
		}
	}
	return false
}

func SliceContainsString(sl []string, v string) bool {
	for _, vv := range sl {
		if vv == v {
			return true
		}
	}
	return false
}

// SliceMerge merges interface slices to one slice.
func SliceMerge(slice1, slice2 []interface{}) (c []interface{}) {
	c = append(slice1, slice2...)
	return
}

func SliceMergeInt(slice1, slice2 []int) (c []int) {
	c = append(slice1, slice2...)
	return
}

func SliceMergeInt64(slice1, slice2 []int64) (c []int64) {
	c = append(slice1, slice2...)
	return
}

func SliceMergeString(slice1, slice2 []string) (c []string) {
	c = append(slice1, slice2...)
	return
}

func SliceUniqueInt64(s []int64) []int64 {
	size := len(s)
	if size == 0 {
		return []int64{}
	}

	m := make(map[int64]bool)
	for i := 0; i < size; i++ {
		m[s[i]] = true
	}

	realLen := len(m)
	ret := make([]int64, realLen)

	idx := 0
	for key := range m {
		ret[idx] = key
		idx++
	}

	return ret
}

func SliceUniqueInt(s []int) []int {
	size := len(s)
	if size == 0 {
		return []int{}
	}

	m := make(map[int]bool)
	for i := 0; i < size; i++ {
		m[s[i]] = true
	}

	realLen := len(m)
	ret := make([]int, realLen)

	idx := 0
	for key := range m {
		ret[idx] = key
		idx++
	}

	return ret
}

func SliceUniqueString(s []string) []string {
	size := len(s)
	if size == 0 {
		return []string{}
	}

	m := make(map[string]bool)
	for i := 0; i < size; i++ {
		m[s[i]] = true
	}

	realLen := len(m)
	ret := make([]string, realLen)

	idx := 0
	for key := range m {
		ret[idx] = key
		idx++
	}

	return ret
}

func SliceSumInt64(intslice []int64) (sum int64) {
	for _, v := range intslice {
		sum += v
	}
	return
}

func SliceSumInt(intslice []int) (sum int) {
	for _, v := range intslice {
		sum += v
	}
	return
}

func SliceSumFloat64(intslice []float64) (sum float64) {
	for _, v := range intslice {
		sum += v
	}
	return
}

//删除数组
func DeleteArray(src []interface{}, index int) (result []interface{}) {
	result = append(src[:index], src[(index+1):]...)
	return
}

// []string => []int
func ArrayStr2Int(data []string) []int {
	var (
		arr = make([]int, 0, len(data))
	)
	if len(data) == 0 {
		return arr
	}
	for i, _ := range data {
		var num, _ = strconv.Atoi(data[i])
		arr = append(arr, num)
	}
	return arr
}

// []int => []string
func ArrayInt2Str(data []int) []string {
	var (
		arr = make([]string, 0, len(data))
	)
	if len(data) == 0 {
		return arr
	}
	for i, _ := range data {
		arr = append(arr, strconv.Itoa(data[i]))
	}
	return arr
}

// str[TrimSpace] in string list
func TrimSpaceStrInArray(str string, data []string) bool {
	if len(data) > 0 {
		for _, row := range data {
			if str == strings.TrimSpace(row) {
				return true
			}
		}
	}
	return false
}

// StringUnique 去重
func StringUnique(a []string) []string {
	num := len(a)

	if num <= 0 {
		return a
	}
	out := make([]string, 0, num)
	exists := make(map[string]bool, num)
	for _, item := range a {
		if exists[item] {
			continue
		}
		exists[item] = true
		out = append(out, item)
	}

	return out
}

// Int64Unique 去重
func Int64Unique(a []int64) []int64 {
	num := len(a)

	if num <= 0 {
		return a
	}
	out := make([]int64, 0, num)
	exists := make(map[int64]bool, num)
	for _, item := range a {
		if exists[item] {
			continue
		}
		exists[item] = true
		out = append(out, item)
	}

	return out
}

func UnSplitString(src []string, sep string) string {
	dst := ""
	for _, item := range src {
		dst = dst + item + sep
	}
	return dst[:len(dst)-1]
}

func UnionStringSlice(slice1, slice2 []string) []string {
	m := make(map[string]int)
	for _, v := range slice1 {
		m[v]++
	}

	for _, v := range slice2 {
		times, _ := m[v]
		if times == 0 {
			slice1 = append(slice1, v)
		}
	}
	return slice1
}

//求交集
func IntersectStringSlice(slice1, slice2 []string) []string {
	m := make(map[string]int)
	nn := make([]string, 0)
	for _, v := range slice1 {
		m[v]++
	}

	for _, v := range slice2 {
		times, _ := m[v]
		if times == 1 {
			nn = append(nn, v)
		}
	}
	return nn
}

//求差集 slice1-并集
func DifferenceStringSlice(slice1, slice2 []string) []string {
	m := make(map[string]int)
	nn := make([]string, 0)
	inter := IntersectStringSlice(slice1, slice2)
	for _, v := range inter {
		m[v]++
	}

	for _, value := range slice1 {
		times, _ := m[value]
		if times == 0 {
			nn = append(nn, value)
		}
	}
	return nn
}
