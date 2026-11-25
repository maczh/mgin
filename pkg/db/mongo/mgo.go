package mongo

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/maczh/mgo"
	"github.com/sadlil/gologger"
)

type Mongodb struct {
	mu       sync.RWMutex
	initMu   sync.Mutex
	multi    bool
	conns    map[string]connection
	tags     []string
	max      int
	conf     *koanf.Koanf
	confUrl  string
	confData []byte
	closed   bool
	initErr  error
}

type connection struct {
	conn *mgo.Session
	db   string
	url  string
}

var logger = gologger.GetLogger()
var ErrClosed = errors.New("mongodb client is closed")

func (m *Mongodb) Init(mongodbConfigData []byte) {
	_ = m.InitWithError(mongodbConfigData)
}

func (m *Mongodb) InitWithError(mongodbConfigData []byte) error {
	if m == nil {
		return errors.New("Mongodb is nil")
	}
	m.initMu.Lock()
	defer m.initMu.Unlock()
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return ErrClosed
	}
	useExisting := mongodbConfigData == nil
	if useExisting {
		mongodbConfigData = append([]byte(nil), m.confData...)
	}
	ready := useExisting && len(m.conns) > 0
	m.mu.RUnlock()
	if ready {
		return nil
	}
	builder := &Mongodb{}
	builder.build(mongodbConfigData)
	if builder.initErr != nil && len(builder.conns) == 0 {
		closeMongoConnections(builder.conns)
		m.mu.Lock()
		if m.closed {
			m.mu.Unlock()
			return ErrClosed
		}
		m.initErr = builder.initErr
		m.mu.Unlock()
		return builder.initErr
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		closeMongoConnections(builder.conns)
		return ErrClosed
	}
	old := m.conns
	m.multi, m.conns, m.tags, m.max = builder.multi, builder.conns, builder.tags, builder.max
	m.conf, m.confData, m.initErr = builder.conf, append([]byte(nil), mongodbConfigData...), builder.initErr
	m.mu.Unlock()
	closeMongoConnections(old)
	return builder.initErr
}

func (m *Mongodb) build(mongodbConfigData []byte) {
	m.initErr = nil
	if mongodbConfigData != nil {
		m.confData = mongodbConfigData
	}
	//if m.confUrl == "" {
	//	logger.Error("MongoDB配置Url为空")
	//	return
	//}
	if m.conns == nil {
		m.conns = make(map[string]connection)
	}
	if len(m.conns) == 0 {
		m.tags = make([]string, 0)
		if m.conf == nil {
			//var confData []byte
			var err error
			//if strings.HasPrefix(m.confUrl, "http://") {
			//	resp, err := grequests.Get(m.confUrl, nil)
			//	if err != nil {
			//		logger.Error("MongoDB配置下载失败! " + err.Error())
			//		return
			//	}
			//	confData = []byte(resp.String())
			//} else {
			//	confData, err = ioutil.ReadFile(m.confUrl)
			//	if err != nil {
			//		logger.Error(fmt.Sprintf("MongoDB本地配置文件%s读取失败:%s", m.confUrl, err.Error()))
			//		return
			//	}
			//}
			m.conf = koanf.New(".")
			err = m.conf.Load(rawbytes.Provider(m.confData), yaml.Parser())
			if err != nil {
				m.initErr = err
				logger.Error("MongoDB配置解析错误:" + err.Error())
				m.conf = nil
				return
			}
		}
		m.multi = m.conf.Bool("go.data.mongodb.multidb")
		if m.multi {
			dbNames := strings.Split(m.conf.String("go.data.mongodb.dbNames"), ",")
			for _, dbName := range dbNames {
				if dbName != "" && m.conf.Exists(fmt.Sprintf("go.data.mongodb.%s.uri", dbName)) {
					uri := m.conf.String(fmt.Sprintf("go.data.mongodb.%s.uri", dbName))
					database := m.conf.String(fmt.Sprintf("go.data.mongodb.%s.db", dbName))
					uri, err := mongoURIWithDatabase(uri, database)
					if err != nil {
						m.initErr = err
						continue
					}
					m.max = 10
					if m.conf.Int("go.data.mongo_pool.max") > 1 {
						m.max = m.conf.Int("go.data.mongo_pool.max")
						if m.max < 10 {
							m.max = 10
						}
					}
					session, err := mgo.DialWithTimeout(uri, 10*time.Second, m.max)
					if err != nil {
						if m.initErr == nil {
							m.initErr = err
						}
						logger.Error(dbName + " MongoDB连接错误:" + err.Error())
						continue
					}
					m.conns[dbName] = connection{
						conn: session,
						db:   database,
						url:  uri,
					}
					m.tags = append(m.tags, dbName)
				}
			}
		} else {
			m.max = 10
			if m.conf.Int("go.data.mongo_pool.max") > 1 {
				m.max = m.conf.Int("go.data.mongo_pool.max")
				if m.max < 10 {
					m.max = 10
				}
			}
			uri, err := mongoURIWithDatabase(m.conf.String("go.data.mongodb.uri"), m.conf.String("go.data.mongodb.db"))
			if err != nil {
				m.initErr = err
				return
			}
			conn, err := mgo.DialWithTimeout(uri, 10*time.Second, m.max)
			if err != nil {
				m.initErr = err
				logger.Error("MongoDB连接错误:" + err.Error())
				return
			}
			m.conns["0"] = connection{
				conn: conn,
				db:   m.conf.String("go.data.mongodb.db"),
				url:  uri,
			}
		}
	}
	if len(m.conns) == 0 && m.initErr == nil {
		m.initErr = errors.New("MongoDB has no available connections")
	}
}

func mongoURIWithDatabase(uri, database string) (string, error) {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("invalid MongoDB URI")
	}
	if parsed.Path == "" || parsed.Path == "/" {
		if database == "" {
			return "", errors.New("MongoDB database name is required")
		}
		parsed.Path = "/" + database
	}
	return parsed.String(), nil
}

func closeMongoConnections(conns map[string]connection) {
	for _, conn := range conns {
		if conn.conn != nil {
			conn.conn.Close()
		}
	}
}

func (m *Mongodb) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.closed = true
	m.confData = nil
	conns := m.conns
	m.conns = nil
	m.mu.Unlock()
	closeMongoConnections(conns)
}

func (m *Mongodb) Check() error {
	if m == nil {
		return errors.New("Mongodb is nil")
	}
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return ErrClosed
	}
	needsInit := len(m.conns) == 0 && len(m.confData) > 0
	confData := append([]byte(nil), m.confData...)
	m.mu.RUnlock()
	if needsInit {
		m.Init(confData)
	}
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return ErrClosed
	}
	connections := make([]string, 0, len(m.conns))
	for name := range m.conns {
		connections = append(connections, name)
	}
	m.mu.RUnlock()
	if len(connections) == 0 {
		return errors.New("MongoDB connections are unavailable")
	}
	var firstErr error
	for _, name := range connections {
		if err := m.checkAndReconnect(name); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", name, err)
		}
	}
	return firstErr
}

// checkAndReconnect keeps the historical Mongo health-check behavior while
// keeping Ping and Dial outside the client state lock.
func (m *Mongodb) checkAndReconnect(tag string) error {
	return m.checkAndReconnectWith(tag,
		func(session *mgo.Session) error { return session.Ping() },
		mgo.DialWithTimeout,
		func(session *mgo.Session) { session.Close() },
	)
}

func (m *Mongodb) checkAndReconnectWith(
	tag string,
	ping func(*mgo.Session) error,
	dial func(string, time.Duration, int) (*mgo.Session, error),
	closeSession func(*mgo.Session),
) error {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return ErrClosed
	}
	current, ok := m.conns[tag]
	max := m.max
	m.mu.RUnlock()
	if !ok || current.conn == nil {
		return errors.New("MongoDB connection not found")
	}
	if err := ping(current.conn); err == nil {
		return nil
	} else {
		logger.Error(tag + " MongoDB Ping failed, reconnecting: " + err.Error())
	}

	replacement, err := dial(current.url, 10*time.Second, max)
	if err != nil {
		return err
	}
	if replacement == nil {
		return errors.New("MongoDB reconnect returned a nil session")
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		closeSession(replacement)
		return ErrClosed
	}
	latest, exists := m.conns[tag]
	if !exists || latest.conn != current.conn {
		m.mu.Unlock()
		closeSession(replacement)
		if exists && latest.conn != nil {
			return nil
		}
		return errors.New("MongoDB connection changed during reconnect")
	}
	m.conns[tag] = connection{conn: replacement, db: current.db, url: current.url}
	m.mu.Unlock()
	closeSession(current.conn)
	return nil
}

func (m *Mongodb) GetConnection(dbName ...string) (*mgo.Database, error) {
	if m == nil {
		return nil, errors.New("Mongodb is nil")
	}
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return nil, ErrClosed
	}
	needsInit := len(m.conns) == 0 && len(m.confData) > 0
	confData := append([]byte(nil), m.confData...)
	m.mu.RUnlock()
	if needsInit {
		m.Init(confData)
	}
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return nil, ErrClosed
	}
	multi := m.multi
	tag := "0"
	if multi {
		if len(dbName) > 1 || len(dbName) == 0 {
			m.mu.RUnlock()
			return nil, errors.New("Multidb Mongodb get connection must be specified one dbName")
		}
		tag = dbName[0]
		if tag == "" {
			if len(m.tags) == 0 {
				m.mu.RUnlock()
				return nil, errors.New("MongoDB multidb has no available database")
			}
			tag = m.tags[0]
		}
	}
	conn, ok := m.conns[tag]
	m.mu.RUnlock()
	if !ok || conn.conn == nil {
		return nil, errors.New("MongoDB connection not found")
	}
	if err := m.checkAndReconnect(tag); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, ErrClosed
	}
	conn, ok = m.conns[tag]
	if !ok || conn.conn == nil {
		return nil, errors.New("MongoDB connection not found")
	}
	copy := conn.conn.Copy()
	if copy == nil {
		return nil, errors.New("MongoDB session copy failed")
	}
	return copy.DB(conn.db), nil
}

func (m *Mongodb) ReturnConnection(conn *mgo.Database) {
	if conn == nil {
		return
	}
	if session := conn.Session(); session != nil {
		session.Close()
	}
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
