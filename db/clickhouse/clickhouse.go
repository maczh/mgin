package clickhouse

import (
	"errors"
	"strings"
	"time"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/sadlil/gologger"
	"gorm.io/driver/clickhouse"
	"gorm.io/gorm"
)

type ClickhouseClient struct {
	db       *gorm.DB
	dbs      map[string]*gorm.DB
	multi    bool
	conf     *koanf.Koanf
	confUrl  string
	confData []byte
	conns    []string
}

var logger = gologger.GetLogger()

func (m *ClickhouseClient) Init(clickhouseConfigData []byte) {
	if clickhouseConfigData != nil && len(clickhouseConfigData) > 0 {
		m.confData = clickhouseConfigData
		m.conf = koanf.New(".")
		err := m.conf.Load(rawbytes.Provider(clickhouseConfigData), yaml.Parser())
		if err != nil {
			logger.Error("ClickHouse配置格式解析错误:" + err.Error())
			m.conf = nil
			return
		}

		m.multi = false
		if m.conf.Exists("go.data.clickhouse.multidb") && m.conf.Bool("go.data.clickhouse.multidb") {
			m.multi = true
			m.dbs = make(map[string]*gorm.DB)
			m.conns = make([]string, 0)
			dbNames := strings.Split(m.conf.String("go.data.clickhouse.dbNames"), ",")
			for _, dbName := range dbNames {
				if dbName != "" && m.conf.String("go.data.clickhouse."+dbName) != "" {
					conn, err := gorm.Open(clickhouse.Open(m.conf.String("go.data.clickhouse."+dbName)), &gorm.Config{})
					if err != nil {
						logger.Error(dbName + " clickhouse connection error:" + err.Error())
						continue
					}
					m.dbs[dbName] = conn
					m.conns = append(m.conns, dbName)
				}
			}
		} else {
			m.db, err = gorm.Open(clickhouse.Open(m.conf.String("go.data.clickhouse")), &gorm.Config{})
			if err != nil {
				logger.Error("clickhouse connection error:" + err.Error())
				return
			}
		}

		if m.db == nil && !m.multi {
			logger.Error("clickhouse connection error")
			return
		}

		if m.conf.Bool("go.data.clickhouse_debug") {
			if m.multi {
				for k := range m.dbs {
					m.dbs[k] = m.dbs[k].Debug()
				}
			} else {
				m.db = m.db.Debug()
			}
		}

		if m.conf.Int("go.data.clickhouse_pool.max") > 1 {
			max := m.conf.Int("go.data.clickhouse_pool.max")
			if max < 10 {
				max = 10
			}
			idle := m.conf.Int("go.data.clickhouse_pool.total")
			if idle == 0 || idle < max {
				idle = 5 * max
			}
			idleTimeout := m.conf.Int("go.data.clickhouse_pool.timeout")
			if idleTimeout == 0 {
				idleTimeout = 60
			}
			lifetime := m.conf.Int("go.data.clickhouse_pool.life")
			if lifetime == 0 {
				lifetime = 60
			}
			if !m.multi {
				sqldb, _ := m.db.DB()
				sqldb.SetConnMaxIdleTime(time.Duration(idleTimeout) * time.Second)
				sqldb.SetMaxIdleConns(idle)
				sqldb.SetMaxOpenConns(max)
				sqldb.SetConnMaxLifetime(time.Duration(lifetime) * time.Minute)
			} else {
				for k := range m.dbs {
					sqldb, _ := m.dbs[k].DB()
					sqldb.SetConnMaxIdleTime(time.Duration(idleTimeout) * time.Second)
					sqldb.SetMaxIdleConns(idle)
					sqldb.SetMaxOpenConns(max)
					sqldb.SetConnMaxLifetime(time.Duration(lifetime) * time.Minute)
				}
			}
		}
	}
}

func (m *ClickhouseClient) Close() {
	if m.multi {
		for k := range m.dbs {
			sqldb, _ := m.dbs[k].DB()
			sqldb.Close()
			delete(m.dbs, k)
		}
	} else if m.db != nil {
		sqldb, _ := m.db.DB()
		sqldb.Close()
		m.db = nil
	}
}

func clickhousesCheck(m *ClickhouseClient) error {
	if !m.multi {
		return errors.New("Not multi clickhouse connections setting")
	}
	if len(m.dbs) == 0 {
		m.Init(m.confData)
		if len(m.dbs) == 0 {
			return errors.New("ClickHouse connection error")
		}
	}
	for k := range m.dbs {
		sqldb, _ := m.dbs[k].DB()
		err := sqldb.Ping()
		if err != nil {
			m.Close()
			m.Init(m.confData)
			if len(m.dbs) == 0 {
				return errors.New("ClickHouse connection error")
			}
		}
	}
	return nil
}

func clickhouseCheck(m *ClickhouseClient) (*gorm.DB, error) {
	if m.db == nil {
		m.Init(m.confData)
		if m.db == nil {
			return nil, errors.New("ClickHouse connection error")
		}
	}
	if sqldb, _ := m.db.DB(); sqldb != nil {
		err := sqldb.Ping()
		if err != nil {
			m.Close()
			m.Init(m.confData)
			if m.db == nil {
				return nil, errors.New("ClickHouse connection error")
			}
		}
	}
	return m.db, nil
}

func (m *ClickhouseClient) Check() error {
	var err error
	if m.multi {
		err = clickhousesCheck(m)
		if err != nil {
			logger.Error(err.Error())
		}
	} else {
		_, err = clickhouseCheck(m)
		if err != nil {
			logger.Error(err.Error())
		}
	}
	return err
}

func (m *ClickhouseClient) GetConnection(dbName ...string) (*gorm.DB, error) {
	if len(dbName) == 0 {
		if m.multi {
			return nil, errors.New("multi get connection must specify a database name")
		}
		return clickhouseCheck(m)
	}
	if len(dbName) > 1 {
		return nil, errors.New("Multidb can only get one connection")
	}
	if !m.multi {
		return clickhouseCheck(m)
	}
	conn := m.dbs[dbName[0]]
	if conn == nil {
		return nil, errors.New(dbName[0] + " clickhouse connection not found or failed")
	}
	return conn, nil
}

func (m *ClickhouseClient) IsMultiDB() bool {
	return m.multi
}

func (m *ClickhouseClient) ListConnNames() []string {
	return m.conns
}
