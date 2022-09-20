package mysql

import (
	"errors"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/levigross/grequests"
	"github.com/sadlil/gologger"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"strings"
	"time"
)

type mysqlClient struct {
	mysql   *gorm.DB
	mysqls  map[string]*gorm.DB
	multi   bool
	conf    *koanf.Koanf
	confUrl string
}

var Mysql = &mysqlClient{}
var logger = gologger.GetLogger()

func (m *mysqlClient) Init(mysqlConfigUrl string) {
	if mysqlConfigUrl != "" {
		m.confUrl = mysqlConfigUrl
	}
	if m.confUrl == "" {
		logger.Error("MySQL配置文件Url为空")
		return
	}
	if m.conf == nil {
		resp, err := grequests.Get(m.confUrl, nil)
		if err != nil {
			logger.Error("MySQL配置下载失败! " + err.Error())
			return
		}
		m.conf = koanf.New(".")
		err = m.conf.Load(rawbytes.Provider([]byte(resp.String())), yaml.Parser())
		if err != nil {
			logger.Error("MySQL配置格式解析错误:" + err.Error())
			m.conf = nil
			return
		}
		m.multi = false
		if m.conf.Exists("go.data.mysql.multi") && m.conf.Bool("go.data.mysql.multi") {
			m.multi = true
			m.mysqls = make(map[string]*gorm.DB)
			dbNames := strings.Split(m.conf.String("go.data.mysql.dbNames"), ",")
			for _, dbName := range dbNames {
				if dbName != "" && m.conf.String("go.data.mysql."+dbName) != "" {
					conn, err := gorm.Open(mysql.Open(m.conf.String("go.data.mysql."+dbName)), &gorm.Config{})
					if err != nil {
						logger.Error(dbName + " mysql connection error:" + err.Error())
						continue
					}
					m.mysqls[dbName] = conn
				}
			}
		} else {
			m.mysql, _ = gorm.Open(mysql.Open(m.conf.String("go.data.mysql")), &gorm.Config{})
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
				sqldb, _ := m.mysql.DB()
				sqldb.SetConnMaxIdleTime(time.Duration(idleTimeout) * time.Second)
				sqldb.SetMaxIdleConns(idle)
				sqldb.SetMaxOpenConns(max)
				sqldb.SetConnMaxLifetime(time.Duration(lifetime) * time.Minute)
			} else {
				for k, _ := range m.mysqls {
					sqldb, _ := m.mysqls[k].DB()
					sqldb.SetConnMaxIdleTime(time.Duration(idleTimeout) * time.Second)
					sqldb.SetMaxIdleConns(idle)
					sqldb.SetMaxOpenConns(max)
					sqldb.SetConnMaxLifetime(time.Duration(lifetime) * time.Minute)
				}
			}
		}
	}
}

func (m *mysqlClient) Close() {
	if m.multi {
		for k, _ := range m.mysqls {
			sqldb, _ := m.mysqls[k].DB()
			sqldb.Close()
			delete(m.mysqls, k)
		}
	} else {
		sqldb, _ := m.mysql.DB()
		sqldb.Close()
		m.mysql = nil
	}
}

func mySqlsCheck(m *mysqlClient) error {
	if !m.multi {
		return errors.New("Not multi mysql connections setting")
	}
	if len(m.mysqls) == 0 {
		m.Init("")
		if len(m.mysqls) == 0 {
			return errors.New("mySQL connection error")
		}
	}
	for k, _ := range m.mysqls {
		sqldb, _ := m.mysqls[k].DB()
		err := sqldb.Ping()
		if err != nil {
			m.Close()
			m.Init("")
			if len(m.mysqls) == 0 {
				return errors.New("mySQL connection error")
			}
		}
	}
	return nil
}

func mySqlCheck(m *mysqlClient) (*gorm.DB, error) {
	if m.mysql == nil {
		m.Init("")
		if m.mysql == nil {
			return nil, errors.New("mySQL connection error")
		}
	}
	sqldb, _ := m.mysql.DB()
	err := sqldb.Ping()
	if err != nil {
		m.Close()
		m.Init("")
		if m.mysql == nil {
			return nil, errors.New("mySQL connection error")
		}
	}
	return m.mysql, nil
}

func (m *mysqlClient) Check() {
	if m.multi {
		err := mySqlsCheck(m)
		if err != nil {
			logger.Error(err.Error())
		}
	} else {
		_, err := mySqlCheck(m)
		if err != nil {
			logger.Error(err.Error())
		}
	}
}

func (m *mysqlClient) GetConnection(dbName ...string) (*gorm.DB, error) {
	if len(dbName) == 0 {
		if m.multi {
			return nil, errors.New("multi get connection must specify a database name")
		}
		return mySqlCheck(m)
	}
	if len(dbName) > 1 {
		return nil, errors.New("Multidb can only get one connection")
	}
	if !m.multi {
		return mySqlCheck(m)
	}
	conn := m.mysqls[dbName[0]]
	if conn == nil {
		return nil, errors.New(dbName[0] + " mysql connection not found or failed")
	}
	return conn, nil
}
