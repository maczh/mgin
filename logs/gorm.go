package logs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// GORMLogger routes GORM diagnostics through the application's logs configuration.
// LogMode returns a copy so per-session Debug does not mutate shared state.
type GORMLogger struct {
	Level                     logger.LogLevel
	SlowThreshold             time.Duration
	IgnoreRecordNotFoundError bool
}

var _ logger.Interface = (*GORMLogger)(nil)

// NewGORMLogger creates a GORM logger with GORM's default warning threshold.
func NewGORMLogger() *GORMLogger {
	return &GORMLogger{Level: logger.Warn, SlowThreshold: 200 * time.Millisecond, IgnoreRecordNotFoundError: true}
}

// LogMode implements logger.Interface.
func (l *GORMLogger) LogMode(level logger.LogLevel) logger.Interface {
	copy := *l
	copy.Level = level
	return &copy
}

// Info implements logger.Interface. GORM uses printf placeholders.
func (l *GORMLogger) Info(ctx context.Context, msg string, args ...interface{}) {
	if l.Level >= logger.Info {
		Info("{}", fmt.Sprintf(msg, args...))
	}
}

// Warn implements logger.Interface.
func (l *GORMLogger) Warn(ctx context.Context, msg string, args ...interface{}) {
	if l.Level >= logger.Warn {
		Warn("{}", fmt.Sprintf(msg, args...))
	}
}

// Error implements logger.Interface.
func (l *GORMLogger) Error(ctx context.Context, msg string, args ...interface{}) {
	if l.Level >= logger.Error {
		Error("{}", fmt.Sprintf(msg, args...))
	}
}

// Trace implements logger.Interface, evaluating SQL only when it will be logged.
func (l *GORMLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.Level == logger.Silent {
		return
	}
	elapsed := time.Since(begin)
	level := ""
	switch {
	case err != nil && l.Level >= logger.Error && (!errors.Is(err, gorm.ErrRecordNotFound) || !l.IgnoreRecordNotFoundError):
		level = "error"
	case l.SlowThreshold > 0 && elapsed > l.SlowThreshold && l.Level >= logger.Warn:
		level = "warn"
	case l.Level >= logger.Info:
		level = "info"
	}
	if level == "" || !enabled(level) {
		return
	}
	sql, rows := fc()
	message := fmt.Sprintf("[gorm] [%.3fms] [rows:%d] %s", float64(elapsed.Microseconds())/1000, rows, sql)
	if err != nil {
		message += " error=" + err.Error()
	}
	emit(level, "{}", []any{message})
}
