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
}

var logger = logs.GetLogger()

func (m *Sqlite) Init(dbFileName string) {
	if m == nil || config.Config == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	path, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	if dbFileName == "" {
		dbFileName = fmt.Sprintf("%s/%s.db", path, config.Config.App.Name)
	} else if !filepath.IsAbs(dbFileName) && !(len(dbFileName) >= 2 && dbFileName[1:2] == ":") {
		dbFileName = fmt.Sprintf("%s/%s", path, dbFileName)
	}
	m.dbFile = dbFileName
	if m.sqlite == nil {
		var err error
		m.sqlite, err = gorm.Open(sqlite.Open(m.dbFile), &gorm.Config{Logger: logs.NewGORMLogger()})
		if err != nil {
			m.sqlite = nil
			logger.Error("SQLite打开失败: " + err.Error())
		}
	}
}

func (m *Sqlite) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sqlite != nil {
		if db, err := m.sqlite.DB(); err == nil && db != nil {
			_ = db.Close()
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
	if m.sqlite == nil {
		return errors.New("SQLite not opened")
	}
	db, err := m.sqlite.DB()
	if err != nil || db == nil {
		return errors.New("SQLite connection unavailable")
	}
	return db.Ping()
}

func (m *Sqlite) GetConnection() (*gorm.DB, error) {
	if m == nil {
		return nil, errors.New("SQLite is nil")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.sqlite == nil {
		return nil, errors.New("SQLite not opened")
	}
	return m.sqlite, nil
}
