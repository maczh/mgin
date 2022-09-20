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
	"net"
	"strings"
	"time"
)

type Redis struct {
	client  *redis.Client
	multi   bool
	clients map[string]*redis.Client
	cfgs    map[string]*redis.Options
	conf    *koanf.Koanf
	confUrl string
}

var logger = gologger.GetLogger()

func (r *Redis) Init(redisConfigUrl string) {
	if redisConfigUrl != "" {
		r.confUrl = redisConfigUrl
	}
	if r.confUrl == "" {
		logger.Error("Redis配置Url为空")
		return
	}
	if r.conf == nil {
		resp, err := grequests.Get(r.confUrl, nil)
		if err != nil {
			logger.Error("Redis配置下载失败! " + err.Error())
			return
		}
		r.conf = koanf.New(".")
		err = r.conf.Load(rawbytes.Provider([]byte(resp.String())), yaml.Parser())
		if err != nil {
			logger.Error("Redis配置文件解析错误:" + err.Error())
			r.conf = nil
			return
		}
		r.multi = r.conf.Bool("go.data.redis.multidb")
		var ro redis.Options
		if r.multi {
			r.clients = make(map[string]*redis.Client)
			r.cfgs = make(map[string]*redis.Options)
			dbNames := strings.Split(r.conf.String("go.data.redis.dbNames"), ",")
			for _, dbName := range dbNames {
				if dbName != "" && r.conf.Exists(fmt.Sprintf("go.data.redis.%s.host", dbName)) {
					ropt := redis.Options{
						Addr:     r.conf.String(fmt.Sprintf("go.data.redis.%s.host", dbName)) + ":" + r.conf.String(fmt.Sprintf("go.data.redis.%s.port", dbName)),
						Password: r.conf.String(fmt.Sprintf("go.data.redis.%s.password", dbName)),
						DB:       r.conf.Int(fmt.Sprintf("go.data.redis.%s.database", dbName)),
						Dialer: func() (net.Conn, error) {
							netDialer := &net.Dialer{
								Timeout:   5 * time.Second,
								KeepAlive: 5 * time.Minute,
							}
							return netDialer.Dial("tcp", r.conf.String(fmt.Sprintf("go.data.redis.%s.host", dbName))+":"+r.conf.String(fmt.Sprintf("go.data.redis.%s.port", dbName)))
						},
					}
					r.cfgs[dbName] = &ropt
				}
			}
		} else {
			ro = redis.Options{
				Addr:     r.conf.String("go.data.redis.host") + ":" + r.conf.String("go.data.redis.port"),
				Password: r.conf.String("go.data.redis.password"),
				DB:       r.conf.Int("go.data.redis.database"),
				Dialer: func() (net.Conn, error) {
					netDialer := &net.Dialer{
						Timeout:   5 * time.Second,
						KeepAlive: 5 * time.Minute,
					}
					return netDialer.Dial("tcp", r.conf.String("go.data.redis.host")+":"+r.conf.String("go.data.redis.port"))
				},
			}
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
			} else {
				ro.PoolSize = max
				ro.MinIdleConns = min
				ro.IdleTimeout = time.Duration(idleTimeout) * time.Minute
				ro.DialTimeout = time.Duration(connectTimeout) * time.Second
			}
		}
		if r.multi {
			for dbName, rds := range r.cfgs {
				rc := redis.NewClient(rds)
				if err := rc.Ping().Err(); err != nil {
					logger.Error(dbName + " Redis连接失败:" + err.Error())
					continue
				}
				fmt.Printf("%s 连接成功\n", dbName)
				r.clients[dbName] = rc
			}
		} else {
			r.client = redis.NewClient(&ro)
			if err := r.client.Ping().Err(); err != nil {
				logger.Error("Redis连接失败:" + err.Error())
			}
		}
	}
}

func (r *Redis) Close() {
	if r.multi {
		for dbName, rc := range r.clients {
			rc.Close()
			delete(r.clients, dbName)
		}
	} else {
		r.client.Close()
		r.client = nil
	}
}

func (r *Redis) redisCheck(dbName string) error {
	fmt.Printf("正在检查%s连接\n", dbName)
	if err := r.clients[dbName].Ping().Err(); err != nil {
		logger.Error("Redis连接故障:" + err.Error())
		ropt := r.cfgs[dbName]
		rc := redis.NewClient(ropt)
		if err := rc.Ping().Err(); err != nil {
			logger.Error(dbName + " Redis连接失败:" + err.Error())
			return err
		}
		r.clients[dbName] = rc
	}
	return nil
}

func (r *Redis) Check() {
	if r.client == nil && len(r.clients) == 0 {
		r.Init("")
		return
	}
	if r.multi {
		for dbName, _ := range r.cfgs {
			_ = r.redisCheck(dbName)
		}
	} else {
		if err := r.client.Ping().Err(); err != nil {
			logger.Error("Redis连接故障:" + err.Error())
			r.Close()
			r.Init("")
		}
	}
}

func (r *Redis) GetConnection(dbName ...string) (*redis.Client, error) {
	if r.multi {
		if len(dbName) == 0 || len(dbName) > 1 {
			return nil, errors.New("Multidb Get Redis connection must specify one database name")
		}
		err := r.redisCheck(dbName[0])
		if err != nil {
			return nil, err
		}
		return r.clients[dbName[0]], nil
	} else {
		r.Check()
		if r.client == nil {
			return nil, errors.New("redis connection failed")
		}
		return r.client, nil
	}
}
