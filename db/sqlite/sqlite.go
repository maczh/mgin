package sqlite

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/logs"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Sqlite struct {
	mu     sync.RWMutex
	sqlite *gorm.DB
	dbFile string
	closed bool
}

var logger = logs.GetLogger()

func (m *Sqlite) Init(dbFileName string) {
	if err := m.InitWithError(dbFileName); err != nil {
		logger.Error("SQLite初始化失败: " + err.Error())
	}
}

// InitWithError 初始化 SQLite，并向调用方返回打开失败原因；旧 Init 签名仍保留兼容。
func (m *Sqlite) InitWithError(dbFileName string) error {
	if m == nil {
		return errors.New("SQLite client is nil")
	}
	m.mu.RLock()
	closed, opened := m.closed, m.sqlite != nil
	m.mu.RUnlock()
	if closed && opened {
		return errors.New("SQLite client is closed")
	}
	if opened {
		return nil
	}
	if config.Config == nil {
		return errors.New("configuration is not initialized")
	}
	executable := "."
	if len(os.Args) > 0 && os.Args[0] != "" {
		executable = os.Args[0]
	}
	path, _ := filepath.Abs(filepath.Dir(executable))
	if dbFileName == "" {
		dbFileName = fmt.Sprintf("%s/%s.db", path, config.Config.App.Name)
	} else if len(dbFileName) < 2 || !(dbFileName[0] == '/' || dbFileName[1] == ':') {
		dbFileName = fmt.Sprintf("%s/%s", path, dbFileName)
	}
	conn, err := gorm.Open(sqlite.Open(dbFileName), &gorm.Config{Logger: logs.NewGORMLogger()})
	if err != nil {
		return err
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		if sqlDB, dbErr := conn.DB(); dbErr == nil && sqlDB != nil {
			_ = sqlDB.Close()
		}
		return errors.New("SQLite client is closed")
	}
	published := m.sqlite == nil
	if published {
		m.sqlite, m.dbFile, m.closed = conn, dbFileName, false
	}
	m.mu.Unlock()
	if !published {
		if sqlDB, dbErr := conn.DB(); dbErr == nil && sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
	return nil
}

func (m *Sqlite) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.closed = true
	conn := m.sqlite
	m.sqlite = nil
	m.mu.Unlock()
	if conn != nil {
		if db, err := conn.DB(); err == nil && db != nil {
			if closeErr := db.Close(); closeErr != nil {
				logger.Error("SQLite关闭失败: " + closeErr.Error())
			}
		}
	}
}

func (m *Sqlite) Check() error {
	if m == nil {
		return errors.New("SQLite client is nil")
	}
	conn, err := m.GetConnection()
	if err != nil {
		return err
	}
	sqldb, err := conn.DB()
	if err != nil {
		return fmt.Errorf("SQLite获取连接失败: %w", err)
	}
	if err = sqldb.Ping(); err != nil {
		return fmt.Errorf("SQLite连接检查失败: %w", err)
	}
	return nil
}

func (m *Sqlite) GetConnection() (*gorm.DB, error) {
	if m == nil {
		return nil, errors.New("SQLite client is nil")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, errors.New("SQLite client is closed")
	}
	if m.sqlite == nil {
		return nil, errors.New("SQLite not opened")
	}
	return m.sqlite, nil
}
