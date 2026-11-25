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
	"github.com/maczh/mgin/v2/pkg/logs"
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
	openConn func(string) (*gorm.DB, error)
}

var logger = logs.GetLogger()

var ErrClosed = errors.New("mysql client is closed")

// Init 保留旧 MginPlugin 签名；新调用方应使用 InitWithError 获取初始化错误。
func (m *MysqlClient) Init(mysqlConfigData []byte) {
	_ = m.InitWithError(mysqlConfigData)
}

type mysqlConnections struct {
	conf     *koanf.Koanf
	confData []byte
	multi    bool
	mysql    *gorm.DB
	mysqls   map[string]*gorm.DB
	conns    []string
}

func (m *MysqlClient) InitWithError(mysqlConfigData []byte) error {
	return m.initialize(mysqlConfigData, false)
}

func (m *MysqlClient) initialize(mysqlConfigData []byte, onlyIfNeeded bool) error {
	if m == nil {
		return ErrClosed
	}
	m.initMu.Lock()
	defer m.initMu.Unlock()

	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return ErrClosed
	}
	if onlyIfNeeded && ((!m.multi && m.mysql != nil) || (m.multi && len(m.mysqls) > 0)) {
		m.mu.RUnlock()
		return nil
	}
	if len(mysqlConfigData) == 0 {
		mysqlConfigData = append([]byte(nil), m.confData...)
	}
	opener := m.openConn
	m.mu.RUnlock()

	if opener == nil {
		opener = openMySQL
	}
	next, err := loadMySQLConnections(mysqlConfigData, opener)
	if err != nil {
		return err
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		next.close()
		return ErrClosed
	}
	oldSingle, oldMulti := m.mysql, m.mysqls
	m.conf = next.conf
	m.confData = next.confData
	m.multi = next.multi
	m.mysql = next.mysql
	m.mysqls = next.mysqls
	m.conns = next.conns
	m.mu.Unlock()

	closeMySQL(oldSingle)
	closeMySQLMap(oldMulti)
	return nil
}

func loadMySQLConnections(mysqlConfigData []byte, opener func(string) (*gorm.DB, error)) (*mysqlConnections, error) {
	if len(mysqlConfigData) == 0 {
		return nil, errors.New("mysql configuration is empty")
	}

	conf := koanf.New(".")
	if err := conf.Load(rawbytes.Provider(mysqlConfigData), yaml.Parser()); err != nil {
		logger.Error("MySQL配置格式解析错误:" + err.Error())
		return nil, err
	}
	next := &mysqlConnections{
		conf:     conf,
		confData: append([]byte(nil), mysqlConfigData...),
		multi:    conf.Bool("go.data.mysql.multidb"),
	}
	if next.multi {
		next.mysqls = make(map[string]*gorm.DB)
		seen := make(map[string]struct{})
		for _, dbName := range strings.Split(conf.String("go.data.mysql.dbNames"), ",") {
			dbName = strings.TrimSpace(dbName)
			if dbName == "" {
				continue
			}
			if _, ok := seen[dbName]; ok {
				continue
			}
			seen[dbName] = struct{}{}
			dsn := conf.String("go.data.mysql." + dbName)
			if dsn == "" {
				next.close()
				return nil, fmt.Errorf("%s mysql connection is empty", dbName)
			}
			conn, err := opener(dsn)
			if err != nil {
				closeMySQL(conn)
				next.close()
				return nil, fmt.Errorf("%s mysql connection error: %w", dbName, err)
			}
			if err := validateMySQL(conn); err != nil {
				closeMySQL(conn)
				next.close()
				return nil, fmt.Errorf("%s mysql connection error: %w", dbName, err)
			}
			next.mysqls[dbName] = conn
			next.conns = append(next.conns, dbName)
		}
		if len(next.mysqls) == 0 {
			next.close()
			return nil, errors.New("mySQL connection error")
		}
	} else {
		conn, err := opener(conf.String("go.data.mysql"))
		if err != nil {
			closeMySQL(conn)
			return nil, err
		}
		if err := validateMySQL(conn); err != nil {
			closeMySQL(conn)
			return nil, err
		}
		next.mysql = conn
	}

	if conf.Bool("go.data.mysql_debug") {
		if next.multi {
			for name, conn := range next.mysqls {
				next.mysqls[name] = conn.Debug()
			}
		} else {
			next.mysql = next.mysql.Debug()
		}
	}
	if conf.Int("go.data.mysql_pool.max") > 1 {
		if next.multi {
			for _, conn := range next.mysqls {
				if err := configurePool(conn, conf); err != nil {
					next.close()
					return nil, err
				}
			}
		} else if err := configurePool(next.mysql, conf); err != nil {
			next.close()
			return nil, err
		}
	}
	return next, nil
}

func (c *mysqlConnections) close() {
	if c == nil {
		return
	}
	closeMySQL(c.mysql)
	closeMySQLMap(c.mysqls)
}

func openMySQL(dsn string) (*gorm.DB, error) {
	conn, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logs.NewGORMLogger()})
	if err != nil {
		logger.Error("mysql connection error:" + err.Error())
		closeMySQL(conn)
		return nil, err
	}
	if err := validateMySQL(conn); err != nil {
		closeMySQL(conn)
		return nil, err
	}
	sqldb, err := conn.DB()
	if err != nil {
		closeMySQL(conn)
		return nil, err
	}
	if err := sqldb.Ping(); err != nil {
		closeMySQL(conn)
		return nil, err
	}
	return conn, nil
}

func validateMySQL(conn *gorm.DB) error {
	if conn == nil || conn.Config == nil || conn.Statement == nil {
		return errors.New("mysql connection is not initialized")
	}
	if conn.Error != nil {
		return conn.Error
	}
	return nil
}

func configurePool(conn *gorm.DB, conf *koanf.Koanf) error {
	sqldb, err := conn.DB()
	if err != nil {
		return err
	}
	max := conf.Int("go.data.mysql_pool.max")
	if max < 10 {
		max = 10
	}
	idle := conf.Int("go.data.mysql_pool.total")
	if idle == 0 || idle < max {
		idle = 5 * max
	}
	idleTimeout := conf.Int("go.data.mysql_pool.timeout")
	if idleTimeout == 0 {
		idleTimeout = 60
	}
	lifetime := conf.Int("go.data.mysql_pool.life")
	if lifetime == 0 {
		lifetime = 60
	}
	sqldb.SetConnMaxIdleTime(time.Duration(idleTimeout) * time.Second)
	sqldb.SetMaxIdleConns(idle)
	sqldb.SetMaxOpenConns(max)
	sqldb.SetConnMaxLifetime(time.Duration(lifetime) * time.Minute)
	return nil
}

func closeMySQL(conn *gorm.DB) {
	if conn == nil || conn.Config == nil {
		return
	}
	if sqldb, err := conn.DB(); err == nil && sqldb != nil {
		_ = sqldb.Close()
	}
}

func closeMySQLMap(conns map[string]*gorm.DB) {
	for _, conn := range conns {
		closeMySQL(conn)
	}
}

func (m *MysqlClient) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	single, multi := m.mysql, m.mysqls
	m.mysql = nil
	m.mysqls = nil
	m.conns = nil
	m.conf = nil
	m.confData = nil
	m.mu.Unlock()

	closeMySQL(single)
	closeMySQLMap(multi)
}

func mySqlCheck(m *MysqlClient) (*gorm.DB, error) {
	if m == nil {
		return nil, ErrClosed
	}
	m.mu.RLock()
	closed, multi, conn := m.closed, m.multi, m.mysql
	m.mu.RUnlock()
	if closed {
		return nil, ErrClosed
	}
	if multi {
		return nil, errors.New("multi get connection must specify a database name")
	}
	if conn == nil {
		if err := m.initialize(nil, true); err != nil {
			return nil, err
		}
		m.mu.RLock()
		closed, multi, conn = m.closed, m.multi, m.mysql
		m.mu.RUnlock()
		if closed {
			return nil, ErrClosed
		}
		if multi {
			return nil, errors.New("multi get connection must specify a database name")
		}
	}
	if conn == nil {
		return nil, errors.New("mysql connection is not initialized")
	}

	conn, err := pingMySQL(conn)
	m.mu.RLock()
	closed = m.closed
	m.mu.RUnlock()
	if closed {
		return nil, ErrClosed
	}
	return conn, err
}

func pingMySQL(conn *gorm.DB) (*gorm.DB, error) {
	if err := validateMySQL(conn); err != nil {
		return nil, err
	}
	sqldb, err := conn.DB()
	if err != nil {
		return nil, err
	}
	if err := sqldb.Ping(); err != nil {
		return nil, err
	}
	return conn, nil
}

func mySqlsCheck(m *MysqlClient) error {
	if m == nil {
		return ErrClosed
	}
	m.mu.RLock()
	closed, multi := m.closed, m.multi
	needsInit := multi && len(m.mysqls) == 0
	if closed || !multi {
		m.mu.RUnlock()
		if closed {
			return ErrClosed
		}
		return errors.New("Not multi mysql connections setting")
	}
	if needsInit {
		m.mu.RUnlock()
		if err := m.initialize(nil, true); err != nil {
			return err
		}
		m.mu.RLock()
		closed, multi = m.closed, m.multi
		if closed || !multi {
			m.mu.RUnlock()
			if closed {
				return ErrClosed
			}
			return errors.New("Not multi mysql connections setting")
		}
	}
	conns := make(map[string]*gorm.DB, len(m.mysqls))
	for name, conn := range m.mysqls {
		conns[name] = conn
	}
	m.mu.RUnlock()

	var errs []error
	for name, conn := range conns {
		if _, err := pingMySQL(conn); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
		}
	}
	m.mu.RLock()
	closed = m.closed
	m.mu.RUnlock()
	if closed {
		return ErrClosed
	}
	return errors.Join(errs...)
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
	} else {
		_, err = mySqlCheck(m)
	}
	if err != nil && !errors.Is(err, ErrClosed) {
		logger.Error(err.Error())
	}
	return err
}

func (m *MysqlClient) GetConnection(dbName ...string) (*gorm.DB, error) {
	if m == nil {
		return nil, ErrClosed
	}
	if len(dbName) > 1 {
		return nil, errors.New("Multidb can only get one connection")
	}
	m.mu.RLock()
	closed, multi := m.closed, m.multi
	m.mu.RUnlock()
	if closed {
		return nil, ErrClosed
	}
	if len(dbName) == 0 || !multi {
		return mySqlCheck(m)
	}

	m.mu.RLock()
	conn := m.mysqls[dbName[0]]
	closed, multi = m.closed, m.multi
	m.mu.RUnlock()
	if closed {
		return nil, ErrClosed
	}
	if !multi {
		return mySqlCheck(m)
	}
	if conn == nil {
		return nil, errors.New(dbName[0] + " mysql connection not found or failed")
	}
	if err := validateMySQL(conn); err != nil {
		return nil, err
	}
	m.mu.RLock()
	closed = m.closed
	m.mu.RUnlock()
	if closed {
		return nil, ErrClosed
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
