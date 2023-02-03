package utils

import (
	"errors"
	"fmt"
	"reflect"
)

func Clone(src interface{}, dst interface{}) {
	FromJSON(ToJSON(src), dst)
}

// Struct2Map return map
func Struct2Map(obj interface{}) map[string]interface{} {
	objT := reflect.TypeOf(obj)
	if objT.Kind() != reflect.Struct {
		panic(errors.New("argument is not of the expected type"))
	}
	objV := reflect.ValueOf(obj)
	var data = make(map[string]interface{})
	for i := 0; i < objT.NumField(); i++ {
		switch objV.Field(i).Type().Kind() {
		case reflect.Struct:
			node := Struct2Map(objV.Field(i).Interface())
			data[getFieldName(objT.Field(i))] = node
		case reflect.Map:
			data[getFieldName(objT.Field(i))] = objV.Field(i).Interface()
		case reflect.Slice:
			target := objV.Field(i).Interface()
			tmp := make([]interface{}, reflect.ValueOf(target).Len())
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
func Struct2MapString(obj interface{}) map[string]string {
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
			data[getFieldName(objT.Field(i))] = objV.Field(i).String()
		default:
			data[getFieldName(objT.Field(i))] = fmt.Sprintf("%v", objV.Field(i).Interface())
		}
	}
	return data
}

func getFieldName(f reflect.StructField) string {
	field := f.Tag.Get("json")
	if field == "" {
		field = f.Name
	}
	return field
}

func GetStructFields(obj interface{}) []string {
	t := reflect.TypeOf(obj)
	fields := make([]string, 0)
	for i := 0; i < t.NumField(); i++ {
		fields = append(fields, t.Field(i).Name)
	}
	return fields
}

func GetStructJsonTags(obj interface{}) []string {
	t := reflect.TypeOf(obj)
	fields := make([]string, 0)
	for i := 0; i < t.NumField(); i++ {
		fields = append(fields, t.Field(i).Tag.Get("json"))
	}
	return fields
}

func AnyToMap(obj interface{}) map[string]string {
	if obj == nil {
		return map[string]string{}
	}
	switch reflect.ValueOf(obj).Type().Kind() {
	case reflect.Map:
		rs := make(map[string]string)
		m, ok := obj.(map[string]interface{})
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
