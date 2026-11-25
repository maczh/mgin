package utils

import (
	"fmt"
	"github.com/maczh/mgin/v2/pkg/logs"
	"reflect"
	"strings"
)

func Clone(src any, dst any) {
	FromJSON(ToJSON(src), dst)
}

func structValue(obj any) (reflect.Type, reflect.Value, bool) {
	v := reflect.ValueOf(obj)
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return nil, reflect.Value{}, false
		}
		v = v.Elem()
	}
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return nil, reflect.Value{}, false
	}
	return v.Type(), v, true
}

// Struct2Map return map
func Struct2Map(obj any) map[string]any {
	objT, objV, ok := structValue(obj)
	if !ok {
		return map[string]any{}
	}
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
	objT, objV, ok := structValue(obj)
	if !ok {
		return map[string]string{}
	}
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
	fields := make([]string, 0)
	t, _, ok := structValue(obj)
	if !ok {
		return fields
	}
	for i := 0; i < t.NumField(); i++ {
		fields = append(fields, t.Field(i).Name)
	}
	return fields
}

func GetStructJsonTags(obj any) []string {
	fields := make([]string, 0)
	t, _, ok := structValue(obj)
	if !ok {
		return fields
	}
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
	value := reflect.ValueOf(obj)
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return map[string]string{}
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Map:
		rs := make(map[string]string)
		if value.Type().Key().Kind() != reflect.String {
			return rs
		}
		iter := value.MapRange()
		for iter.Next() {
			key := iter.Key().String()
			item := iter.Value()
			if !item.IsValid() || (item.Kind() == reflect.Interface && item.IsNil()) {
				rs[key] = "<nil>"
				continue
			}
			if item.Kind() == reflect.Interface {
				item = item.Elem()
			}
			if !item.IsValid() {
				rs[key] = "<nil>"
			} else if item.Kind() == reflect.String {
				rs[key] = item.String()
			} else if item.CanInterface() {
				v := item.Interface()
				switch item.Kind() {
				case reflect.Struct, reflect.Slice, reflect.Map, reflect.Array:
					rs[key] = ToJSON(v)
				default:
					rs[key] = fmt.Sprintf("%v", v)
				}
			}
		}
		return rs
	case reflect.Struct:
		if value.CanInterface() {
			return Struct2MapString(value.Interface())
		}
		return map[string]string{}
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
