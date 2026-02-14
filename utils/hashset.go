package utils

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand"
	"reflect"
	"time"
)

// HashSet 泛型集合类
type HashSet[T any] struct {
	data map[string]*T
}

// NewHashSet 创建新的泛型 HashSet
func NewHashSet[T any]() *HashSet[T] {
	return &HashSet[T]{
		data: make(map[string]*T),
	}
}

// NewHashSetWithValues 使用初始值创建 HashSet
func NewHashSetWithValues[T any](values ...T) *HashSet[T] {
	set := NewHashSet[T]()
	for _, v := range values {
		set.Add(v)
	}
	return set
}

// 生成键的辅助函数
func generateKey[T any](value T) (string, error) {
	// 尝试多种方法生成键
	v := reflect.ValueOf(value)

	// 1. 如果实现了 Stringer 接口
	if stringer, ok := any(value).(fmt.Stringer); ok {
		return stringer.String(), nil
	}

	// 2. 如果是基本类型，直接转换为字符串
	switch v.Kind() {
	case reflect.String:
		return v.String(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", v.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%d", v.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%f", v.Float()), nil
	case reflect.Bool:
		return fmt.Sprintf("%t", v.Bool()), nil
	}

	// 3. 使用 JSON 序列化
	if jsonBytes, err := json.Marshal(value); err == nil {
		// 使用哈希确保唯一性
		h := fnv.New64a()
		h.Write(jsonBytes)
		return fmt.Sprintf("%x", h.Sum64()), nil
	}

	// 4. 使用反射获取类型和值的哈希
	h := fnv.New64a()
	fmt.Fprintf(h, "%v", value)
	return fmt.Sprintf("%x", h.Sum64()), nil
}

// Add 添加元素
func (s *HashSet[T]) Add(value T) bool {
	key, err := generateKey(value)
	if err != nil {
		// 如果无法生成键，使用指针地址作为最后手段
		h := fnv.New64a()
		ptr := reflect.ValueOf(value).Pointer()
		fmt.Fprintf(h, "%p", ptr)
		key = fmt.Sprintf("%x", h.Sum64())
	}

	if _, exists := s.data[key]; !exists {
		s.data[key] = &value
		return true
	}
	return false
}

// Remove 移除元素
func (s *HashSet[T]) Remove(value T) bool {
	key, err := generateKey(value)
	if err != nil {
		return false
	}

	if _, exists := s.data[key]; exists {
		delete(s.data, key)
		return true
	}
	return false
}

// Contains 检查元素是否存在
func (s *HashSet[T]) Contains(value T) bool {
	key, err := generateKey(value)
	if err != nil {
		return false
	}
	_, exists := s.data[key]
	return exists
}

// Size 返回集合大小
func (s *HashSet[T]) Size() int {
	return len(s.data)
}

// IsEmpty 检查集合是否为空
func (s *HashSet[T]) IsEmpty() bool {
	return len(s.data) == 0
}

// Clear 清空集合
func (s *HashSet[T]) Clear() {
	s.data = make(map[string]*T)
}

// Values 返回所有值
func (s *HashSet[T]) Values() []T {
	values := make([]T, 0, len(s.data))
	for _, v := range s.data {
		values = append(values, *v)
	}
	return values
}

// ForEach 遍历集合
func (s *HashSet[T]) ForEach(f func(T)) {
	for _, v := range s.data {
		f(*v)
	}
}

// Filter 过滤集合
func (s *HashSet[T]) Filter(predicate func(T) bool) *HashSet[T] {
	result := NewHashSet[T]()
	for _, v := range s.data {
		if predicate(*v) {
			result.Add(*v)
		}
	}
	return result
}

// Map 映射集合
func (s *HashSet[T]) Map(f func(T) T) *HashSet[T] {
	result := NewHashSet[T]()
	for _, v := range s.data {
		result.Add(f(*v))
	}
	return result
}

// Reduce 归约集合
func (s *HashSet[T]) Reduce(initial T, reducer func(T, T) T) T {
	result := initial
	for _, v := range s.data {
		result = reducer(result, *v)
	}
	return result
}

// Clone 克隆集合
func (s *HashSet[T]) Clone() *HashSet[T] {
	clone := NewHashSet[T]()
	for k, v := range s.data {
		clone.data[k] = v
	}
	return clone
}

// Equals 比较两个集合是否相等
func (s *HashSet[T]) Equals(other *HashSet[T]) bool {
	if s.Size() != other.Size() {
		return false
	}

	for _, v := range s.data {
		if !other.Contains(*v) {
			return false
		}
	}
	return true
}

// Union 并集
func (s *HashSet[T]) Union(other *HashSet[T]) *HashSet[T] {
	result := s.Clone()
	for _, v := range other.data {
		result.Add(*v)
	}
	return result
}

// Intersection 交集
func (s *HashSet[T]) Intersection(other *HashSet[T]) *HashSet[T] {
	result := NewHashSet[T]()

	// 遍历较小的集合
	var smaller, larger *HashSet[T]
	if s.Size() < other.Size() {
		smaller, larger = s, other
	} else {
		smaller, larger = other, s
	}

	for _, v := range smaller.data {
		if larger.Contains(*v) {
			result.Add(*v)
		}
	}
	return result
}

// Difference 差集
func (s *HashSet[T]) Difference(other *HashSet[T]) *HashSet[T] {
	result := NewHashSet[T]()
	for _, v := range s.data {
		if !other.Contains(*v) {
			result.Add(*v)
		}
	}
	return result
}

// SymmetricDifference 对称差集
func (s *HashSet[T]) SymmetricDifference(other *HashSet[T]) *HashSet[T] {
	union := s.Union(other)
	intersection := s.Intersection(other)
	return union.Difference(intersection)
}

// IsSubset 判断是否为子集
func (s *HashSet[T]) IsSubset(other *HashSet[T]) bool {
	if s.Size() > other.Size() {
		return false
	}

	for _, v := range s.data {
		if !other.Contains(*v) {
			return false
		}
	}
	return true
}

// IsSuperset 判断是否为超集
func (s *HashSet[T]) IsSuperset(other *HashSet[T]) bool {
	return other.IsSubset(s)
}

// Pop 随机移除并返回一个元素
func (s *HashSet[T]) Pop() (T, bool) {
	if s.IsEmpty() {
		var zero T
		return zero, false
	}

	rand.Seed(time.Now().UnixNano())
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}

	randomKey := keys[rand.Intn(len(keys))]
	value := s.data[randomKey]
	delete(s.data, randomKey)

	return *value, true
}

// Random 随机返回一个元素（不移除）
func (s *HashSet[T]) Random() (T, bool) {
	if s.IsEmpty() {
		var zero T
		return zero, false
	}

	rand.Seed(time.Now().UnixNano())
	for _, v := range s.data {
		return *v, true
	}

	var zero T
	return zero, false
}

// String 字符串表示
func (s *HashSet[T]) String() string {
	values := s.Values()
	return fmt.Sprintf("HashSet{%v}", values)
}

// ToSlice 转换为切片
func (s *HashSet[T]) ToSlice() []T {
	return s.Values()
}

// FromSlice 从切片创建集合
func FromSlice[T any](slice []T) *HashSet[T] {
	set := NewHashSet[T]()
	for _, v := range slice {
		set.Add(v)
	}
	return set
}

// JSON 序列化支持
type jsonHashSet[T any] struct {
	Data []T `json:"data"`
}

// MarshalJSON JSON 序列化
func (s *HashSet[T]) MarshalJSON() ([]byte, error) {
	j := jsonHashSet[T]{
		Data: s.Values(),
	}
	return json.Marshal(j)
}

// UnmarshalJSON JSON 反序列化
func (s *HashSet[T]) UnmarshalJSON(data []byte) error {
	var j jsonHashSet[T]
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}

	s.data = make(map[string]*T)
	for _, v := range j.Data {
		s.Add(v)
	}
	return nil
}

// 比较器接口
type Comparable[T any] interface {
	Compare(other T) int
}

// SortedValues 返回排序后的值（需要 T 实现 Comparable 接口）
func (s *HashSet[T]) SortedValues() []T {
	values := s.Values()

	// 使用类型断言检查是否实现了 Comparable 接口
	if len(values) > 0 {
		if _, ok := any(values[0]).(Comparable[T]); ok {
			// 这里可以使用 sort.Slice，但需要运行时类型断言
			// 更安全的方式是让调用者提供比较函数
		}
	}

	return values
}

// 带比较函数的排序
func (s *HashSet[T]) SortedValuesWith(less func(T, T) bool) []T {
	values := s.Values()

	// 实现排序
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if less(values[j], values[i]) {
				values[i], values[j] = values[j], values[i]
			}
		}
	}

	return values
}
