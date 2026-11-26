package mysql

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/maczh/mgin/logs"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type MysqlClient struct {
	mu       sync.RWMutex
	initMu   sync.Mutex
	mysql    *gorm.DB
	mysqls   map[string]*gorm.DB
	multi    bool
	conf     *koanf.Koanf
	confUrl  string
	confData []byte
	conns    []string
	cache    bool
	closed   bool
}

var logger = logs.GetLogger()

var ErrClosed = errors.New("mysql client is closed")

func (m *MysqlClient) Init(mysqlConfigData []byte) {
	if err := m.InitWithError(mysqlConfigData); err != nil {
		logger.Error("MySQL初始化失败: " + err.Error())
	}
}

// InitWithError initializes a candidate connection set and publishes it only
// after configuration parsing and connection setup succeed.
func (m *MysqlClient) InitWithError(mysqlConfigData []byte) error {
	if m == nil {
		return ErrClosed
	}
	m.initMu.Lock()
	defer m.initMu.Unlock()
	if len(mysqlConfigData) == 0 {
		return errors.New("mysql configuration is empty")
	}
	candidate := &MysqlClient{}
	if err := candidate.init(mysqlConfigData); err != nil {
		closeConnections(candidate.mysql, candidate.mysqls, candidate.multi)
		return err
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		closeConnections(candidate.mysql, candidate.mysqls, candidate.multi)
		return ErrClosed
	}
	oldSingle, oldMulti, oldIsMulti := m.mysql, m.mysqls, m.multi
	m.mysql, m.mysqls, m.multi = candidate.mysql, candidate.mysqls, candidate.multi
	m.closed = false
	m.conf, m.confData, m.conns = candidate.conf, append([]byte(nil), candidate.confData...), append([]string(nil), candidate.conns...)
	m.mu.Unlock()
	closeConnections(oldSingle, oldMulti, oldIsMulti)
	return nil
}

func (m *MysqlClient) init(mysqlConfigData []byte) error {
	//if mysqlConfigUrl != "" {
	//	m.confUrl = mysqlConfigUrl
	//}
	//if m.confUrl == "" {
	//	logger.Error("MySQL配置文件Url为空")
	//	return
	//}
	if mysqlConfigData != nil && len(mysqlConfigData) > 0 {
		m.confData = mysqlConfigData
		//if m.conf == nil {
		//	var confData []byte
		//	var err error
		//	if strings.HasPrefix(m.confUrl, "http://") {
		//		resp, err := grequests.Get(m.confUrl, nil)
		//		if err != nil {
		//			logger.Error("MySQL配置下载失败! " + err.Error())
		//			return
		//		}
		//		confData = []byte(resp.String())
		//	} else {
		//		confData, err = ioutil.ReadFile(m.confUrl)
		//		if err != nil {
		//			logger.Error(fmt.Sprintf("MySQL本地配置文件%s读取失败:%s", m.confUrl, err.Error()))
		//			return
		//		}
		//	}
		m.conf = koanf.New(".")
		err := m.conf.Load(rawbytes.Provider(mysqlConfigData), yaml.Parser())
		if err != nil {
			logger.Error("MySQL配置格式解析错误:" + err.Error())
			m.conf = nil
			return err
		}
		//}
		m.multi = false
		if m.conf.Exists("go.data.mysql.multidb") && m.conf.Bool("go.data.mysql.multidb") {
			m.multi = true
			m.mysqls = make(map[string]*gorm.DB)
			m.conns = make([]string, 0)
			dbNames := strings.Split(m.conf.String("go.data.mysql.dbNames"), ",")
			for _, dbName := range dbNames {
				if dbName != "" && m.conf.String("go.data.mysql."+dbName) != "" {
					conn, err := gorm.Open(mysql.Open(m.conf.String("go.data.mysql."+dbName)), &gorm.Config{Logger: logs.NewGORMLogger()})
					if err != nil {
						logger.Error(dbName + " mysql connection error:" + err.Error())
						return fmt.Errorf("%s mysql connection error: %w", dbName, err)
					}
					m.mysqls[dbName] = conn
					m.conns = append(m.conns, dbName)
				}
			}
			if len(m.mysqls) == 0 {
				return errors.New("mySQL connection error: no configured connections available")
			}
		} else {
			m.mysql, err = gorm.Open(mysql.Open(m.conf.String("go.data.mysql")), &gorm.Config{Logger: logs.NewGORMLogger()})
			if err != nil {
				logger.Error("mysql connection error:" + err.Error())
				return err
			}
		}
		if !m.multi && m.mysql == nil {
			logger.Error("mysql connection error")
			return errors.New("mySQL connection error")
		}
		if m.conf.Bool("go.data.mysql_debug") {
			if m.multi {
				for k, _ := range m.mysqls {
					m.mysqls[k] = m.mysqls[k].Debug()
				}
			} else {
				m.mysql = m.mysql.Debug()
			}
		}
		if m.conf.Int("go.data.mysql_pool.max") > 1 {
			max := m.conf.Int("go.data.mysql_pool.max")
			if max < 10 {
				max = 10
			}
			idle := m.conf.Int("go.data.mysql_pool.total")
			if idle == 0 || idle < max {
				idle = 5 * max
			}
			idleTimeout := m.conf.Int("go.data.mysql_pool.timeout")
			if idleTimeout == 0 {
				idleTimeout = 60
			}
			lifetime := m.conf.Int("go.data.mysql_pool.life")
			if lifetime == 0 {
				lifetime = 60
			}
			if !m.multi {
				sqldb, err := m.mysql.DB()
				if err != nil || sqldb == nil {
					logger.Error("mysql connection pool error")
					if err != nil {
						return fmt.Errorf("mysql connection pool error: %w", err)
					}
					return errors.New("mysql connection pool is nil")
				}
				sqldb.SetConnMaxIdleTime(time.Duration(idleTimeout) * time.Second)
				sqldb.SetMaxIdleConns(idle)
				sqldb.SetMaxOpenConns(max)
				sqldb.SetConnMaxLifetime(time.Duration(lifetime) * time.Minute)
			} else {
				for k := range m.mysqls {
					if m.mysqls[k] == nil {
						return fmt.Errorf("%s mysql connection is nil", k)
					}
					sqldb, err := m.mysqls[k].DB()
					if err != nil || sqldb == nil {
						if err != nil {
							return fmt.Errorf("%s mysql connection pool error: %w", k, err)
						}
						return fmt.Errorf("%s mysql connection pool is nil", k)
					}
					sqldb.SetConnMaxIdleTime(time.Duration(idleTimeout) * time.Second)
					sqldb.SetMaxIdleConns(idle)
					sqldb.SetMaxOpenConns(max)
					sqldb.SetConnMaxLifetime(time.Duration(lifetime) * time.Minute)
				}
			}
		}
	}
	return nil
}

func (m *MysqlClient) Close() {
	if m == nil {
		return
	}
	m.initMu.Lock()
	defer m.initMu.Unlock()
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	single, multi, isMulti := m.mysql, m.mysqls, m.multi
	m.mysql, m.mysqls, m.conns, m.conf, m.confData = nil, nil, nil, nil, nil
	m.mu.Unlock()
	closeConnections(single, multi, isMulti)
}

func closeConnections(single *gorm.DB, multi map[string]*gorm.DB, isMulti bool) {
	closeDB := func(conn *gorm.DB) {
		if conn != nil {
			if sqlDB, err := conn.DB(); err == nil && sqlDB != nil {
				_ = sqlDB.Close()
			}
		}
	}
	if isMulti {
		for _, conn := range multi {
			closeDB(conn)
		}
		return
	}
	closeDB(single)
}

func mySqlsCheck(m *MysqlClient) error {
	if m == nil {
		return ErrClosed
	}
	m.mu.RLock()
	closed, multi := m.closed, m.multi
	conns := make(map[string]*gorm.DB, len(m.mysqls))
	for name, conn := range m.mysqls {
		conns[name] = conn
	}
	m.mu.RUnlock()
	if closed {
		return ErrClosed
	}
	if !multi {
		return errors.New("Not multi mysql connections setting")
	}
	for name, conn := range conns {
		if conn == nil {
			return errors.New("mySQL connection error: " + name)
		}
		sqldb, err := conn.DB()
		if err != nil || sqldb == nil {
			return errors.New("mySQL connection error: " + name)
		}
		if err = sqldb.Ping(); err != nil {
			return fmt.Errorf("mySQL connection error: %s: %w", name, err)
		}
	}
	return nil
}

func mySqlCheck(m *MysqlClient) (*gorm.DB, error) {
	if m == nil {
		return nil, ErrClosed
	}
	m.mu.RLock()
	closed, conn := m.closed, m.mysql
	m.mu.RUnlock()
	if closed {
		return nil, ErrClosed
	}
	if conn == nil {
		return nil, errors.New("mySQL connection error")
	}
	sqldb, err := conn.DB()
	if err != nil || sqldb == nil {
		return nil, errors.New("mySQL connection error")
	}
	err = sqldb.Ping()
	if err != nil {
		return nil, errors.New("mySQL connection error")
	}
	m.mu.RLock()
	closed = m.closed
	m.mu.RUnlock()
	if closed {
		return nil, ErrClosed
	}
	return conn, nil
}

func (m *MysqlClient) Check() error {
	if m == nil {
		return ErrClosed
	}
	m.mu.RLock()
	closed, multi := m.closed, m.multi
	m.mu.RUnlock()
	if closed {
		return ErrClosed
	}
	var err error
	if multi {
		err = mySqlsCheck(m)
		if err != nil {
			logger.Error(err.Error())
		}
	} else {
		_, err = mySqlCheck(m)
		if err != nil {
			logger.Error(err.Error())
		}
	}
	return err
}

func (m *MysqlClient) GetConnection(dbName ...string) (*gorm.DB, error) {
	if m == nil {
		return nil, ErrClosed
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, ErrClosed
	}
	if len(dbName) == 0 {
		if m.multi {
			return nil, errors.New("multi get connection must specify a database name")
		}
		if m.mysql == nil {
			return nil, errors.New("mySQL connection error")
		}
		return m.mysql, nil
	}
	if len(dbName) > 1 {
		return nil, errors.New("Multidb can only get one connection")
	}
	if !m.multi {
		if m.mysql == nil {
			return nil, errors.New("mySQL connection error")
		}
		return m.mysql, nil
	}
	conn := m.mysqls[dbName[0]]
	if conn == nil {
		return nil, errors.New(dbName[0] + " mysql connection not found or failed")
	}
	return conn, nil
}

func (m *MysqlClient) IsMultiDB() bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.multi
}

func (m *MysqlClient) ListConnNames() []string {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string(nil), m.conns...)
}
