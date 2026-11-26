package logs

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/logsink"
	"google.golang.org/protobuf/encoding/protowire"
)

const VictoriaLogsTarget = "victorialogs"
const VictoriaLogsClientTarget = "victorialogs-client"

// VictoriaLogs ingests logs via its OpenTelemetry OTLP/HTTP endpoint. VictoriaLogs
// only accepts protobuf encoding for OTLP, so the payload is encoded directly
// with protobuf wire primitives instead of pulling in the full OTel SDK.
const victoriaOTLPLogsPath = "/insert/opentelemetry/v1/logs"

const (
	otlpScopeName     = "mgin"
	otlpContentType   = "application/x-protobuf"
	otlpFieldTime     = protowire.Number(1)
	otlpFieldSeverity = protowire.Number(2)
	otlpFieldSevText  = protowire.Number(3)
	otlpFieldBody     = protowire.Number(5)
	otlpFieldAttrs    = protowire.Number(6)
)

type victoriaEvent struct {
	App       string
	Type      string
	Level     string
	Message   string
	Timestamp string
	RequestID string
	Fields    any
}

type victoriaCollector struct {
	client   *http.Client
	endpoint string
	token    string
	username string
	password string
	app      string
}

type otlpAttribute struct {
	key   string
	value any
}

// InitVictoriaLogs installs destinations selected by YAML. Logs are submitted
// to VictoriaLogs through its OpenTelemetry OTLP/HTTP endpoint. Invalid TLS or
// auth configuration fails startup rather than silently losing redirected logs.
func InitVictoriaLogs() error {
	snap := config.Current()
	cfg := snap.Log.VictoriaLogs
	console := false
	for _, out := range snap.Logger.Outputs {
		if out.Target == VictoriaLogsTarget {
			console = true
		}
	}
	if !cfg.Request && !cfg.Client && !console {
		return nil
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`).MatchString(snap.App.Name) {
		return fmt.Errorf("victorialogs: invalid application name")
	}
	u, err := url.Parse(cfg.URL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("victorialogs: url must be an HTTPS collector base URL")
	}
	if strings.TrimSpace(cfg.APIToken) == "" && strings.TrimSpace(cfg.HTTPPass) == "" {
		return fmt.Errorf("victorialogs: api_token is required")
	}
	var roots *x509.CertPool
	if !cfg.InsecureSkipVerify {
		roots = x509.NewCertPool()
		if !roots.AppendCertsFromPEM([]byte(cfg.APICert)) {
			return fmt.Errorf("victorialogs: api_crt must contain a PEM certificate")
		}
	}
	timeout := time.Duration(cfg.Timeout) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	baseTransport, ok := http.DefaultTransport.(*http.Transport)
	var transport *http.Transport
	if ok && baseTransport != nil {
		transport = baseTransport.Clone()
	} else {
		transport = &http.Transport{Proxy: http.ProxyFromEnvironment}
	}
	transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12, InsecureSkipVerify: cfg.InsecureSkipVerify} // #nosec G402 -- explicitly opt-in for test/self-signed collectors
	c := &victoriaCollector{client: &http.Client{Transport: transport, Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		endpoint: strings.TrimRight(cfg.URL, "/") + victoriaOTLPLogsPath, token: cfg.APIToken, app: snap.App.Name}
	if cfg.HTTPPass != "" {
		c.username = cfg.HTTPUser
		if c.username == "" {
			c.username = "vlogs"
		}
		c.password = cfg.HTTPPass
	}
	if cfg.Request {
		h := logsink.NewHandlerFunc(VictoriaLogsTarget, func(e *logsink.Entry) error {
			return c.write(victoriaEvent{App: c.app, Type: "request", Level: "info", Message: e.Method + " " + e.Uri,
				Timestamp: e.Time, RequestID: e.RequestId, Fields: e})
		}).WithClose(func() error { transport.CloseIdleConnections(); return nil })
		if err := logsink.Register(h, logsink.WithQueueSize(snap.Log.Sink.Queue)); err != nil {
			return err
		}
	}
	for _, target := range []struct {
		name, kind string
		enabled    bool
	}{
		{VictoriaLogsTarget, "console", console}, {VictoriaLogsClientTarget, "request", cfg.Client},
	} {
		if !target.enabled {
			continue
		}
		kind := target.kind
		h := logsink.NewRecordHandlerFunc(target.name, func(r *logsink.Record) error {
			app := r.App
			if app == "" {
				app = c.app
			}
			return c.write(victoriaEvent{App: app, Type: kind, Level: r.Level, Message: r.Message,
				Timestamp: r.Time, RequestID: r.RequestID, Fields: r})
		}).WithClose(func() error { transport.CloseIdleConnections(); return nil })
		if err := logsink.RegisterRecord(h, logsink.WithQueueSize(snap.Log.Sink.Queue)); err != nil {
			return err
		}
	}
	return nil
}

func (c *victoriaCollector) write(e victoriaEvent) error {
	if len(e.Message) == 0 || len(e.Message) > 1<<20 {
		return fmt.Errorf("victorialogs: message must contain 1 to 1MiB bytes")
	}
	if e.App == "" {
		e.App = c.app
	}
	data := otlpEncodeRequest(e)
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewReader(data))
		if err != nil {
			return err
		}
		if c.password != "" {
			credentials := base64.StdEncoding.EncodeToString([]byte(c.username + ":" + c.password))
			req.Header.Set("Authorization", "Basic "+credentials)
		} else {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		req.Header.Set("Content-Type", otlpContentType)
		resp, err := c.client.Do(req)
		if err != nil {
			return fmt.Errorf("victorialogs: collector request failed: %w", err)
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			return nil
		}
		if resp.StatusCode != http.StatusServiceUnavailable || attempt == 2 {
			return fmt.Errorf("victorialogs: collector returned HTTP %d", resp.StatusCode)
		}
		time.Sleep(time.Duration(100*(1<<attempt)) * time.Millisecond)
	}
	return nil
}

// otlpEncodeRequest builds an OTLP ExportLogsServiceRequest:
//
//	ResourceLogs{ resource{service.name}, scope_logs{ scope{name}, log_record } }
//
// Field numbers follow opentelemetry-proto logs/v1 and resource/v1.
func otlpEncodeRequest(e victoriaEvent) []byte {
	scope := otlpAppendString(nil, 1, otlpScopeName)
	var scopeLogs []byte
	scopeLogs = otlpAppendMessage(scopeLogs, 1, scope)
	scopeLogs = otlpAppendMessage(scopeLogs, 2, otlpEncodeLogRecord(e))

	var resource []byte
	resource = otlpAppendMessage(resource, 1, otlpEncodeKeyValue("service.name", e.App))

	var resourceLogs []byte
	resourceLogs = otlpAppendMessage(resourceLogs, 1, resource)
	resourceLogs = otlpAppendMessage(resourceLogs, 2, scopeLogs)

	// ExportLogsServiceRequest.resource_logs = 1
	return otlpAppendMessage(nil, 1, resourceLogs)
}

func otlpEncodeLogRecord(e victoriaEvent) []byte {
	severityNumber, severityText := otlpSeverity(e.Level)
	var record []byte
	record = otlpAppendFixed64(record, otlpFieldTime, otlpUnixNano(e.Timestamp))
	record = otlpAppendVarint(record, otlpFieldSeverity, uint64(severityNumber))
	record = otlpAppendString(record, otlpFieldSevText, severityText)
	// AnyValue.string_value = 1
	record = otlpAppendMessage(record, otlpFieldBody, otlpAppendString(nil, 1, e.Message))
	for _, attr := range otlpAttributes(e) {
		record = otlpAppendMessage(record, otlpFieldAttrs, otlpEncodeKeyValue(attr.key, attr.value))
	}
	return record
}

var otlpReservedFieldKeys = map[string]bool{
	"app": true, "type": true, "time": true, "level": true, "message": true, "request_id": true,
}

// otlpAttributes keeps the routing fields explicit and flattens the structured
// payload into OTLP attributes, so entry/record fields stay queryable.
func otlpAttributes(e victoriaEvent) []otlpAttribute {
	attrs := []otlpAttribute{
		{key: "app", value: e.App},
		{key: "type", value: e.Type},
	}
	if e.Level != "" {
		// Keep the legacy field name queryable alongside severity_text.
		attrs = append(attrs, otlpAttribute{key: "level", value: e.Level})
	}
	if e.RequestID != "" {
		attrs = append(attrs, otlpAttribute{key: "request_id", value: e.RequestID})
	}
	if e.Fields == nil {
		return attrs
	}
	raw, err := json.Marshal(e.Fields)
	if err != nil {
		return attrs
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return append(attrs, otlpAttribute{key: "fields", value: string(raw)})
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		if !otlpReservedFieldKeys[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		if fields[key] == nil {
			continue
		}
		attrs = append(attrs, otlpAttribute{key: key, value: fields[key]})
	}
	return attrs
}

func otlpEncodeKeyValue(key string, value any) []byte {
	var kv []byte
	kv = otlpAppendString(kv, 1, key)
	kv = otlpAppendMessage(kv, 2, otlpEncodeAnyValue(value))
	return kv
}

func otlpEncodeAnyValue(v any) []byte {
	switch value := v.(type) {
	case string:
		return otlpAppendString(nil, 1, value)
	case bool:
		if value {
			return otlpAppendVarint(nil, 2, 1)
		}
		return otlpAppendVarint(nil, 2, 0)
	case float64:
		if value == math.Trunc(value) && math.Abs(value) < 1<<53 {
			return otlpAppendVarint(nil, 3, uint64(int64(value)))
		}
		return otlpAppendDouble(nil, 4, value)
	case []any:
		var array []byte
		for _, item := range value {
			if item != nil {
				array = otlpAppendMessage(array, 1, otlpEncodeAnyValue(item))
			}
		}
		return otlpAppendMessage(nil, 5, array)
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			if value[key] != nil {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		var list []byte
		for _, key := range keys {
			list = otlpAppendMessage(list, 1, otlpEncodeKeyValue(key, value[key]))
		}
		return otlpAppendMessage(nil, 6, list)
	default:
		return otlpAppendString(nil, 1, fmt.Sprint(value))
	}
}

func otlpSeverity(level string) (int, string) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "trace":
		return 1, "TRACE"
	case "debug":
		return 5, "DEBUG"
	case "warn", "warning":
		return 13, "WARN"
	case "error":
		return 17, "ERROR"
	case "fatal", "critical", "crit", "panic":
		return 21, "FATAL"
	default:
		return 9, "INFO"
	}
}

func otlpUnixNano(timestamp string) uint64 {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05"} {
		if t, err := time.ParseInLocation(layout, timestamp, time.Local); err == nil {
			return uint64(t.UnixNano())
		}
	}
	return uint64(time.Now().UnixNano())
}

func otlpAppendString(buf []byte, field protowire.Number, value string) []byte {
	buf = protowire.AppendTag(buf, field, protowire.BytesType)
	return protowire.AppendString(buf, value)
}

func otlpAppendMessage(buf []byte, field protowire.Number, message []byte) []byte {
	buf = protowire.AppendTag(buf, field, protowire.BytesType)
	return protowire.AppendBytes(buf, message)
}

func otlpAppendVarint(buf []byte, field protowire.Number, value uint64) []byte {
	buf = protowire.AppendTag(buf, field, protowire.VarintType)
	return protowire.AppendVarint(buf, value)
}

func otlpAppendFixed64(buf []byte, field protowire.Number, value uint64) []byte {
	buf = protowire.AppendTag(buf, field, protowire.Fixed64Type)
	return protowire.AppendFixed64(buf, value)
}

func otlpAppendDouble(buf []byte, field protowire.Number, value float64) []byte {
	return otlpAppendFixed64(buf, field, math.Float64bits(value))
}
