package redis

import (
	"errors"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/levigross/grequests"
	"github.com/sadlil/gologger"
	"io/ioutil"
	"strings"
	"time"
)

type RedisClient struct {
	multi   bool
	master  string
	clients map[string]redis.UniversalClient
	cfgs    map[string]redis.UniversalOptions
	conf    *koanf.Koanf
	confUrl string
	conns   []string
}

var logger = gologger.GetLogger()

func (r *RedisClient) Init(redisConfigUrl string) {
	if redisConfigUrl != "" {
		r.confUrl = redisConfigUrl
	}
	if r.confUrl == "" {
		logger.Error("Redis配置Url为空")
		return
	}
	if len(r.clients) == 0 {
		if r.conf == nil {
			var confData []byte
			var err error
			if strings.HasPrefix(r.confUrl, "http://") {
				resp, err := grequests.Get(r.confUrl, nil)
				if err != nil {
					logger.Error("Redis配置下载失败! " + err.Error())
					return
				}
				confData = []byte(resp.String())
			} else {
				confData, err = ioutil.ReadFile(r.confUrl)
				if err != nil {
					logger.Error(fmt.Sprintf("Redis本地配置文件%s读取失败:%s", r.confUrl, err.Error()))
					return
				}
			}
			r.conf = koanf.New(".")
			err = r.conf.Load(rawbytes.Provider(confData), yaml.Parser())
			if err != nil {
				logger.Error("Redis配置文件解析错误:" + err.Error())
				r.conf = nil
				return
			}
		}
		r.multi = r.conf.Bool("go.data.redis.multidb")
		if r.multi {
			r.clients = make(map[string]redis.UniversalClient)
			r.cfgs = make(map[string]redis.UniversalOptions)
			r.conns = make([]string, 0)
			dbNames := strings.Split(r.conf.String("go.data.redis.dbNames"), ",")
			for _, dbName := range dbNames {
				var uo redis.UniversalOptions
				if dbName != "" {
					if r.conf.Exists(fmt.Sprintf("go.data.redis.%s.uri", dbName)) {
						uo.Addrs = strings.Split(r.conf.String(fmt.Sprintf("go.data.redis.%s.uri", dbName)), ",")
					} else if r.conf.Exists(fmt.Sprintf("go.data.redis.%s.host", dbName)) {
						hosts := strings.Split(r.conf.String(fmt.Sprintf("go.data.redis.%s.host", dbName)), ",")
						ports := strings.Split(r.conf.String(fmt.Sprintf("go.data.redis.%s.port", dbName)), ",")
						addrs := make([]string, 0)
						for i, addr := range hosts {
							addrs = append(addrs, fmt.Sprintf("%s:%s", addr, ports[i]))
						}
						uo.Addrs = addrs
					} else {
						continue
					}
					if r.conf.String(fmt.Sprintf("go.data.redis.%s.master", dbName)) != "" {
						r.master = r.conf.String(fmt.Sprintf("go.data.redis.%s.master", dbName))
						uo.MasterName = r.master
					}
					uo.Password = r.conf.String(fmt.Sprintf("go.data.redis.%s.password", dbName))
					uo.DB = r.conf.Int(fmt.Sprintf("go.data.redis.%s.database", dbName))
					r.cfgs[dbName] = uo
					r.conns = append(r.conns, dbName)
				}
			}
		}
	} else {
		var uo redis.UniversalOptions
		if r.conf.Exists("go.data.redis.uri") {
			uo.Addrs = strings.Split(r.conf.String("go.data.redis.uri"), ",")
		} else if r.conf.Exists("go.data.redis.host") {
			hosts := strings.Split(r.conf.String("go.data.redis.host"), ",")
			ports := strings.Split(r.conf.String("go.data.redis.port"), ",")
			addrs := make([]string, 0)
			for i, addr := range hosts {
				addrs = append(addrs, fmt.Sprintf("%s:%s", addr, ports[i]))
			}
			uo.Addrs = addrs
		}
		if r.conf.String("go.data.redis.master") != "" {
			r.master = r.conf.String("go.data.redis.master")
			uo.MasterName = r.master
		}
		uo.Password = r.conf.String("go.data.redis.password")
		uo.DB = r.conf.Int("go.data.redis.database")
		r.cfgs["0"] = uo
		r.conns = append(r.conns, "0")
	}
	if r.conf.Int("go.data.redis_pool.max") > 1 {
		min := r.conf.Int("go.data.redis_pool.min")
		if min == 0 {
			min = 2
		}
		max := r.conf.Int("go.data.redis_pool.max")
		if max < 10 {
			max = 10
		}
		idleTimeout := r.conf.Int("go.data.redis_pool.idleTimeout")
		if idleTimeout == 0 {
			idleTimeout = 5
		}
		connectTimeout := r.conf.Int("go.data.redis_pool.timeout")
		if connectTimeout == 0 {
			connectTimeout = 60
		}
		if r.multi {
			for k, rds := range r.cfgs {
				rds.PoolSize = max
				rds.MinIdleConns = min
				rds.IdleTimeout = time.Duration(idleTimeout) * time.Minute
				rds.DialTimeout = time.Duration(connectTimeout) * time.Second
				r.cfgs[k] = rds
			}
		}
	}
	for dbName, rds := range r.cfgs {
		rc := redis.NewUniversalClient(&rds)
		if err := rc.Ping().Err(); err != nil {
			logger.Error(dbName + " Redis连接失败:" + err.Error())
			continue
		}
		fmt.Printf("%s 连接成功\n", dbName)
		r.clients[dbName] = rc
	}
}

func (r *RedisClient) Close() {
	if r.multi {
		for dbName, rc := range r.clients {
			rc.Close()
			delete(r.clients, dbName)
		}
	} else {
		r.clients["0"].Close()
		delete(r.clients, "0")
	}
}

func (r *RedisClient) redisCheck(dbName string) error {
	//fmt.Printf("正在检查%s连接\n", dbName)
	//if err := r.clients[dbName].Ping().Err(); err != nil {
	//	logger.Error("Redis连接故障:" + err.Error())
	//	r.clients[dbName].Close()
	//	ropt := r.cfgs[dbName]
	//	rc := redis.NewClient(ropt)
	//	if err := rc.Ping().Err(); err != nil {
	//		logger.Error(dbName + " Redis连接失败:" + err.Error())
	//		return err
	//	}
	//	r.clients[dbName] = rc
	//}
	return nil
}

func (r *RedisClient) Check() error {
	//var err error
	//if r.client == nil && len(r.clients) == 0 {
	//	r.Init("")
	//}
	//if r.multi {
	//	for dbName, _ := range r.cfgs {
	//		err = r.redisCheck(dbName)
	//		if err != nil {
	//			logger.Error(dbName + " Redis检查失败:" + err.Error())
	//		}
	//	}
	//} else {
	//	if err = r.client.Ping().Err(); err != nil {
	//		logger.Error("Redis连接故障:" + err.Error())
	//		r.Close()
	//		r.Init("")
	//		if err = r.client.Ping().Err(); err != nil {
	//			logger.Error("Redis重新连接之后依然故障:" + err.Error())
	//		} else {
	//			logger.Error("Redis重新连接成功")
	//		}
	//	}
	//}
	return nil
}

func (r *RedisClient) GetConnection(dbName ...string) (redis.UniversalClient, error) {
	if r.multi {
		if len(dbName) == 0 || len(dbName) > 1 {
			return nil, errors.New("Multidb Get RedisClient connection must specify one database name")
		}
		if _, ok := r.clients[dbName[0]]; !ok {
			return nil, errors.New("Redis multidb db name invalid")
		}
		err := r.redisCheck(dbName[0])
		if err != nil {
			return nil, err
		}
		return r.clients[dbName[0]], nil
	} else {
		err := r.Check()
		if err != nil {
			return nil, errors.New("redis connection failed")
		}
		return r.clients["0"], nil
	}
}

func (r *RedisClient) IsMultiDB() bool {
	return r.multi
}

func (r *RedisClient) ListConnNames() []string {
	return r.conns
}
