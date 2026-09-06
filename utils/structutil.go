package utils

import (
	"errors"
	"fmt"
	"github.com/maczh/mgin/logs"
	"reflect"
	"strings"
)

func Clone(src any, dst any) {
	FromJSON(ToJSON(src), dst)
}

// Struct2Map return map
func Struct2Map(obj any) map[string]any {
	objT := reflect.TypeOf(obj)
	if objT == nil || objT.Kind() != reflect.Struct {
		panic(errors.New("argument is not of the expected type"))
	}
	objV := reflect.ValueOf(obj)
	var data = make(map[string]any, objT.NumField())
	for i := 0; i < objT.NumField(); i++ {
		field := objT.Field(i)
		// 未导出字段无法调用 Interface()，必须跳过，否则 panic
		if !field.IsExported() {
			continue
		}
		switch objV.Field(i).Type().Kind() {
		case reflect.Struct:
			node := Struct2Map(objV.Field(i).Interface())
			data[getFieldName(field)] = node
		case reflect.Map:
			data[getFieldName(field)] = objV.Field(i).Interface()
		case reflect.Slice:
			target := objV.Field(i).Interface()
			tv := reflect.ValueOf(target)
			tmp := make([]any, tv.Len())
			for j := 0; j < tv.Len(); j++ {
				if tv.Index(j).Kind() == reflect.Struct {
					tmp[j] = Struct2Map(tv.Index(j).Interface())
				} else {
					tmp[j] = tv.Index(j).Interface()
				}
			}
			data[getFieldName(field)] = tmp
		default:
			data[getFieldName(field)] = objV.Field(i).Interface()
		}
	}
	return data
}

func Struct2MapString(obj any) map[string]string {
	objT := reflect.TypeOf(obj)
	if objT == nil || objT.Kind() != reflect.Struct {
		panic(errors.New("argument is not of the expected type"))
	}
	objV := reflect.ValueOf(obj)
	var data = make(map[string]string, objT.NumField())
	for i := 0; i < objT.NumField(); i++ {
		field := objT.Field(i)
		// 未导出字段无法调用 Interface()，必须跳过，否则 panic
		if !field.IsExported() {
			continue
		}
		switch objV.Field(i).Type().Kind() {
		case reflect.Struct, reflect.Slice, reflect.Map:
			val := ToJSON(objV.Field(i).Interface())
			data[getFieldName(field)] = val
		case reflect.String:
			k, v := getFieldName(field), objV.Field(i).String()
			if k != "-" && v != "" {
				data[k] = v
			}
		default:
			data[getFieldName(field)] = fmt.Sprintf("%v", objV.Field(i).Interface())
		}
	}
	return data
}

// structTypeOf 安全地把任意对象解引用为结构体类型，非结构体返回 nil
func structTypeOf(obj any) reflect.Type {
	t := reflect.TypeOf(obj)
	if t == nil {
		return nil
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	return t
}

func GetStructFields(obj any) []string {
	t := structTypeOf(obj)
	if t == nil {
		return []string{}
	}
	fields := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		fields = append(fields, t.Field(i).Name)
	}
	return fields
}

func GetStructJsonTags(obj any) []string {
	t := structTypeOf(obj)
	if t == nil {
		return []string{}
	}
	fields := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		fields = append(fields, t.Field(i).Tag.Get("json"))
	}
	return fields
}

func getFieldName(f reflect.StructField) string {
	field := f.Tag.Get("json")
	if field == "" {
		field = f.Name
	}
	if strings.Contains(field, ",") {
		field = strings.Split(field, ",")[0]
	}
	return field
}

func AnyToMap(obj any) map[string]string {
	if obj == nil {
		return map[string]string{}
	}
	ov := reflect.ValueOf(obj)
	if !ov.IsValid() {
		return map[string]string{}
	}
	switch ov.Type().Kind() {
	case reflect.Map:
		if m, ok := obj.(map[string]string); ok {
			return m
		}
		rs := make(map[string]string)
		if m, ok := obj.(map[string]any); ok {
			for k, v := range m {
				// map 中的 nil 值：reflect.ValueOf(nil) 是 zero Value，取 Type() 会 panic
				if v == nil {
					rs[k] = ""
					continue
				}
				switch reflect.ValueOf(v).Kind() {
				case reflect.String:
					if s, ok := v.(string); ok {
						rs[k] = s
					} else {
						rs[k] = fmt.Sprintf("%v", v)
					}
				case reflect.Struct, reflect.Slice, reflect.Map:
					rs[k] = ToJSON(v)
				default:
					rs[k] = fmt.Sprintf("%v", v)
				}
			}
			return rs
		}
		// 其它 map 类型：通用转换，避免类型断言 panic
		for _, key := range ov.MapKeys() {
			mv := ov.MapIndex(key)
			if !mv.IsValid() {
				rs[fmt.Sprintf("%v", key.Interface())] = ""
				continue
			}
			rs[fmt.Sprintf("%v", key.Interface())] = ToJSON(mv.Interface())
		}
		return rs
	case reflect.Struct:
		return Struct2MapString(obj)
	default:
		return map[string]string{}
	}
}

func deepCopy(from, to interface{}) {
	fromValue := reflect.ValueOf(from)
	if fromValue.Kind() != reflect.Struct {
		logs.Error("源对象不是结构体类型")
		return
	}
	val := reflect.ValueOf(to)
	if val.Kind() != reflect.Ptr {
		logs.Error("目标对象不是结构体指针")
		return
	}
	toValue := val.Elem()

	for i := 0; i < fromValue.NumField(); i++ {
		fromFieldValue := fromValue.Field(i)
		toFieldValue := toValue.FieldByName(fromValue.Type().Field(i).Name)

		if !toFieldValue.IsValid() {
			continue
		}

		if fromFieldValue.Type() == toFieldValue.Type() {
			toFieldValue.Set(fromFieldValue)
		} else if fromFieldValue.Kind() == reflect.Struct && toFieldValue.Kind() == reflect.Struct {
			deepCopy(fromFieldValue.Addr().Interface(), toFieldValue.Addr().Interface())
		}
	}
}

func CopyStruct(src, dst any) {
	deepCopy(src, dst)
}

func DeepCopy[D any](src any) D {
	var dst D
	deepCopy(src, &dst)
	return dst
}
