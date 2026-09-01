package sqlite

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-gorm/caches/v4"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/db/mysql"
	"github.com/maczh/mgin/db/redis"
	"github.com/sadlil/gologger"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

func (m *Sqlite) UseCache() bool {
	if !strings.Contains(config.Config.Config.Used, "redis") {
		logger.Error("SQLite cache use memory cache")
		cachesPlugin := &caches.Caches{
			Conf: &caches.Config{
				Easer: true,
			},
		}
		m.sqlite.Use(cachesPlugin)
		return true
	}
	rds, err := redis.Redis.GetConnection()
	if err != nil {
		logger.Error("MySQL init Redis Cache connection error: " + err.Error())
		return false
	}
	cachesPlugin := &caches.Caches{
		Conf: &caches.Config{
			Cacher: &mysql.RedisCacher{
				Rdb:        rds,
				Expiration: 5 * time.Minute,
			},
		},
	}
	m.sqlite.Use(cachesPlugin)
	return true
}
