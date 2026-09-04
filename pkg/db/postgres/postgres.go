package postgres

import (
	"errors"
	"strings"
	"time"

	caches "github.com/go-gorm/caches/v4"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/maczh/mgin/pkg/config"
	"github.com/maczh/mgin/pkg/db/mysql"
	"github.com/maczh/mgin/pkg/db/redis"
	"github.com/sadlil/gologger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PostgresClient struct {
	postgres   *gorm.DB
	postgreses map[string]*gorm.DB
	multi      bool
	conf       *koanf.Koanf
	confUrl    string
	confData   []byte
	conns      []string
	cache      bool
}

var logger = gologger.GetLogger()

func (p *PostgresClient) Init(postgresConfigData []byte) {
	if postgresConfigData != nil && len(postgresConfigData) > 0 {
		p.confData = postgresConfigData
		p.conf = koanf.New(".")
		err := p.conf.Load(rawbytes.Provider(postgresConfigData), yaml.Parser())
		if err != nil {
			logger.Error("PostgreSQL配置格式解析错误:" + err.Error())
			p.conf = nil
			return
		}
		p.multi = false
		if p.conf.Exists("go.data.postgres.multidb") && p.conf.Bool("go.data.postgres.multidb") {
			p.multi = true
			p.postgreses = make(map[string]*gorm.DB)
			p.conns = make([]string, 0)
			dbNames := strings.Split(p.conf.String("go.data.postgres.dbNames"), ",")
			for _, dbName := range dbNames {
				if dbName != "" && p.conf.String("go.data.postgres."+dbName+".dsn") != "" {
					conn, err := gorm.Open(postgres.Open(p.conf.String("go.data.postgres."+dbName+".dsn")), &gorm.Config{})
					if err != nil {
						logger.Error(dbName + " postgres connection error:" + err.Error())
						continue
					}
					p.postgreses[dbName] = conn
					p.conns = append(p.conns, dbName)
				}
			}
		} else {
			p.postgres, err = gorm.Open(postgres.Open(p.conf.String("go.data.postgres.dsn")), &gorm.Config{})
			if err != nil {
				logger.Error("postgres connection error:" + err.Error())
				return
			}
		}
		if p.postgres == nil && !p.multi {
			logger.Error("postgres connection error")
			return
		}
		if p.conf.Bool("go.data.postgres.debug") {
			if p.multi {
				for k, _ := range p.postgreses {
					p.postgreses[k] = p.postgreses[k].Debug()
				}
			} else {
				p.postgres = p.postgres.Debug()
			}
		}
		if p.conf.Int("go.data.postgres.pool.max") > 1 {
			max := p.conf.Int("go.data.postgres.pool.max")
			if max < 10 {
				max = 10
			}
			idle := p.conf.Int("go.data.postgres.pool.total")
			if idle == 0 || idle < max {
				idle = 5 * max
			}
			idleTimeout := p.conf.Int("go.data.postgres.pool.timeout")
			if idleTimeout == 0 {
				idleTimeout = 60
			}
			lifetime := p.conf.Int("go.data.postgres.pool.life")
			if lifetime == 0 {
				lifetime = 60
			}
			if !p.multi {
				sqldb, _ := p.postgres.DB()
				sqldb.SetConnMaxIdleTime(time.Duration(idleTimeout) * time.Second)
				sqldb.SetMaxIdleConns(idle)
				sqldb.SetMaxOpenConns(max)
				sqldb.SetConnMaxLifetime(time.Duration(lifetime) * time.Minute)
			} else {
				for k, _ := range p.postgreses {
					sqldb, _ := p.postgreses[k].DB()
					sqldb.SetConnMaxIdleTime(time.Duration(idleTimeout) * time.Second)
					sqldb.SetMaxIdleConns(idle)
					sqldb.SetMaxOpenConns(max)
					sqldb.SetConnMaxLifetime(time.Duration(lifetime) * time.Minute)
				}
			}
		}
	}
}

func (p *PostgresClient) Close() {
	if p.multi {
		for k, _ := range p.postgreses {
			sqldb, _ := p.postgreses[k].DB()
			sqldb.Close()
			delete(p.postgreses, k)
		}
	} else {
		sqldb, _ := p.postgres.DB()
		sqldb.Close()
		p.postgres = nil
	}
}

func postgresesCheck(p *PostgresClient) error {
	if !p.multi {
		return errors.New("Not multi postgres connections setting")
	}
	if len(p.postgreses) == 0 {
		p.Init(p.confData)
		if len(p.postgreses) == 0 {
			return errors.New("postgres connection error")
		}
	}
	for k, _ := range p.postgreses {
		sqldb, _ := p.postgreses[k].DB()
		err := sqldb.Ping()
		if err != nil {
			p.Close()
			p.Init(p.confData)
			if len(p.postgreses) == 0 {
				return errors.New("postgres connection error")
			}
		}
	}
	return nil
}

func postgresCheck(p *PostgresClient) (*gorm.DB, error) {
	if p.postgres == nil {
		p.Init(p.confData)
		if p.postgres == nil {
			return nil, errors.New("postgres connection error")
		}
	}
	sqldb, _ := p.postgres.DB()
	err := sqldb.Ping()
	if err != nil {
		p.Close()
		p.Init(p.confData)
		if p.postgres == nil {
			return nil, errors.New("postgres connection error")
		}
	}
	return p.postgres, nil
}

func (p *PostgresClient) Check() error {
	var err error
	if p.multi {
		err = postgresesCheck(p)
		if err != nil {
			logger.Error(err.Error())
		}
	} else {
		_, err = postgresCheck(p)
		if err != nil {
			logger.Error(err.Error())
		}
	}
	return err
}

func (p *PostgresClient) GetConnection(dbName ...string) (*gorm.DB, error) {
	if len(dbName) == 0 {
		if p.multi {
			return nil, errors.New("multi get connection must specify a database name")
		}
		return postgresCheck(p)
	}
	if len(dbName) > 1 {
		return nil, errors.New("Multidb can only get one connection")
	}
	if !p.multi {
		return postgresCheck(p)
	}
	conn := p.postgreses[dbName[0]]
	if conn == nil {
		return nil, errors.New(dbName[0] + " postgres connection not found or failed")
	}
	return conn, nil
}

func (p *PostgresClient) IsMultiDB() bool {
	return p.multi
}

func (p *PostgresClient) ListConnNames() []string {
	return p.conns
}

func (p *PostgresClient) UseCache() bool {
	if !p.conf.Bool("go.data.postgres.cache.enabled") {
		return false
	}
	if !strings.Contains(config.Config.Config.Used, "redis") {
		logger.Error("PostgreSQL cache use memory cache")
		cachesPlugin := &caches.Caches{
			Conf: &caches.Config{
				Easer: true,
			},
		}
		if p.multi {
			for k, _ := range p.postgreses {
				p.postgreses[k].Use(cachesPlugin)
			}
		} else {
			p.postgres.Use(cachesPlugin)
		}
		return true
	}
	rds, err := redis.Redis.GetConnection()
	if err != nil {
		logger.Error("PostgreSQL init Redis Cache connection error: " + err.Error())
		return false
	}
	exp := 5 * time.Minute
	if p.conf.Duration("go.data.postgres.cache.expired") > 0 {
		exp = p.conf.Duration("go.data.postgres.cache.expired") * time.Second
	}
	cachesPlugin := &caches.Caches{
		Conf: &caches.Config{
			Cacher: &mysql.RedisCacher{
				Rdb:        rds,
				Expiration: exp,
			},
		},
	}
	if p.multi {
		for k, _ := range p.postgreses {
			p.postgreses[k].Use(cachesPlugin)
		}
	} else {
		p.postgres.Use(cachesPlugin)
	}
	return true
}
