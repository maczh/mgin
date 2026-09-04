package mysql

import (
	"errors"
	"strings"
	"time"

	caches "github.com/go-gorm/caches/v4"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/maczh/mgin/pkg/config"
	"github.com/maczh/mgin/pkg/db/redis"
	"github.com/sadlil/gologger"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type MysqlClient struct {
	mysql    *gorm.DB
	mysqls   map[string]*gorm.DB
	multi    bool
	conf     *koanf.Koanf
	confUrl  string
	confData []byte
	conns    []string
	cache    bool
}

var logger = gologger.GetLogger()

func (m *MysqlClient) Init(mysqlConfigData []byte) {
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
			return
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
					conn, err := gorm.Open(mysql.Open(m.conf.String("go.data.mysql."+dbName)), &gorm.Config{})
					if err != nil {
						logger.Error(dbName + " mysql connection error:" + err.Error())
						continue
					}
					m.mysqls[dbName] = conn
					m.conns = append(m.conns, dbName)
				}
			}
		} else {
			m.mysql, err = gorm.Open(mysql.Open(m.conf.String("go.data.mysql")), &gorm.Config{})
			if err != nil {
				logger.Error("mysql connection error:" + err.Error())
				return
			}
		}
		if m.mysql == nil {
			logger.Error("mysql connection error")
			return
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

func (m *MysqlClient) Close() {
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

func mySqlsCheck(m *MysqlClient) error {
	if !m.multi {
		return errors.New("Not multi mysql connections setting")
	}
	if len(m.mysqls) == 0 {
		m.Init(m.confData)
		if len(m.mysqls) == 0 {
			return errors.New("mySQL connection error")
		}
	}
	for k, _ := range m.mysqls {
		sqldb, _ := m.mysqls[k].DB()
		err := sqldb.Ping()
		if err != nil {
			m.Close()
			m.Init(m.confData)
			if len(m.mysqls) == 0 {
				return errors.New("mySQL connection error")
			}
		}
	}
	return nil
}

func mySqlCheck(m *MysqlClient) (*gorm.DB, error) {
	if m.mysql == nil {
		m.Init(m.confData)
		if m.mysql == nil {
			return nil, errors.New("mySQL connection error")
		}
	}
	sqldb, _ := m.mysql.DB()
	err := sqldb.Ping()
	if err != nil {
		m.Close()
		m.Init(m.confData)
		if m.mysql == nil {
			return nil, errors.New("mySQL connection error")
		}
	}
	return m.mysql, nil
}

func (m *MysqlClient) Check() error {
	var err error
	if m.multi {
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

func (m *MysqlClient) IsMultiDB() bool {
	return m.multi
}

func (m *MysqlClient) ListConnNames() []string {
	return m.conns
}

func (m *MysqlClient) UseCache() bool {
	if !m.conf.Bool("go.data.mysql_cache") {
		return false
	}
	if !strings.Contains(config.Config.Config.Used, "redis") {
		logger.Error("MySQL cache use memory cache")
		cachesPlugin := &caches.Caches{
			Conf: &caches.Config{
				Easer: true,
			},
		}
		if m.multi {
			for k, _ := range m.mysqls {
				m.mysqls[k].Use(cachesPlugin)
			}
		} else {
			m.mysql.Use(cachesPlugin)
		}
		return true
	}
	rds, err := redis.Redis.GetConnection()
	if err != nil {
		logger.Error("MySQL init Redis Cache connection error: " + err.Error())
		return false
	}
	exp := 5 * time.Minute
	if m.conf.Duration("go.data.mysql_cache_expired") > 0 {
		exp = m.conf.Duration("go.data.mysql_cache_expired") * time.Second
	}
	cachesPlugin := &caches.Caches{
		Conf: &caches.Config{
			Cacher: &RedisCacher{
				Rdb:        rds,
				Expiration: exp,
			},
		},
	}
	if m.multi {
		for k, _ := range m.mysqls {
			m.mysqls[k].Use(cachesPlugin)
		}
	} else {
		m.mysql.Use(cachesPlugin)
	}
	return true
}
