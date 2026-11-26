package logs

import (
	"encoding/base64"
	"encoding/pem"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/logsink"
	"github.com/maczh/mgin/models"
	"google.golang.org/protobuf/encoding/protowire"
)

func TestVictoriaLogsBasicAuth(t *testing.T) {
	oldLog, oldLogger, oldName := config.Config.Log, config.Config.Logger, config.Config.App.Name
	defer func() {
		config.Config.Log, config.Config.Logger, config.Config.App.Name = oldLog, oldLogger, oldName
		logsink.Reset()
	}()
	logsink.Reset()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "Basic " + base64.StdEncoding.EncodeToString([]byte("vlogs:secret"))
		if r.Header.Get("Authorization") != want {
			t.Errorf("authorization = %q, want %q", r.Header.Get("Authorization"), want)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	config.Config.App.Name = "test-app"
	config.Config.Logger.Outputs = []config.LogOutput{{Target: VictoriaLogsTarget}}
	config.Config.Log.VictoriaLogs = config.VictoriaLogsConfig{URL: server.URL, HTTPPass: "secret", APICert: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))}
	if err := InitVictoriaLogs(); err != nil {
		t.Fatal(err)
	}
	logsink.EmitRecordTo(VictoriaLogsTarget, &logsink.Record{Level: "info", Message: "basic auth"})
	logsink.Close(3 * time.Second)
}

func TestVictoriaLogsOTLPAndRouting(t *testing.T) {
	oldLog, oldLogger, oldName := config.Config.Log, config.Config.Logger, config.Config.App.Name
	defer func() {
		config.Config.Log = oldLog
		config.Config.Logger = oldLogger
		config.Config.App.Name = oldName
		logsink.Reset()
	}()
	logsink.Reset()
	var mu sync.Mutex
	payloads := [][]byte{}
	retries := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/insert/opentelemetry/v1/logs" || r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("Content-Type") != "application/x-protobuf" {
			t.Error("incorrect request headers or URL")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		mu.Lock()
		defer mu.Unlock()
		if retries == 0 {
			retries++
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		payloads = append(payloads, body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	config.Config.App.Name = "test-app"
	config.Config.Logger.Outputs = []config.LogOutput{{Target: VictoriaLogsTarget}}
	config.Config.Log.VictoriaLogs = config.VictoriaLogsConfig{URL: server.URL, APIToken: "test-token",
		APICert: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})), Request: true, Client: true}
	if err := InitVictoriaLogs(); err != nil {
		t.Fatal(err)
	}
	logsink.Emit(&models.PostLog{Time: "2026-11-26 12:00:00", Method: "GET", Uri: "/hello", RequestId: "request-1",
		RequestParam: map[string]any{"page": float64(2), "tags": []any{"a", "b"}}})
	logsink.EmitRecordTo(VictoriaLogsTarget, &logsink.Record{Level: "warn", Message: "console hello"})
	logsink.EmitRecordTo(VictoriaLogsClientTarget, &logsink.Record{Level: "info", Message: "client hello"})
	logsink.Close(3 * time.Second)

	records := decodeOTLPRecords(t, payloads)
	if len(records) != 3 {
		t.Fatalf("got %d records", len(records))
	}
	byMessage := map[string]decodedOTLPRecord{}
	for _, record := range records {
		byMessage[record.message] = record
		if record.serviceName != "test-app" {
			t.Errorf("service.name=%s", record.serviceName)
		}
	}
	if kinds := map[string]string{
		"GET /hello":    byMessage["GET /hello"].attributes["type"].(string),
		"console hello": byMessage["console hello"].attributes["type"].(string),
		"client hello":  byMessage["client hello"].attributes["type"].(string),
	}; kinds["GET /hello"] != "request" || kinds["console hello"] != "console" || kinds["client hello"] != "request" {
		t.Fatalf("routing=%v", kinds)
	}
	for message, record := range byMessage {
		if record.attributes["app"] != "test-app" {
			t.Errorf("%s app=%v", message, record.attributes["app"])
		}
	}
	if got := byMessage["console hello"].severityNumber; got != 13 {
		t.Fatalf("warn severity = %d, want 13", got)
	}
	if got := byMessage["console hello"].attributes["level"]; got != "warn" {
		t.Fatalf("level attribute = %v", got)
	}
	if got := byMessage["client hello"].severityNumber; got != 9 {
		t.Fatalf("info severity = %d, want 9", got)
	}
	request := byMessage["GET /hello"]
	if request.attributes["request_id"] != "request-1" {
		t.Fatalf("request_id = %v", request.attributes["request_id"])
	}
	if request.attributes["method"] != "GET" {
		t.Fatalf("method = %v", request.attributes["method"])
	}
	if _, ok := request.attributes["ttl"].(int64); !ok {
		t.Fatalf("ttl = %#v", request.attributes["ttl"])
	}
	param, ok := request.attributes["requestParam"].(map[string]any)
	if !ok || param["page"] != int64(2) {
		t.Fatalf("requestParam = %#v", request.attributes["requestParam"])
	}
	if tags, ok := param["tags"].([]any); !ok || len(tags) != 2 || tags[0] != "a" {
		t.Fatalf("requestParam.tags = %#v", param["tags"])
	}
	if request.timeUnixNano == 0 {
		t.Fatal("timeUnixNano is zero")
	}

	config.Config.Log.VictoriaLogs.APICert = "invalid"
	if err := InitVictoriaLogs(); err == nil {
		t.Fatal("invalid certificate accepted")
	}
}

type pbField struct {
	num  protowire.Number
	typ  protowire.Type
	data []byte
	u64  uint64
}

func parsePB(t *testing.T, data []byte) []pbField {
	t.Helper()
	var fields []pbField
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			t.Fatalf("consume tag: %v", protowire.ParseError(n))
		}
		data = data[n:]
		field := pbField{num: num, typ: typ}
		switch typ {
		case protowire.BytesType:
			value, n := protowire.ConsumeBytes(data)
			if n < 0 {
				t.Fatalf("consume bytes: %v", protowire.ParseError(n))
			}
			field.data, data = value, data[n:]
		case protowire.VarintType:
			value, n := protowire.ConsumeVarint(data)
			if n < 0 {
				t.Fatalf("consume varint: %v", protowire.ParseError(n))
			}
			field.u64, data = value, data[n:]
		case protowire.Fixed64Type:
			value, n := protowire.ConsumeFixed64(data)
			if n < 0 {
				t.Fatalf("consume fixed64: %v", protowire.ParseError(n))
			}
			field.u64, data = value, data[n:]
		case protowire.Fixed32Type:
			value, n := protowire.ConsumeFixed32(data)
			if n < 0 {
				t.Fatalf("consume fixed32: %v", protowire.ParseError(n))
			}
			field.u64, data = uint64(value), data[n:]
		default:
			t.Fatalf("unexpected wire type %v", typ)
		}
		fields = append(fields, field)
	}
	return fields
}

func pbFirst(t *testing.T, fields []pbField, num protowire.Number, typ protowire.Type) pbField {
	t.Helper()
	for _, field := range fields {
		if field.num == num && field.typ == typ {
			return field
		}
	}
	t.Fatalf("field %d/%v not found", num, typ)
	return pbField{}
}

func pbAll(fields []pbField, num protowire.Number, typ protowire.Type) []pbField {
	var out []pbField
	for _, field := range fields {
		if field.num == num && field.typ == typ {
			out = append(out, field)
		}
	}
	return out
}

func pbKeyValue(t *testing.T, data []byte) (string, any) {
	t.Helper()
	fields := parsePB(t, data)
	key := string(pbFirst(t, fields, 1, protowire.BytesType).data)
	value := decodeAnyValue(t, pbFirst(t, fields, 2, protowire.BytesType).data)
	return key, value
}

func decodeAnyValue(t *testing.T, data []byte) any {
	t.Helper()
	if len(data) == 0 {
		return nil
	}
	fields := parsePB(t, data)
	field := fields[0]
	switch field.num {
	case 1:
		return string(field.data)
	case 2:
		return field.u64 != 0
	case 3:
		return int64(field.u64)
	case 4:
		return math.Float64frombits(field.u64)
	case 5:
		items := pbAll(parsePB(t, field.data), 1, protowire.BytesType)
		values := make([]any, 0, len(items))
		for _, item := range items {
			values = append(values, decodeAnyValue(t, item.data))
		}
		return values
	case 6:
		items := pbAll(parsePB(t, field.data), 1, protowire.BytesType)
		values := map[string]any{}
		for _, item := range items {
			key, value := pbKeyValue(t, item.data)
			values[key] = value
		}
		return values
	default:
		t.Fatalf("unexpected AnyValue field %d", field.num)
		return nil
	}
}

type decodedOTLPRecord struct {
	message        string
	timeUnixNano   uint64
	severityNumber int
	attributes     map[string]any
	serviceName    string
}

func decodeOTLPRecords(t *testing.T, payloads [][]byte) []decodedOTLPRecord {
	t.Helper()
	var records []decodedOTLPRecord
	for _, payload := range payloads {
		request := parsePB(t, payload)
		resourceLogs := pbFirst(t, request, 1, protowire.BytesType)
		resourceLogsFields := parsePB(t, resourceLogs.data)
		resource := pbFirst(t, resourceLogsFields, 1, protowire.BytesType)
		resourceFields := parsePB(t, resource.data)
		key, value := pbKeyValue(t, pbFirst(t, resourceFields, 1, protowire.BytesType).data)
		if key != "service.name" {
			t.Fatalf("resource key = %q", key)
		}
		serviceName, _ := value.(string)
		scopeLogs := pbFirst(t, resourceLogsFields, 2, protowire.BytesType)
		scopeLogsFields := parsePB(t, scopeLogs.data)
		for _, recordField := range pbAll(scopeLogsFields, 2, protowire.BytesType) {
			recordFields := parsePB(t, recordField.data)
			record := decodedOTLPRecord{
				timeUnixNano:   pbFirst(t, recordFields, 1, protowire.Fixed64Type).u64,
				severityNumber: int(pbFirst(t, recordFields, 2, protowire.VarintType).u64),
				attributes:     map[string]any{},
				serviceName:    serviceName,
			}
			body := parsePB(t, pbFirst(t, recordFields, 5, protowire.BytesType).data)
			record.message = string(pbFirst(t, body, 1, protowire.BytesType).data)
			for _, attrField := range pbAll(recordFields, 6, protowire.BytesType) {
				key, value := pbKeyValue(t, attrField.data)
				record.attributes[key] = value
			}
			records = append(records, record)
		}
	}
	return records
}
