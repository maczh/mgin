package mongo

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/maczh/mgo"
	"github.com/sadlil/gologger"
)

type Mongodb struct {
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

func (m *Mongodb) Init(mongodbConfigData []byte) {
	if mongodbConfigData != nil {
		m.confData = mongodbConfigData
	}
	//if m.confUrl == "" {
	//	logger.Error("MongoDB配置Url为空")
	//	return
	//}
	m.tags = make([]string, 0)
	if m.conns == nil {
		m.conns = make(map[string]connection)
	}
	if len(m.conns) == 0 {
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
					m.max = 10
					if m.conf.Int("go.data.mongo_pool.max") > 1 {
						m.max = m.conf.Int("go.data.mongo_pool.max")
						if m.max < 10 {
							m.max = 10
						}
					}
					session, err := mgo.DialWithTimeout(uri, 10*time.Second, m.max)
					if err != nil {
						logger.Error(dbName + " MongoDB连接错误:" + err.Error())
						continue
					}
					m.conns[dbName] = connection{
						conn: session,
						db:   m.conf.String(fmt.Sprintf("go.data.mongodb.%s.db", dbName)),
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
			conn, err := mgo.DialWithTimeout(m.conf.String("go.data.mongodb.uri"), 10*time.Second, m.max)
			if err != nil {
				logger.Error("MongoDB连接错误:" + err.Error())
				return
			}
			m.conns["0"] = connection{
				conn: conn,
				db:   m.conf.String("go.data.mongodb.db"),
				url:  m.conf.String("go.data.mongodb.uri"),
			}
		}
	}
}

func (m *Mongodb) Close() {
	if m == nil || m.conns == nil {
		return
	}
	if m.multi {
		for k, conn := range m.conns {
			if conn.conn != nil {
				conn.conn.Close()
			}
			delete(m.conns, k)
		}
	} else {
		if conn, ok := m.conns["0"]; ok && conn.conn != nil {
			conn.conn.Close()
		}
		delete(m.conns, "0")
	}
}

func (m *Mongodb) mgoCheck(tag string) error {
	if m == nil {
		return errors.New("Mongodb is nil")
	}
	if len(m.conns) == 0 {
		m.Init(m.confData)
	}
	conn, ok := m.conns[tag]
	if !ok || conn.conn == nil {
		return errors.New("MongoDB connection not found")
	}
	if conn.conn.Ping() != nil {
		uri := conn.url
		db := conn.db
		conn.conn.Close()
		session, err := mgo.DialWithTimeout(uri, 10*time.Second, m.max)
		if err != nil {
			logger.Error(tag + " MongoDB连接错误:" + err.Error())
			return err
		}
		m.conns[tag] = connection{
			conn: session,
			db:   db,
			url:  uri,
		}
	}
	return nil
}

func (m *Mongodb) Check() error {
	if m == nil {
		return errors.New("Mongodb is nil")
	}
	var err error
	if len(m.conns) == 0 {
		m.Init(m.confData)
	}
	if m.multi {
		for dbName := range m.conns {
			err = m.mgoCheck(dbName)
			if err != nil {
				logger.Error(dbName + "连接检查失败:" + err.Error())
				continue
			}
		}
	} else {
		err = m.mgoCheck("0")
	}
	return err
}

func (m *Mongodb) GetConnection(dbName ...string) (*mgo.Database, error) {
	if m == nil {
		return nil, errors.New("Mongodb is nil")
	}
	if m.multi {
		if len(dbName) > 1 || len(dbName) == 0 {
			return nil, errors.New("Multidb Mongodb get connection must be specified one dbName")
		}
		if dbName[0] == "" {
			if len(m.tags) == 0 {
				return nil, errors.New("MongoDB multidb has no available database")
			}
			dbName[0] = m.tags[0]
		}
		if _, ok := m.conns[dbName[0]]; !ok {
			return nil, errors.New("MongoDB multidb db name invalid")
		}
		err := m.mgoCheck(dbName[0])
		if err != nil {
			return nil, err
		}
		return m.conns[dbName[0]].conn.Copy().DB(m.conns[dbName[0]].db), nil
	} else {
		m.Check()
		if len(m.conns) == 0 {
			return nil, errors.New("Mongodb connection failed")
		}
		conn, ok := m.conns["0"]
		if !ok || conn.conn == nil {
			return nil, errors.New("Mongodb connection failed")
		}
		return conn.conn.Copy().DB(conn.db), nil
	}
}

func (m *Mongodb) ReturnConnection(conn *mgo.Database) {
	if conn == nil {
		return
	}
	conn.Session().Close()
}

func (m *Mongodb) IsMultiDB() bool {
	return m.multi
}

func (m *Mongodb) ListConnNames() []string {
	return m.tags
}
