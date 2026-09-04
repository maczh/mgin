// Copyright 2022 Liuxiangchao iwind.liu@gmail.com. All rights reserved. Official site: https://goedge.cn .

package utils

import "testing"

func TestMapToStruct(t *testing.T) {
	type User struct {
		Name      string  `json:"name,omitempty"`
		Age       int     `json:"age,omitempty"`
		Mobile    int64   `json:"mobile,omitempty"`
		PayAmount float64 `json:"payAmount,omitempty"`
		Show      bool    `json:"show,omitempty"`
	}
	user := User{
		Name:      "iwind",
		Age:       18,
		Mobile:    13950280566,
		PayAmount: 1001.01,
		Show:      true,
	}
	m, err := Struct2StringMap(user)
	if err != nil {
		t.Error(err)
	}
	t.Log(ToJSONPretty(m))

}
