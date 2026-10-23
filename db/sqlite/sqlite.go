package sqlite

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/logs"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Sqlite struct {
	sqlite *gorm.DB
	dbFile string
}

var logger = logs.GetLogger()

func (m *Sqlite) Init(dbFileName string) {
	path, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	if dbFileName == "" {
		dbFileName = fmt.Sprintf("%s/%s.db", path, config.Config.App.Name)
	} else if len(dbFileName) < 2 || !(dbFileName[0] == '/' || dbFileName[1] == ':') {
		dbFileName = fmt.Sprintf("%s/%s", path, dbFileName)
	}
	m.dbFile = dbFileName
	if m.sqlite == nil {
		var err error
		m.sqlite, err = gorm.Open(sqlite.Open(m.dbFile), &gorm.Config{Logger: logs.NewGORMLogger()})
		if err != nil {
			logger.Error("SQLite打开失败: " + err.Error())
		}
	}
}

func (m *Sqlite) Close() {
	if m.sqlite != nil {
		if db, err := m.sqlite.DB(); err == nil && db != nil {
			db.Close()
		}
		m.sqlite = nil
	}
}

func (m *Sqlite) Check() error {
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
	if m.sqlite == nil {
		return nil, errors.New("SQLite not opened")
	}
	return m.sqlite, nil
}
