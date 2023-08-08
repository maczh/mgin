package sqlite

import (
	"errors"
	"fmt"
	"github.com/maczh/mgin/config"
	"github.com/sadlil/gologger"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"os"
	"path/filepath"
)

type Sqlite struct {
	sqlite *gorm.DB
	dbFile string
}

var logger = gologger.GetLogger()

func (m *Sqlite) Init(dbFileName string) {
	path, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	if dbFileName == "" {
		dbFileName = fmt.Sprintf("%s/%s.db", path, config.Config.App.Name)
	} else if !(dbFileName[:1] == "/" || dbFileName[1:2] == ":") {
		dbFileName = fmt.Sprintf("%s/%s", path, dbFileName)
	}
	m.dbFile = dbFileName
	if m.sqlite == nil {
		m.sqlite, _ = gorm.Open(sqlite.Open(m.dbFile), &gorm.Config{})
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
