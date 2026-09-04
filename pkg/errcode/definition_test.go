package errcode

import "testing"

func TestNew_ValidatesHTTPStatus(t *testing.T) {
	// 0 视为 500（服务端错误）
	d := New("common", 9999, 0, "test.zero")
	if d.HTTPStatus != 500 {
		t.Errorf("expected HTTPStatus 500 for 0, got %d", d.HTTPStatus)
	}

	// 非法值（>= 600 或 < 100）回退到 500
	for _, bad := range []int{99, 600, 1000, -1} {
		d := New("common", 9999, bad, "test.bad")
		if d.HTTPStatus != 500 {
			t.Errorf("expected HTTPStatus 500 for %d, got %d", bad, d.HTTPStatus)
		}
	}
}

func TestLookupDef_Builtin(t *testing.T) {
	// 预置映射应能找到
	d := LookupDef(URI_NOT_FOUND)
	if d.Code != URI_NOT_FOUND {
		t.Errorf("expected Code=%d, got %d", URI_NOT_FOUND, d.Code)
	}
	if d.HTTPStatus != 404 {
		t.Errorf("expected HTTPStatus=404 for URI_NOT_FOUND, got %d", d.HTTPStatus)
	}

	d = LookupDef(AUTHENTICATION_FAILURE)
	if d.HTTPStatus != 401 {
		t.Errorf("expected HTTPStatus=401 for AUTHENTICATION_FAILURE, got %d", d.HTTPStatus)
	}
}

func TestLookupDef_Unknown(t *testing.T) {
	d := LookupDef(99999)
	if d.HTTPStatus != 500 {
		t.Errorf("expected HTTPStatus=500 for unknown code, got %d", d.HTTPStatus)
	}
	if d.Code != 99999 {
		t.Errorf("expected Code=99999 preserved, got %d", d.Code)
	}
}

func TestRegister_Override(t *testing.T) {
	// 业务可覆盖预置定义
	custom := New(ModuleAuth, URI_NOT_FOUND, 410, "test.gone")
	RegisterDef(custom)
	d := LookupDef(URI_NOT_FOUND)
	if d.HTTPStatus != 410 {
		t.Errorf("expected HTTPStatus=410 after RegisterDef, got %d", d.HTTPStatus)
	}
	// 测试结束恢复原值，避免污染其他测试
	definitions[URI_NOT_FOUND] = New(ModuleCommon, URI_NOT_FOUND, 404, "errcode.uri_not_found")
}
