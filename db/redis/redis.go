package redis

import (
	"context"
	"errors"
	"fmt"
	redis "github.com/go-redis/redis/v7"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/maczh/mgin/internal/safego"
	"github.com/maczh/mgin/logs"
	"github.com/sadlil/gologger"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type RedisClient struct {
	mu            sync.RWMutex
	multi         bool
	master        string
	clients       map[string]redis.UniversalClient
	cfgs          map[string]redis.UniversalOptions
	conf          *koanf.Koanf
	confUrl       string
	confData      []byte
	conns         []string
	subMu         sync.Mutex
	subscriptions map[uint64]context.CancelFunc
	subWG         sync.WaitGroup
	nextSub       atomic.Uint64
	subClosed     bool
}

var Redis = &RedisClient{}

var logger = gologger.GetLogger()

// Init parses the Redis configuration and builds all clients outside the
// publish lock. A slow or unreachable Redis therefore cannot block readers
// (GetConnection/Check/IsMultiDB) while the reload is in flight.
func (r *RedisClient) Init(redisConfigData []byte) {
	if r == nil {
		return
	}
	if len(redisConfigData) == 0 {
		logger.Error("Redis 配置错误，无法获取配置地址")
		return
	}

	conf := koanf.New(".")
	if err := conf.Load(rawbytes.Provider(redisConfigData), yaml.Parser()); err != nil {
		logger.Error("Redis配置文件解析错误:" + err.Error())
		return
	}

	multi := conf.Bool("go.data.redis.multidb")
	master := ""
	cfgs := make(map[string]redis.UniversalOptions)
	conns := make([]string, 0)
	if multi {
		dbNames := strings.Split(conf.String("go.data.redis.dbNames"), ",")
		for _, dbName := range dbNames {
			var uo redis.UniversalOptions
			if dbName == "" {
				continue
			}
			if conf.Exists(fmt.Sprintf("go.data.redis.%s.uri", dbName)) {
				uo.Addrs = strings.Split(conf.String(fmt.Sprintf("go.data.redis.%s.uri", dbName)), ",")
			} else if conf.Exists(fmt.Sprintf("go.data.redis.%s.host", dbName)) {
				hosts := strings.Split(conf.String(fmt.Sprintf("go.data.redis.%s.host", dbName)), ",")
				ports := strings.Split(conf.String(fmt.Sprintf("go.data.redis.%s.port", dbName)), ",")
				addrs := make([]string, 0, len(hosts))
				for i, addr := range hosts {
					if i >= len(ports) {
						break
					}
					addrs = append(addrs, fmt.Sprintf("%s:%s", addr, ports[i]))
				}
				uo.Addrs = addrs
			} else {
				continue
			}
			if conf.String(fmt.Sprintf("go.data.redis.%s.master", dbName)) != "" {
				master = conf.String(fmt.Sprintf("go.data.redis.%s.master", dbName))
				uo.MasterName = master
			}
			uo.Password = conf.String(fmt.Sprintf("go.data.redis.%s.password", dbName))
			uo.DB = conf.Int(fmt.Sprintf("go.data.redis.%s.database", dbName))
			if uo.DB == -1 || len(uo.Addrs) > 1 {
				uo.DB = 0
			}
			cfgs[dbName] = uo
			conns = append(conns, dbName)
		}
	} else {
		var uo redis.UniversalOptions
		if conf.Exists("go.data.redis.uri") {
			uo.Addrs = strings.Split(conf.String("go.data.redis.uri"), ",")
		} else if conf.Exists("go.data.redis.host") {
			hosts := strings.Split(conf.String("go.data.redis.host"), ",")
			ports := strings.Split(conf.String("go.data.redis.port"), ",")
			addrs := make([]string, 0, len(hosts))
			for i, addr := range hosts {
				if i >= len(ports) {
					break
				}
				addrs = append(addrs, fmt.Sprintf("%s:%s", addr, ports[i]))
			}
			uo.Addrs = addrs
		}
		if conf.String("go.data.redis.master") != "" {
			master = conf.String("go.data.redis.master")
			uo.MasterName = master
		}
		uo.Password = conf.String("go.data.redis.password")
		uo.DB = conf.Int("go.data.redis.database")
		if uo.DB == -1 || len(uo.Addrs) > 1 {
			uo.DB = 0
		}
		cfgs["0"] = uo
		conns = append(conns, "0")
	}
	if conf.Int("go.data.redis_pool.max") > 1 {
		min := conf.Int("go.data.redis_pool.min")
		if min == 0 {
			min = 2
		}
		max := conf.Int("go.data.redis_pool.max")
		if max < 10 {
			max = 10
		}
		idleTimeout := conf.Int("go.data.redis_pool.idleTimeout")
		if idleTimeout == 0 {
			idleTimeout = 5
		}
		connectTimeout := conf.Int("go.data.redis_pool.timeout")
		if connectTimeout == 0 {
			connectTimeout = 60
		}
		for k, rds := range cfgs {
			rds.PoolSize = max
			rds.MinIdleConns = min
			rds.IdleTimeout = time.Duration(idleTimeout) * time.Minute
			rds.DialTimeout = time.Duration(connectTimeout) * time.Second
			cfgs[k] = rds
		}
	}

	clients := make(map[string]redis.UniversalClient, len(cfgs))
	for dbName, rds := range cfgs {
		opts := rds
		rc := redis.NewUniversalClient(&opts)
		if err := pingRedis(rc); err != nil {
			logger.Error(dbName + " Redis连接失败:" + err.Error())
			if err.Error() != "ERR SELECT is not allowed in cluster mode" {
				_ = rc.Close()
				continue
			}
		}
		fmt.Printf("%s 连接成功\n", dbName)
		clients[dbName] = rc
	}

	r.mu.Lock()
	if len(r.clients) > 0 {
		r.mu.Unlock()
		for _, rc := range clients {
			_ = rc.Close()
		}
		return
	}
	r.conf = conf
	r.multi = multi
	r.master = master
	r.cfgs = cfgs
	r.conns = conns
	r.clients = clients
	r.confData = append([]byte(nil), redisConfigData...)
	r.mu.Unlock()

	r.subMu.Lock()
	r.subClosed = false
	r.subMu.Unlock()
}

// pingRedis bounds candidate connection checks independently of DialTimeout so
// a black-holed endpoint cannot stretch a reload indefinitely.
func pingRedis(rc redis.UniversalClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return rc.ProcessContext(ctx, redis.NewStatusCmd("ping"))
}

func (r *RedisClient) Close() {
	if r == nil {
		return
	}
	r.subMu.Lock()
	r.subClosed = true
	for _, cancel := range r.subscriptions {
		cancel()
	}
	r.subMu.Unlock()
	done := make(chan struct{})
	safego.Run(func() { r.subWG.Wait(); close(done) })
	select {
	case <-done:
	case <-time.After(time.Second):
		logger.Error("等待Redis订阅退出超时")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.multi {
		for dbName, rc := range r.clients {
			if rc != nil {
				_ = rc.Close()
			}
			delete(r.clients, dbName)
		}
	} else {
		if client, ok := r.clients["0"]; ok {
			_ = client.Close()
			delete(r.clients, "0")
		}
	}
}

func (r *RedisClient) redisCheck(dbName string) error {
	//fmt.Printf("正在检查%s连接\n", dbName)
	//if err := r.clients[dbName].Ping().Err(); err != nil {
	//	logger.Error("Redis连接故障:" + err.Error())
	//	r.clients[dbName].Close()
	//	ropt := r.cfgs[dbName]
	//	rc := redis.NewUniversalClient(&ropt)
	//	if err := rc.Ping().Err(); err != nil {
	//		logger.Error(dbName + " Redis连接失败:" + err.Error())
	//		return err
	//	}
	//	r.clients[dbName] = rc
	//}
	return nil
}

func (r *RedisClient) Check() error {
	if r == nil {
		return errors.New("Redis client is nil")
	}
	r.mu.RLock()
	multi := r.multi
	clients := make(map[string]redis.UniversalClient, len(r.clients))
	for name, client := range r.clients {
		clients[name] = client
	}
	r.mu.RUnlock()
	if len(clients) == 0 {
		return errors.New("redis connection is not initialized")
	}
	if multi {
		for name, client := range clients {
			if client == nil {
				return fmt.Errorf("redis %s connection is nil", name)
			}
			if err := client.Ping().Err(); err != nil {
				return fmt.Errorf("redis %s ping failed: %w", name, err)
			}
		}
		return nil
	}
	client := clients["0"]
	if client == nil {
		return errors.New("redis connection is not initialized")
	}
	if err := client.Ping().Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}
	return nil
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
}

func (r *RedisClient) GetConnection(dbName ...string) (redis.UniversalClient, error) {
	if r == nil {
		return nil, errors.New("Redis client is nil")
	}
	r.mu.RLock()
	multi := r.multi
	var client redis.UniversalClient
	var exists bool
	if multi {
		if len(dbName) == 1 {
			client, exists = r.clients[dbName[0]]
		}
	} else {
		client, exists = r.clients["0"]
	}
	r.mu.RUnlock()
	if multi {
		if len(dbName) == 0 || len(dbName) > 1 {
			return nil, errors.New("Multidb Get RedisClient connection must specify one database name")
		}
		if !exists {
			return nil, errors.New("Redis multidb db name invalid")
		}
		err := r.redisCheck(dbName[0])
		if err != nil {
			return nil, err
		}
		if client != nil {
			return client, nil
		} else {
			return nil, errors.New("Redis connection failed.")
		}
	} else {
		//err := r.Check()
		//if err != nil {
		//	return nil, errors.New("redis connection failed")
		//}
		if exists && client != nil {
			return client, nil
		} else {
			return nil, errors.New("Redis connection failed.")
		}
	}
}

func (r *RedisClient) IsMultiDB() bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.multi
}

func (r *RedisClient) ListConnNames() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]string(nil), r.conns...)
}

// PSubscribe 订阅消息并自动重连
func (r *RedisClient) PSubscribe(dbName string, handler func(msg *redis.Message, dbName string), channels ...string) {
	r.PSubscribeContext(context.Background(), dbName, handler, channels...)
}

// PSubscribeContext starts a reconnecting subscription that stops when ctx is canceled or Close is called.
func (r *RedisClient) PSubscribeContext(ctx context.Context, dbName string, handler func(msg *redis.Message, dbName string), channels ...string) {
	if r == nil || handler == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	id := r.nextSub.Add(1)
	r.subMu.Lock()
	if r.subClosed {
		r.subMu.Unlock()
		cancel()
		return
	}
	if r.subscriptions == nil {
		r.subscriptions = make(map[uint64]context.CancelFunc)
	}
	r.subscriptions[id] = cancel
	r.subWG.Add(1)
	r.subMu.Unlock()
	safego.Run(func() {
		defer func() {
			cancel()
			r.subMu.Lock()
			delete(r.subscriptions, id)
			r.subMu.Unlock()
			r.subWG.Done()
		}()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			conn, err := r.GetConnection(dbName)
			if err != nil {
				logger.Error("Redis connection failed: " + err.Error())
				timer := time.NewTimer(time.Second)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
				continue
			}
			dbNum := 0
			if cli, ok := conn.(*redis.Client); ok {
				dbNum = cli.Options().DB
			}
			localChannels := make([]string, len(channels))
			for i, channel := range channels {
				localChannels[i] = strings.ReplaceAll(channel, "{db}", fmt.Sprintf("%d", dbNum))
			}
			pubsub := conn.PSubscribe(localChannels...)
			_, err = pubsub.ReceiveTimeout(time.Second)
			if err != nil {
				logs.Error("订阅错误:" + err.Error())
				_ = pubsub.Close()
				if ctx.Err() != nil {
					return
				}
				continue
			}
			ch := pubsub.Channel()
			reconnect := false
			for !reconnect {
				select {
				case <-ctx.Done():
					reconnect = true
				case msg, ok := <-ch:
					if !ok {
						reconnect = true
						continue
					}
					if msg == nil {
						continue
					}
					logs.Debug("收到Redis消息:{}", msg.Payload)
					func() {
						defer func() {
							if recovered := recover(); recovered != nil {
								logs.Error("Redis订阅回调异常:{}", recovered)
							}
						}()
						handler(msg, dbName)
					}()
				}
			}
			logger.Error("Redis订阅异常退出")
			_ = pubsub.Close()
			if ctx.Err() != nil {
				return
			}
		}
	})
}
