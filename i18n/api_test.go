package i18n

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/middleware/xlang"
	"github.com/maczh/mgin/models/vo"
)

func TestDictionaryAPIAndRequestLanguage(t *testing.T) {
	s := testStore(t)
	old := currentStore.Swap(s)
	defer currentStore.Store(old)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(xlang.RequestLanguage())
	RouterGroup(r.Group("/i18n"))
	r.GET("/translate", func(c *gin.Context) { c.JSON(200, vo.Success(String(c.Query("messageId")))) })
	call := func(method, path, body, language string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if language != "" {
			req.Header.Set("X-Lang", language)
		}
		recorder := httptest.NewRecorder()
		r.ServeHTTP(recorder, req)
		if recorder.Code != 200 {
			t.Fatalf("HTTP status=%d: %s", recorder.Code, recorder.Body.String())
		}
		return recorder
	}
	check := func(method, path, body string, want int) vo.Response[Entry] {
		t.Helper()
		recorder := call(method, path, body, "")
		var response vo.Response[Entry]
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Code != want {
			t.Fatalf("%s %s => %s", method, path, recorder.Body.String())
		}
		return response
	}
	entry := `{"messageId":"hello","language":"EN","text":"Hello"}`
	got := check(http.MethodPost, "/i18n", entry, vo.SUCCESS)
	if got.Data.Language != "en" || got.Data.Text != "Hello" {
		t.Fatal(got)
	}
	check(http.MethodPost, "/i18n", entry, 409)
	path := "/i18n?" + url.Values{"messageId": {"hello"}, "language": {"en"}}.Encode()
	got = check(http.MethodGet, path, "", vo.SUCCESS)
	if got.Data.Text != "Hello" {
		t.Fatal(got)
	}
	check(http.MethodPut, "/i18n", `{"messageId":"hello","language":"en","text":"Updated"}`, vo.SUCCESS)
	got = check(http.MethodGet, path, "", vo.SUCCESS)
	if got.Data.Text != "Updated" {
		t.Fatal(got)
	}
	var list vo.ResponsePage[Entry]
	if err := json.Unmarshal(call(http.MethodGet, "/i18n/list?page=1&pageSize=1&language=en", "", "").Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.Code != vo.SUCCESS || list.Data.Total != 1 || len(list.Data.List) != 1 || list.Data.List[0].Text != "Updated" {
		t.Fatalf("list=%+v", list)
	}
	check(http.MethodPost, "/i18n", `{"language":"en","text":"bad"}`, 400)
	check(http.MethodPost, "/i18n", `{broken`, 400)
	check(http.MethodGet, "/i18n/list?pageSize=101", "", 400)
	check(http.MethodGet, "/i18n/list?page=bad", "", 400)
	if err := s.Create(context.Background(), &Entry{MessageID: "hello", Language: "zh-cn", Text: "你好"}); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ lang, want string }{{"en", "Updated"}, {"de", "你好"}, {"", "你好"}} {
		var translated vo.Response[string]
		recorder := call(http.MethodGet, "/translate?messageId=hello", "", test.lang)
		if err := json.Unmarshal(recorder.Body.Bytes(), &translated); err != nil {
			t.Fatal(err)
		}
		if translated.Data != test.want {
			t.Fatalf("language=%q translation=%+v", test.lang, translated)
		}
	}
	check(http.MethodDelete, path, "", vo.SUCCESS)
	check(http.MethodGet, path, "", 404)
	check(http.MethodDelete, path, "", 404)
	check(http.MethodPut, "/i18n", `{"messageId":"missing","language":"en","text":"bad"}`, 404)
	currentStore.Store(nil)
	check(http.MethodGet, path, "", 503)
}
