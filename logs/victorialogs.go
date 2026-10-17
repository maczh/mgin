package logs

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/logsink"
)

const VictoriaLogsTarget = "victorialogs"
const VictoriaLogsClientTarget = "victorialogs-client"

type victoriaEvent struct {
	App       string `json:"app"`
	Type      string `json:"type"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Fields    any    `json:"fields,omitempty"`
}

type victoriaCollector struct {
	client   *http.Client
	endpoint string
	token    string
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
	if strings.TrimSpace(cfg.APIToken) == "" {
		return fmt.Errorf("victorialogs: api_token is required")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(cfg.APICert)) {
		return fmt.Errorf("victorialogs: api_crt must contain a PEM certificate")
	}
	timeout := time.Duration(cfg.Timeout) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	c := &victoriaCollector{client: &http.Client{Transport: transport, Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		endpoint: strings.TrimRight(cfg.URL, "/") + "/v1/logs", token: cfg.APIToken, app: config.Config.App.Name}
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
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewReader(data))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.client.Do(req)
		if err != nil {
			return fmt.Errorf("victorialogs: collector request failed: %w", err)
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if resp.StatusCode == http.StatusAccepted {
			return nil
		}
		if resp.StatusCode != http.StatusServiceUnavailable || attempt == 2 {
			return fmt.Errorf("victorialogs: collector returned HTTP %d", resp.StatusCode)
		}
		time.Sleep(time.Duration(100*(1<<attempt)) * time.Millisecond)
	}
	return nil
}
