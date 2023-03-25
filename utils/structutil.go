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
	if objT.Kind() != reflect.Struct {
		panic(errors.New("argument is not of the expected type"))
	}
	objV := reflect.ValueOf(obj)
	var data = make(map[string]any)
	for i := 0; i < objT.NumField(); i++ {
		switch objV.Field(i).Type().Kind() {
		case reflect.Struct:
			node := Struct2Map(objV.Field(i).Interface())
			data[getFieldName(objT.Field(i))] = node
		case reflect.Map:
			data[getFieldName(objT.Field(i))] = objV.Field(i).Interface()
		case reflect.Slice:
			target := objV.Field(i).Interface()
			tmp := make([]any, reflect.ValueOf(target).Len())
			for j := 0; j < reflect.ValueOf(target).Len(); j++ {
				if reflect.ValueOf(target).Index(j).Kind() == reflect.Struct {
					node := Struct2Map(reflect.ValueOf(target).Index(j).Interface())
					tmp[j] = node
				} else {
					tmp[j] = reflect.ValueOf(target).Index(j).Interface()
				}
			}
			data[getFieldName(objT.Field(i))] = tmp
		default:
			data[getFieldName(objT.Field(i))] = objV.Field(i).Interface()
		}
	}
	return data
}

func Struct2MapString(obj any) map[string]string {
	objT := reflect.TypeOf(obj)
	if objT.Kind() != reflect.Struct {
		panic(errors.New("argument is not of the expected type"))
	}
	objV := reflect.ValueOf(obj)
	var data = make(map[string]string)
	for i := 0; i < objT.NumField(); i++ {
		switch objV.Field(i).Type().Kind() {
		case reflect.Struct, reflect.Slice, reflect.Map:
			val := ToJSON(objV.Field(i).Interface())
			data[getFieldName(objT.Field(i))] = val
		case reflect.String:
			k, v := getFieldName(objT.Field(i)), objV.Field(i).String()
			if k != "-" && v != "" {
				data[k] = v
			}
		default:
			data[getFieldName(objT.Field(i))] = fmt.Sprintf("%v", objV.Field(i).Interface())
		}
	}
	return data
}

func GetStructFields(obj any) []string {
	t := reflect.TypeOf(obj)
	fields := make([]string, 0)
	for i := 0; i < t.NumField(); i++ {
		fields = append(fields, t.Field(i).Name)
	}
	return fields
}

func GetStructJsonTags(obj any) []string {
	t := reflect.TypeOf(obj)
	fields := make([]string, 0)
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
	switch reflect.ValueOf(obj).Type().Kind() {
	case reflect.Map:
		if m, ok := obj.(map[string]string); ok {
			return m
		}
		rs := make(map[string]string)
		m, ok := obj.(map[string]any)
		if ok {
			for k, v := range m {
				switch reflect.ValueOf(v).Type().Kind() {
				case reflect.String:
					rs[k] = v.(string)
				case reflect.Struct, reflect.Slice, reflect.Map:
					rs[k] = ToJSON(v)
				default:
					rs[k] = fmt.Sprintf("%v", v)
				}
			}
			return rs
		} else {
			return obj.(map[string]string)
		}
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
