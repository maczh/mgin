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

type MongoDB struct {
	conn       *mgo.Database
	mongo      *mgo.Session
	mgodb      string
	multi      bool
	mongos     map[string]*mgo.Session
	mgoDbNames map[string]string
	mongoUrls  map[string]string
	max        int
	conf       *koanf.Koanf
	confUrl    string
}

var logger = gologger.GetLogger()

func (m *MongoDB) Init(mongodbConfigUrl string) {
	if mongodbConfigUrl != "" {
		m.confUrl = mongodbConfigUrl
	}
	if m.confUrl == "" {
		logger.Error("MongoDB配置Url为空")
		return
	}
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
		if m.conf.Bool("go.data.MongoDB.debug") {
			mgo.SetDebug(true)
			var mgoLogger *log.Logger
			mgoLogger = log.New(os.Stderr, "", log.LstdFlags)
			mgo.SetLogger(mgoLogger)
		}
		m.multi = m.conf.Bool("go.data.MongoDB.multidb")
		if m.multi {
			m.mongos = make(map[string]*mgo.Session)
			m.mgoDbNames = make(map[string]string)
			m.mongoUrls = make(map[string]string)
			dbNames := strings.Split(m.conf.String("go.data.MongoDB.dbNames"), ",")
			for _, dbName := range dbNames {
				if dbName != "" && m.conf.Exists(fmt.Sprintf("go.data.MongoDB.%s.uri", dbName)) {
					m.mongoUrls[dbName] = m.conf.String(fmt.Sprintf("go.data.MongoDB.%s.uri", dbName))
					session, err := mgo.Dial(m.mongoUrls[dbName])
					if err != nil {
						logger.Error(dbName + " MongoDB连接错误:" + err.Error())
						continue
					}
					m.mongos[dbName] = session
					m.mgoDbNames[dbName] = m.conf.String(fmt.Sprintf("go.data.MongoDB.%s.db", dbName))
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
			m.mongo, err = mgo.Dial(m.conf.String("go.data.MongoDB.uri"))
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
			m.mgodb = m.conf.String("go.data.MongoDB.db")
			m.conn = m.mongo.Copy().DB(m.mgodb)
		}
	}
}

func (m *MongoDB) Close() {
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

func (m *MongoDB) mgoCheck(dbName string) error {
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

func (m *MongoDB) Check() {
	if (m.conn == nil || m.mongo == nil) && len(m.mongos) == 0 {
		m.Init("")
		return
	}
	if m.multi {
		for dbName, _ := range m.mongos {
			err := m.mgoCheck(dbName)
			if err != nil {
				continue
			}
		}
	} else {
		if m.mongo.Ping() != nil {
			m.Close()
			m.Init("")
		}
	}
}

func (m *MongoDB) GetConnection(dbName ...string) (*mgo.Database, error) {
	if m.multi {
		if len(dbName) > 1 || len(dbName) == 0 {
			return nil, errors.New("Multidb MongoDB get connection must be specified one dbName")
		}
		err := m.mgoCheck(dbName[0])
		if err != nil {
			return nil, err
		}
		return m.mongos[dbName[0]].Copy().DB(m.mgoDbNames[dbName[0]]), nil
	} else {
		m.Check()
		if m.mongo == nil {
			return nil, errors.New("MongoDB connection failed")
		}
		return m.mongo.Copy().DB(m.mgodb), nil
	}
}

func (m *MongoDB) ReturnConnection(conn *mgo.Database) {
	conn.Session.Close()
}
