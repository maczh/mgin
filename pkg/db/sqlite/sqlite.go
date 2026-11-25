package sqlite

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/maczh/mgin/v2/pkg/logs"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Sqlite struct {
	mu     sync.RWMutex
	sqlite *gorm.DB
	dbFile string
	closed bool
}

var ErrClosed = errors.New("SQLite client is closed")

var logger = logs.GetLogger()

func validateSqlite(conn *gorm.DB) error {
	if conn == nil || conn.Config == nil || conn.Statement == nil || conn.Error != nil {
		return errors.New("SQLite connection unavailable")
	}
	db, err := conn.DB()
	if err != nil || db == nil {
		return errors.New("SQLite connection unavailable")
	}
	return nil
}

func (m *Sqlite) Init(dbFileName string) {
	_ = m.InitWithError(dbFileName)
}

func (m *Sqlite) InitWithError(dbFileName string) error {
	if m == nil {
		return errors.New("SQLite configuration is unavailable")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	if config.Config == nil {
		return errors.New("SQLite configuration is unavailable")
	}
	path, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return err
	}
	if dbFileName == "" {
		dbFileName = fmt.Sprintf("%s/%s.db", path, config.Config.App.Name)
	} else if !filepath.IsAbs(dbFileName) && !(len(dbFileName) >= 2 && dbFileName[1:2] == ":") {
		dbFileName = fmt.Sprintf("%s/%s", path, dbFileName)
	}
	m.dbFile = dbFileName
	if m.sqlite == nil {
		m.sqlite, err = gorm.Open(sqlite.Open(m.dbFile), &gorm.Config{Logger: logs.NewGORMLogger()})
		if err != nil {
			m.sqlite = nil
			logger.Error("SQLite打开失败: " + err.Error())
			return err
		}
	}
	return nil
}

func (m *Sqlite) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	if m.sqlite != nil {
		if m.sqlite.Config != nil && m.sqlite.Statement != nil {
			if db, err := m.sqlite.DB(); err != nil {
				logger.Error(fmt.Sprintf("SQLite底层连接获取失败:%s", err.Error()))
			} else if db != nil {
				if err := db.Close(); err != nil {
					logger.Error(fmt.Sprintf("SQLite底层连接关闭失败:%s", err.Error()))
				}
			}
		}
		m.sqlite = nil
	}
}

func (m *Sqlite) Check() error {
	if m == nil {
		return errors.New("SQLite is nil")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return ErrClosed
	}
	if m.sqlite == nil {
		return errors.New("SQLite not opened")
	}
	if err := validateSqlite(m.sqlite); err != nil {
		return err
	}
	db, _ := m.sqlite.DB()
	return db.Ping()
}

func (m *Sqlite) GetConnection() (*gorm.DB, error) {
	if m == nil {
		return nil, errors.New("SQLite is nil")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, ErrClosed
	}
	if m.sqlite == nil {
		return nil, errors.New("SQLite not opened")
	}
	if err := validateSqlite(m.sqlite); err != nil {
		return nil, err
	}
	return m.sqlite, nil
}
