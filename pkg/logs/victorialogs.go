package logs

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/maczh/mgin/v2/pkg/logsink"
	"google.golang.org/protobuf/encoding/protowire"
)

const VictoriaLogsTarget = "victorialogs"
const VictoriaLogsClientTarget = "victorialogs-client"

const victoriaOTLPLogsPath = "/insert/opentelemetry/v1/logs"

type victoriaEvent struct {
	App       string `json:"app"`
	Type      string `json:"type"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp,omitempty"`
	RequestID string `json:"request_id,omitempty"`
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

// InitVictoriaLogs installs destinations selected by YAML. Invalid TLS or auth
// configuration fails startup rather than silently losing redirected logs.
func InitVictoriaLogs() error {
	cfg := config.Config.Log.VictoriaLogs
	console := false
	for _, out := range config.Config.Logger.Outputs {
		if out.Target == VictoriaLogsTarget {
			console = true
		}
	}
	if !cfg.Request && !cfg.Client && !console {
		return nil
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`).MatchString(config.Config.App.Name) {
		return fmt.Errorf("victorialogs: invalid application name")
	}
	u, err := url.Parse(cfg.URL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("victorialogs: url must be an HTTPS collector base URL")
	}
	if strings.TrimSpace(cfg.APIToken) == "" && strings.TrimSpace(cfg.HTTPPass) == "" {
		return fmt.Errorf("victorialogs: api_token is required")
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
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true} // #nosec G402 -- explicitly requested for VictoriaLogs
	c := &victoriaCollector{client: &http.Client{Transport: transport, Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		endpoint: strings.TrimRight(cfg.URL, "/") + victoriaOTLPLogsPath, token: cfg.APIToken, app: config.Config.App.Name}
	if cfg.HTTPPass != "" {
		c.username, c.password = cfg.HTTPUser, cfg.HTTPPass
		if c.username == "" {
			c.username = "vlogs"
		}
	}
	if cfg.Request {
		h := logsink.NewHandlerFunc(VictoriaLogsTarget, func(e *logsink.Entry) error {
			return c.write(victoriaEvent{App: c.app, Type: "request", Level: "info", Message: e.Method + " " + e.Uri,
				Timestamp: e.Time, RequestID: e.RequestId, Fields: e})
		}).WithClose(func() error { transport.CloseIdleConnections(); return nil })
		if err := logsink.Register(h, logsink.WithQueueSize(config.Config.Log.Sink.Queue)); err != nil {
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
		if err := logsink.RegisterRecord(h, logsink.WithQueueSize(config.Config.Log.Sink.Queue)); err != nil {
			return err
		}
	}
	return nil
}

func (c *victoriaCollector) write(e victoriaEvent) error {
	if len(e.Message) == 0 || len(e.Message) > 1<<20 {
		return fmt.Errorf("victorialogs: message must contain 1 to 1MiB bytes")
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
		req.Header.Set("Content-Type", "application/x-protobuf")
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

const (
	otlpScopeName                          = "mgin"
	otlpFieldTime         protowire.Number = 1
	otlpFieldSeverity     protowire.Number = 2
	otlpFieldSeverityText protowire.Number = 3
	otlpFieldBody         protowire.Number = 5
	otlpFieldAttributes   protowire.Number = 6
)

type otlpAttribute struct {
	key   string
	value any
}

func otlpEncodeRequest(e victoriaEvent) []byte {
	scope := otlpAppendString(nil, 1, otlpScopeName)
	scopeLogs := otlpAppendMessage(nil, 1, scope)
	scopeLogs = otlpAppendMessage(scopeLogs, 2, otlpEncodeLogRecord(e))
	resource := otlpAppendMessage(nil, 1, otlpEncodeKeyValue("service.name", e.App))
	resourceLogs := otlpAppendMessage(nil, 1, resource)
	resourceLogs = otlpAppendMessage(resourceLogs, 2, scopeLogs)
	return otlpAppendMessage(nil, 1, resourceLogs)
}

func otlpEncodeLogRecord(e victoriaEvent) []byte {
	severity, text := otlpSeverity(e.Level)
	var record []byte
	record = otlpAppendFixed64(record, otlpFieldTime, otlpUnixNano(e.Timestamp))
	record = otlpAppendVarint(record, otlpFieldSeverity, uint64(severity))
	record = otlpAppendString(record, otlpFieldSeverityText, text)
	record = otlpAppendMessage(record, otlpFieldBody, otlpAppendString(nil, 1, e.Message))
	for _, attr := range otlpAttributes(e) {
		record = otlpAppendMessage(record, otlpFieldAttributes, otlpEncodeKeyValue(attr.key, attr.value))
	}
	return record
}

func otlpAttributes(e victoriaEvent) []otlpAttribute {
	attrs := []otlpAttribute{{"app", e.App}, {"type", e.Type}}
	if e.Level != "" {
		attrs = append(attrs, otlpAttribute{"level", e.Level})
	}
	if e.RequestID != "" {
		attrs = append(attrs, otlpAttribute{"request_id", e.RequestID})
	}
	if e.Fields == nil {
		return attrs
	}
	raw, err := json.Marshal(e.Fields)
	if err != nil {
		return attrs
	}
	var fields map[string]any
	if json.Unmarshal(raw, &fields) != nil {
		return append(attrs, otlpAttribute{"fields", string(raw)})
	}
	for key, value := range fields {
		if key == "app" || key == "type" || key == "level" || key == "message" || key == "request_id" || value == nil {
			continue
		}
		attrs = append(attrs, otlpAttribute{key, value})
	}
	return attrs
}

func otlpEncodeKeyValue(key string, value any) []byte {
	var kv []byte
	kv = otlpAppendString(kv, 1, key)
	return otlpAppendMessage(kv, 2, otlpEncodeAnyValue(value))
}

func otlpEncodeAnyValue(value any) []byte {
	switch v := value.(type) {
	case string:
		return otlpAppendString(nil, 1, v)
	case bool:
		if v {
			return otlpAppendVarint(nil, 2, 1)
		}
		return otlpAppendVarint(nil, 2, 0)
	case float64:
		if v == math.Trunc(v) && math.Abs(v) < 1<<53 {
			return otlpAppendVarint(nil, 3, uint64(int64(v)))
		}
		return otlpAppendDouble(nil, 4, v)
	case []any:
		var array []byte
		for _, item := range v {
			if item != nil {
				array = otlpAppendMessage(array, 1, otlpEncodeAnyValue(item))
			}
		}
		return otlpAppendMessage(nil, 5, array)
	case map[string]any:
		var list []byte
		for key, item := range v {
			if item != nil {
				list = otlpAppendMessage(list, 1, otlpEncodeKeyValue(key, item))
			}
		}
		return otlpAppendMessage(nil, 6, list)
	default:
		return otlpAppendString(nil, 1, fmt.Sprint(v))
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
