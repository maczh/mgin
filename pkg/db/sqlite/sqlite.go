package sqlite

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/maczh/mgin/v2/pkg/config"
	"github.com/maczh/mgin/v2/pkg/logs"
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
	} else if !filepath.IsAbs(dbFileName) && !(len(dbFileName) >= 2 && dbFileName[1:2] == ":") {
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
		db, _ := m.sqlite.DB()
		db.Close()
		m.sqlite = nil
	}
}

func (m *Sqlite) Check() error {
	return nil
}

func (m *Sqlite) GetConnection() (*gorm.DB, error) {
	if m.sqlite == nil {
		return nil, errors.New("SQLite not opened")
	}
	return m.sqlite, nil
}
