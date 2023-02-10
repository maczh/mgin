package mongo

import (
	"errors"
	"fmt"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/levigross/grequests"
	"github.com/sadlil/gologger"
	"gopkg.in/mgo.v2"
	"log"
	"os"
	"strings"
)

type Mongodb struct {
	conn       *mgo.Database
	mongo      *mgo.Session
	mgodb      string
	multi      bool
	mongos     map[string]*mgo.Session
	mgoDbNames map[string]string
	mongoUrls  map[string]string
	conns      []string
	max        int
	conf       *koanf.Koanf
	confUrl    string
}

var logger = gologger.GetLogger()

func (m *Mongodb) Init(mongodbConfigUrl string) {
	if mongodbConfigUrl != "" {
		m.confUrl = mongodbConfigUrl
	}
	if m.confUrl == "" {
		logger.Error("MongoDB配置Url为空")
		return
	}
	m.conns = make([]string, 0)
	var err error
	if m.conn == nil && len(m.mongos) == 0 {
		if m.conf == nil {
			resp, err := grequests.Get(m.confUrl, nil)
			if err != nil {
				logger.Error("MongoDB配置下载失败! " + err.Error())
				return
			}
			m.conf = koanf.New(".")
			err = m.conf.Load(rawbytes.Provider([]byte(resp.String())), yaml.Parser())
			if err != nil {
				logger.Error("MongoDB配置解析错误:" + err.Error())
				m.conf = nil
				return
			}
		}
		if m.conf.Bool("go.data.mongodb.debug") {
			mgo.SetDebug(true)
			var mgoLogger *log.Logger
			mgoLogger = log.New(os.Stderr, "", log.LstdFlags)
			mgo.SetLogger(mgoLogger)
		}
		m.multi = m.conf.Bool("go.data.mongodb.multidb")
		if m.multi {
			m.mongos = make(map[string]*mgo.Session)
			m.mgoDbNames = make(map[string]string)
			m.mongoUrls = make(map[string]string)
			dbNames := strings.Split(m.conf.String("go.data.mongodb.dbNames"), ",")
			for _, dbName := range dbNames {
				if dbName != "" && m.conf.Exists(fmt.Sprintf("go.data.Mongodb.%s.uri", dbName)) {
					m.mongoUrls[dbName] = m.conf.String(fmt.Sprintf("go.data.Mongodb.%s.uri", dbName))
					session, err := mgo.Dial(m.mongoUrls[dbName])
					if err != nil {
						logger.Error(dbName + " MongoDB连接错误:" + err.Error())
						continue
					}
					m.mongos[dbName] = session
					m.conns = append(m.conns, dbName)
					m.mgoDbNames[dbName] = m.conf.String(fmt.Sprintf("go.data.Mongodb.%s.db", dbName))
					if m.conf.Int("go.data.mongo_pool.max") > 1 {
						m.max = m.conf.Int("go.data.mongo_pool.max")
						if m.max < 10 {
							m.max = 10
						}
						session.SetPoolLimit(m.max)
						session.SetMode(mgo.Monotonic, true)
					}
				}
			}
		} else {
			m.mongo, err = mgo.Dial(m.conf.String("go.data.mongodb.uri"))
			if err != nil {
				logger.Error("MongoDB连接错误:" + err.Error())
				return
			}
			if m.conf.Int("go.data.mongo_pool.max") > 1 {
				m.max = m.conf.Int("go.data.mongo_pool.max")
				if m.max < 10 {
					m.max = 10
				}
				m.mongo.SetPoolLimit(m.max)
				m.mongo.SetMode(mgo.Monotonic, true)
			}
			m.mgodb = m.conf.String("go.data.mongodb.db")
			m.conn = m.mongo.Copy().DB(m.mgodb)
		}
	}
}

func (m *Mongodb) Close() {
	if m.multi {
		for k, _ := range m.mongos {
			m.mongos[k].Close()
			delete(m.mongos, k)
		}
	} else {
		m.mongo.Close()
		m.conn = nil
		m.mongo = nil
	}
}

func (m *Mongodb) mgoCheck(dbName string) error {
	if m.mongos[dbName].Ping() != nil {
		m.mongos[dbName].Close()
		session, err := mgo.Dial(m.mongoUrls[dbName])
		if err != nil {
			logger.Error(dbName + " MongoDB连接错误:" + err.Error())
			return err
		}
		m.mongos[dbName] = session
		session.SetPoolLimit(m.max)
		session.SetMode(mgo.Monotonic, true)
	}
	return nil
}

func (m *Mongodb) Check() error {
	var err error
	if (m.conn == nil || m.mongo == nil) && len(m.mongos) == 0 {
		m.Init("")
	}
	if m.multi {
		for dbName, _ := range m.mongos {
			err := m.mgoCheck(dbName)
			if err != nil {
				logger.Error(dbName + "连接检查失败:" + err.Error())
				continue
			}
		}
	} else {
		if err = m.mongo.Ping(); err != nil {
			logger.Error("MongoDB连接ping失败:" + err.Error())
			m.Close()
			m.Init("")
			if err = m.mongo.Ping(); err != nil {
				logger.Error("MongoDB重新连接之后依然ping失败:" + err.Error())
			} else {
				logger.Error("MongoDB重新连接之后ping成功")
			}
		}
	}
	return err
}

func (m *Mongodb) GetConnection(dbName ...string) (*mgo.Database, error) {
	if m.multi {
		if len(dbName) > 1 || len(dbName) == 0 {
			return nil, errors.New("Multidb Mongodb get connection must be specified one dbName")
		}
		if dbName[0] == "" {
			dbName[0] = m.conns[0]
		}
		err := m.mgoCheck(dbName[0])
		if err != nil {
			return nil, err
		}
		return m.mongos[dbName[0]].Copy().DB(m.mgoDbNames[dbName[0]]), nil
	} else {
		m.Check()
		if m.mongo == nil {
			return nil, errors.New("Mongodb connection failed")
		}
		return m.mongo.Copy().DB(m.mgodb), nil
	}
}

func (m *Mongodb) ReturnConnection(conn *mgo.Database) {
	conn.Session.Close()
}

func (m *Mongodb) IsMultiDB() bool {
	return m.multi
}

func (m *Mongodb) ListConnNames() []string {
	return m.conns
}
