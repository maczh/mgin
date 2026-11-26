package mongo

import (
	"errors"
	"fmt"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/maczh/mgo"
	"github.com/sadlil/gologger"
	"strings"
	"sync"
	"time"
)

type Mongodb struct {
	mu       sync.RWMutex
	multi    bool
	conns    map[string]connection
	tags     []string
	max      int
	conf     *koanf.Koanf
	confUrl  string
	confData []byte
}

type connection struct {
	conn *mgo.Session
	db   string
	url  string
}

var logger = gologger.GetLogger()

// Init parses MongoDB configuration and dials every configured database
// outside the publish lock. A slow endpoint is bounded to at most 10s per
// database and no longer blocks readers during a reload.
func (m *Mongodb) Init(mongodbConfigData []byte) {
	if m == nil {
		return
	}
	if len(mongodbConfigData) == 0 {
		logger.Error("Mongodb 配置错误，无法获取配置地址")
		return
	}

	m.mu.RLock()
	initialized := len(m.conns) > 0
	m.mu.RUnlock()
	if initialized {
		return
	}

	conf := koanf.New(".")
	if err := conf.Load(rawbytes.Provider(mongodbConfigData), yaml.Parser()); err != nil {
		logger.Error("MongoDB配置解析错误:" + err.Error())
		return
	}

	multi := conf.Bool("go.data.mongodb.multidb")
	max := 10
	if conf.Int("go.data.mongo_pool.max") > 1 {
		max = conf.Int("go.data.mongo_pool.max")
		if max < 10 {
			max = 10
		}
	}
	conns := make(map[string]connection)
	tags := make([]string, 0)
	if multi {
		dbNames := strings.Split(conf.String("go.data.mongodb.dbNames"), ",")
		for _, dbName := range dbNames {
			if dbName == "" || !conf.Exists(fmt.Sprintf("go.data.mongodb.%s.uri", dbName)) {
				continue
			}
			uri := conf.String(fmt.Sprintf("go.data.mongodb.%s.uri", dbName))
			session, err := mgo.DialWithTimeout(uri, 10*time.Second, max)
			if err != nil {
				logger.Error(dbName + " MongoDB连接错误:" + err.Error())
				continue
			}
			conns[dbName] = connection{
				conn: session,
				db:   conf.String(fmt.Sprintf("go.data.mongodb.%s.db", dbName)),
				url:  uri,
			}
			tags = append(tags, dbName)
		}
	} else {
		uri := conf.String("go.data.mongodb.uri")
		conn, err := mgo.DialWithTimeout(uri, 10*time.Second, max)
		if err != nil {
			logger.Error("MongoDB连接错误:" + err.Error())
			return
		}
		conns["0"] = connection{
			conn: conn,
			db:   conf.String("go.data.mongodb.db"),
			url:  uri,
		}
	}

	m.mu.Lock()
	if len(m.conns) > 0 {
		m.mu.Unlock()
		for _, candidate := range conns {
			if candidate.conn != nil {
				candidate.conn.Close()
			}
		}
		return
	}
	m.conf = conf
	m.multi = multi
	m.max = max
	m.conns = conns
	m.tags = tags
	m.confData = append([]byte(nil), mongodbConfigData...)
	m.mu.Unlock()
}

func (m *Mongodb) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	connections := m.conns
	m.conns = make(map[string]connection)
	m.tags = nil
	m.mu.Unlock()
	for _, conn := range connections {
		if conn.conn != nil {
			conn.conn.Close()
		}
	}
}

func (m *Mongodb) mgoCheck(tag string) error {
	if m == nil {
		return errors.New("MongoDB client is nil")
	}
	m.mu.RLock()
	empty, confData := len(m.conns) == 0, append([]byte(nil), m.confData...)
	m.mu.RUnlock()
	if empty {
		m.Init(confData)
	}
	m.mu.RLock()
	current, ok := m.conns[tag]
	m.mu.RUnlock()
	if !ok || current.conn == nil {
		return errors.New("MongoDB connection not found")
	}
	if err := current.conn.Ping(); err != nil {
		m.mu.RLock()
		max := m.max
		m.mu.RUnlock()
		session, err := mgo.DialWithTimeout(current.url, 10*time.Second, max)
		if err != nil {
			logger.Error(tag + " MongoDB连接错误:" + err.Error())
			return err
		}
		if session == nil {
			return errors.New("MongoDB reconnect returned a nil session")
		}
		m.mu.Lock()
		latest, exists := m.conns[tag]
		if !exists || latest.conn != current.conn {
			m.mu.Unlock()
			session.Close()
			if exists && latest.conn != nil {
				return nil
			}
			return errors.New("MongoDB connection changed during reconnect")
		}
		m.conns[tag] = connection{
			conn: session,
			db:   current.db,
			url:  current.url,
		}
		m.mu.Unlock()
		current.conn.Close()
	}
	return nil
}

func (m *Mongodb) Check() error {
	if m == nil {
		return errors.New("MongoDB client is nil")
	}
	var err error
	m.mu.RLock()
	empty, confData, multi := len(m.conns) == 0, append([]byte(nil), m.confData...), m.multi
	m.mu.RUnlock()
	if empty {
		m.Init(confData)
	}
	m.mu.RLock()
	names := make([]string, 0, len(m.conns))
	for name := range m.conns {
		names = append(names, name)
	}
	multi = m.multi
	m.mu.RUnlock()
	if multi {
		for _, dbName := range names {
			err = m.mgoCheck(dbName)
			if err != nil {
				logger.Error(dbName + "连接检查失败:" + err.Error())
				continue
			}
		}
	} else {
		if len(names) == 0 {
			return errors.New("MongoDB connections are unavailable")
		}
		err = m.mgoCheck("0")
	}
	return err
}

func (m *Mongodb) GetConnection(dbName ...string) (*mgo.Database, error) {
	if m == nil {
		return nil, errors.New("MongoDB client is nil")
	}
	m.mu.RLock()
	multi := m.multi
	m.mu.RUnlock()
	if multi {
		if len(dbName) > 1 || len(dbName) == 0 {
			return nil, errors.New("Multidb Mongodb get connection must be specified one dbName")
		}
		m.mu.RLock()
		if dbName[0] == "" {
			if len(m.tags) == 0 {
				m.mu.RUnlock()
				return nil, errors.New("MongoDB multidb has no available database")
			}
			dbName[0] = m.tags[0]
		}
		_, exists := m.conns[dbName[0]]
		m.mu.RUnlock()
		if !exists {
			return nil, errors.New("MongoDB multidb db name invalid")
		}
		err := m.mgoCheck(dbName[0])
		if err != nil {
			return nil, err
		}
		m.mu.RLock()
		conn, ok := m.conns[dbName[0]]
		if !ok || conn.conn == nil {
			m.mu.RUnlock()
			return nil, errors.New("MongoDB connection not found")
		}
		copy := conn.conn.Copy()
		m.mu.RUnlock()
		if copy == nil {
			return nil, errors.New("MongoDB session copy failed")
		}
		return copy.DB(conn.db), nil
	} else {
		if err := m.Check(); err != nil {
			return nil, err
		}
		m.mu.RLock()
		conn, ok := m.conns["0"]
		if !ok || conn.conn == nil {
			m.mu.RUnlock()
			return nil, errors.New("Mongodb connection failed")
		}
		copy := conn.conn.Copy()
		m.mu.RUnlock()
		if copy == nil {
			return nil, errors.New("MongoDB session copy failed")
		}
		return copy.DB(conn.db), nil
	}
}

func (m *Mongodb) ReturnConnection(conn *mgo.Database) {
	if conn == nil {
		return
	}
	conn.Session().Close()
}

func (m *Mongodb) IsMultiDB() bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.multi
}

func (m *Mongodb) ListConnNames() []string {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string(nil), m.tags...)
}
