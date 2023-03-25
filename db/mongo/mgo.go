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
	"io/ioutil"
	"log"
	"os"
	"strings"
)

type Mongodb struct {
	multi   bool
	conns   map[string]connection
	tags    []string
	max     int
	conf    *koanf.Koanf
	confUrl string
}

type connection struct {
	conn *mgo.Session
	db   string
	url  string
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
	m.tags = make([]string, 0)
	if m.conns == nil {
		m.conns = make(map[string]connection)
	}
	if len(m.conns) == 0 {
		if m.conf == nil {
			var confData []byte
			var err error
			if strings.HasPrefix(m.confUrl, "http://") {
				resp, err := grequests.Get(m.confUrl, nil)
				if err != nil {
					logger.Error("MongoDB配置下载失败! " + err.Error())
					return
				}
				confData = []byte(resp.String())
			} else {
				confData, err = ioutil.ReadFile(m.confUrl)
				if err != nil {
					logger.Error(fmt.Sprintf("MongoDB本地配置文件%s读取失败:%s", m.confUrl, err.Error()))
					return
				}
			}
			m.conf = koanf.New(".")
			err = m.conf.Load(rawbytes.Provider(confData), yaml.Parser())
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
			dbNames := strings.Split(m.conf.String("go.data.mongodb.dbNames"), ",")
			for _, dbName := range dbNames {
				if dbName != "" && m.conf.Exists(fmt.Sprintf("go.data.mongodb.%s.uri", dbName)) {
					uri := m.conf.String(fmt.Sprintf("go.data.mongodb.%s.uri", dbName))
					session, err := mgo.Dial(uri)
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
			conn, err := mgo.Dial(m.conf.String("go.data.mongodb.uri"))
			if err != nil {
				logger.Error("MongoDB连接错误:" + err.Error())
				return
			}
			m.conns["0"] = connection{
				conn: conn,
				db:   m.conf.String("go.data.mongodb.db"),
				url:  m.conf.String("go.data.mongodb.uri"),
			}
			if m.conf.Int("go.data.mongo_pool.max") > 1 {
				m.max = m.conf.Int("go.data.mongo_pool.max")
				if m.max < 10 {
					m.max = 10
				}
				m.conns["0"].conn.SetPoolLimit(m.max)
				m.conns["0"].conn.SetMode(mgo.Monotonic, true)
			}
		}
	}
}

func (m *Mongodb) Close() {
	if m.multi {
		for k, _ := range m.conns {
			m.conns[k].conn.Close()
			delete(m.conns, k)
		}
	} else {
		m.conns["0"].conn.Close()
		delete(m.conns, "0")
	}
}

func (m *Mongodb) mgoCheck(tag string) error {
	if len(m.conns) == 0 {
		m.Init("")
	}
	if m.conns[tag].conn.Ping() != nil {
		uri := m.conns[tag].url
		db := m.conns[tag].db
		m.conns[tag].conn.Close()
		session, err := mgo.Dial(uri)
		if err != nil {
			logger.Error(tag + " MongoDB连接错误:" + err.Error())
			return err
		}
		m.conns[tag] = connection{
			conn: session,
			db:   db,
			url:  uri,
		}
		session.SetPoolLimit(m.max)
		session.SetMode(mgo.Monotonic, true)
	}
	return nil
}

func (m *Mongodb) Check() error {
	var err error
	if len(m.conns) == 0 {
		m.Init("")
	}
	if m.multi {
		for dbName, _ := range m.conns {
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
	if m.multi {
		if len(dbName) > 1 || len(dbName) == 0 {
			return nil, errors.New("Multidb Mongodb get connection must be specified one dbName")
		}
		if dbName[0] == "" {
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
		return m.conns["0"].conn.Copy().DB(m.conns["0"].db), nil
	}
}

func (m *Mongodb) ReturnConnection(conn *mgo.Database) {
	conn.Session.Close()
}

func (m *Mongodb) IsMultiDB() bool {
	return m.multi
}

func (m *Mongodb) ListConnNames() []string {
	return m.tags
}
