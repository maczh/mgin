package config

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ValidationError describes a rejected configuration key.
type ValidationError struct{ Key, Message string }

func (e ValidationError) Error() string { return e.Key + ": " + e.Message }

// Validate checks the safety-critical v1.26 settings without requiring any
// optional component. It is intentionally additive and can be called before Init.
func (c *config) Validate() []ValidationError {
	if c == nil || c.Cnf == nil {
		return []ValidationError{{Key: "config", Message: "configuration is not loaded"}}
	}
	var out []ValidationError
	if c.App.Port < 0 || c.App.Port > 65535 {
		out = append(out, ValidationError{"go.application.port", "must be between 0 and 65535"})
	}
	if c.App.PortSSL < 0 || c.App.PortSSL > 65535 {
		out = append(out, ValidationError{"go.application.port_ssl", "must be between 0 and 65535"})
	}
	if c.Config.Type == "" && c.Config.Server != "" {
		out = append(out, ValidationError{"go.config.server_type", "is required when config server is set"})
	}
	return out
}

// ValidateCurrent returns a stable error suitable for startup gates.
func ValidateCurrent() error {
	items := Config.Validate()
	if len(items) == 0 {
		return nil
	}
	return errors.Join(func() []error {
		r := make([]error, len(items))
		for i := range items {
			r[i] = items[i]
		}
		return r
	}()...)
}

type ConfigRevision struct {
	Version   uint64
	Hash      string
	ChangedAt time.Time
	Actor     string
	Snapshot  []byte
}

// RevisionStore is a bounded, concurrency-safe audit and rollback history.
type RevisionStore struct {
	mu    sync.RWMutex
	limit int
	next  uint64
	items []ConfigRevision
}

func NewRevisionStore(limit int) *RevisionStore {
	if limit < 1 {
		limit = 20
	}
	return &RevisionStore{limit: limit}
}
func (s *RevisionStore) Record(snapshot []byte, actor string) ConfigRevision {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	sum := sha256.Sum256(snapshot)
	r := ConfigRevision{s.next, hex.EncodeToString(sum[:]), time.Now(), actor, append([]byte(nil), snapshot...)}
	s.items = append(s.items, r)
	if len(s.items) > s.limit {
		s.items = s.items[len(s.items)-s.limit:]
	}
	return r
}
func (s *RevisionStore) List() []ConfigRevision {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]ConfigRevision(nil), s.items...)
}
func (s *RevisionStore) Get(version uint64) (ConfigRevision, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.items {
		if r.Version == version {
			return r, true
		}
	}
	return ConfigRevision{}, false
}
func (s *RevisionStore) Rollback(version uint64) ([]byte, error) {
	r, ok := s.Get(version)
	if !ok {
		return nil, fmt.Errorf("config revision %d not found", version)
	}
	return append([]byte(nil), r.Snapshot...), nil
}
