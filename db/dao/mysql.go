package dao

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	cacheStore "github.com/maczh/mgin/cache"
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/models"
	"github.com/sadlil/gologger"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type MySQLDao[E schema.Tabler] struct {
	debug bool
	ctx   *context.Context
	Tag   func() string
	cache *CacheConfig
}

type QueryOption struct {
	Preloads []string
	OrderBy  []string
}

// CacheConfig controls result caching for the DAO query methods.
type CacheConfig struct {
	Enabled bool
	Store   cacheStore.ICache
	TTL     time.Duration
	Prefix  string
}

var logger = gologger.GetLogger()

func (m *MySQLDao[E]) Debug() *MySQLDao[E] {
	return &MySQLDao[E]{
		debug: true,
		Tag:   m.Tag,
		cache: m.cache,
	}
}

func (m *MySQLDao[E]) WithContext(ctx *context.Context) *MySQLDao[E] {
	m.ctx = ctx
	return m
}

// WithCache enables configurable result caching for this DAO.
func (m *MySQLDao[E]) WithCache(config CacheConfig) *MySQLDao[E] {
	if config.Prefix == "" {
		config.Prefix = "mgin:mysql:"
	}
	if config.TTL < 0 {
		config.TTL = 0
	}
	m.cache = &config
	return m
}

func (m *MySQLDao[E]) cacheEnabled() bool {
	return m.cache != nil && m.cache.Enabled && m.cache.Store != nil
}

func (m *MySQLDao[E]) cachePrefix() string {
	if m.cache == nil || m.cache.Prefix == "" {
		return "mgin:mysql:"
	}
	return m.cache.Prefix
}

func (m *MySQLDao[E]) cacheTablePrefix() string {
	var entity E
	tag := ""
	if m.Tag != nil {
		tag = m.Tag()
	}
	return fmt.Sprintf("%s%s:%s:", m.cachePrefix(), tag, entity.TableName())
}

func (m *MySQLDao[E]) cacheKey(operation string, value interface{}) (string, bool) {
	query, err := cacheQueryString(value)
	if err != nil {
		return "", false
	}
	hash := md5.Sum([]byte(query))
	return m.cacheTablePrefix() + operation + ":" + hex.EncodeToString(hash[:]), true
}

func cacheQueryString(value interface{}) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	var decoded interface{}
	if err = json.Unmarshal(data, &decoded); err != nil {
		return "", err
	}
	fields, ok := decoded.(map[string]interface{})
	if !ok {
		return "value=" + string(data), nil
	}
	keys := make([]string, 0, len(fields))
	for key, fieldValue := range fields {
		if !isEmptyCacheValue(fieldValue) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		fieldData, err := json.Marshal(fields[key])
		if err != nil {
			return "", err
		}
		if text, ok := fields[key].(string); ok {
			parts = append(parts, key+"="+text)
		} else {
			parts = append(parts, key+"="+string(fieldData))
		}
	}
	return strings.Join(parts, "&"), nil
}

func isEmptyCacheValue(value interface{}) bool {
	switch value := value.(type) {
	case nil:
		return true
	case string:
		return value == ""
	case []interface{}:
		return len(value) == 0
	case map[string]interface{}:
		return len(value) == 0
	default:
		return false
	}
}

func (m *MySQLDao[E]) loadCached(key string, target interface{}) bool {
	if !m.cacheEnabled() {
		return false
	}
	value, ok := m.cache.Store.Get(key)
	if !ok {
		return false
	}
	data, ok := value.([]byte)
	if !ok {
		text, ok := value.(string)
		if !ok {
			return false
		}
		data = []byte(text)
	}
	return json.Unmarshal(data, target) == nil
}

func (m *MySQLDao[E]) storeCached(key string, value interface{}) {
	if !m.cacheEnabled() {
		return
	}
	data, err := json.Marshal(value)
	if err == nil {
		m.cache.Store.Set(key, data, m.cache.TTL)
	}
}

func (m *MySQLDao[E]) clearCache() {
	if !m.cacheEnabled() {
		return
	}
	prefix := m.cacheTablePrefix()
	m.cache.Store.Range(func(key, _ interface{}) bool {
		if keyText, ok := key.(string); ok && strings.HasPrefix(keyText, prefix) {
			m.cache.Store.Delete(key)
		}
		return true
	})
}

// Where mysql动态查询数据
func (m *MySQLDao[E]) Where(query interface{}, args ...interface{}) *gorm.DB {
	if m.Tag == nil {
		m.Tag = notag
	}
	conn, err := db.Mysql.GetConnection(m.Tag())
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return nil
	}
	if m.debug {
		conn = conn.Debug()
	}
	if m.ctx != nil {
		conn = conn.WithContext(*m.ctx)
	}
	var e E
	return conn.Model(e).Where(query, args...)
}

// Create mysql动态插入数据
func (receiver *MySQLDao[E]) Create(entity *E) error {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	conn, err := db.Mysql.GetConnection(receiver.Tag())
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return errors.New("数据库连接失败")
	}
	if receiver.debug {
		conn = conn.Debug()
	}
	if receiver.ctx != nil {
		conn = conn.WithContext(*receiver.ctx)
	}
	err = conn.Create(entity).Error
	if err != nil {
		logger.Error("数据库插入失败: " + err.Error())
		return errors.New("数据库插入失败")
	}
	receiver.clearCache()
	return nil
}

// MultiCreate mysql动态插入多条数据
func (receiver *MySQLDao[E]) MultiCreate(entities []*E) error {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	conn, err := db.Mysql.GetConnection(receiver.Tag())
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return errors.New("数据库连接失败")
	}
	if receiver.debug {
		conn = conn.Debug()
	}
	if receiver.ctx != nil {
		conn = conn.WithContext(*receiver.ctx)
	}
	err = conn.Create(entities).Error
	if err != nil {
		logger.Error("数据库插入失败: " + err.Error())
		return errors.New("数据库插入失败")
	}
	receiver.clearCache()
	return nil
}

// Delete mysql动态删除数据
func (receiver *MySQLDao[E]) Delete(entity E) error {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	conn, err := db.Mysql.GetConnection(receiver.Tag())
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return errors.New("数据库连接失败")
	}
	var e E
	if receiver.debug {
		conn = conn.Debug()
	}
	if receiver.ctx != nil {
		conn = conn.WithContext(*receiver.ctx)
	}
	err = conn.Where(entity).Delete(&e).Error
	if err != nil {
		logger.Error("数据库删除失败: " + err.Error())
		return errors.New("数据库删除失败")
	}
	receiver.clearCache()
	return nil
}

// Updates mysql动态更新数据
func (receiver *MySQLDao[E]) Updates(entity *E) error {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	conn, err := db.Mysql.GetConnection(receiver.Tag())
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return errors.New("数据库连接失败")
	}
	if receiver.debug {
		conn = conn.Debug()
	}
	if receiver.ctx != nil {
		conn = conn.WithContext(*receiver.ctx)
	}
	err = conn.Updates(entity).Error
	if err != nil {
		logger.Error("数据库更新失败: " + err.Error())
		return errors.New("数据库更新失败")
	}
	receiver.clearCache()
	return nil
}

// Save mysql动态保存数据
func (receiver *MySQLDao[E]) Save(entity *E) error {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	conn, err := db.Mysql.GetConnection(receiver.Tag())
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return errors.New("数据库连接失败")
	}
	if receiver.debug {
		conn = conn.Debug()
	}
	if receiver.ctx != nil {
		conn = conn.WithContext(*receiver.ctx)
	}
	err = conn.Save(entity).Error
	if err != nil {
		logger.Error("数据库保存失败: " + err.Error())
		return errors.New("数据库保存失败")
	}
	receiver.clearCache()
	return nil
}

// All mysql动态查询数据
func (receiver *MySQLDao[E]) All(entity E, opts ...QueryOption) ([]E, error) {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	conn, err := db.Mysql.GetConnection(receiver.Tag())
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return nil, errors.New("数据库连接失败")
	}

	var result = make([]E, 0)
	key, cacheable := receiver.cacheKey("all", struct {
		Entity E
		Opts   []QueryOption
	}{entity, opts})
	if cacheable && receiver.loadCached(key, &result) {
		return result, nil
	}
	if receiver.debug {
		conn = conn.Debug()
	}
	if receiver.ctx != nil {
		conn = conn.WithContext(*receiver.ctx)
	}
	if opts != nil && len(opts) > 0 {
		for _, opt := range opts {
			if opt.Preloads != nil && len(opt.Preloads) > 0 {
				for _, preload := range opt.Preloads {
					conn = conn.Preload(preload)
				}
			}
			if opt.OrderBy != nil && len(opt.OrderBy) > 0 {
				conn = conn.Order(strings.Join(opt.OrderBy, ","))
			}
		}
	}
	err = conn.Where(entity).Find(&result).Error
	if err != nil {
		logger.Error("数据库查询失败: " + err.Error())
		return nil, errors.New("数据库查询失败")
	}
	if cacheable {
		receiver.storeCached(key, result)
	}
	return result, nil
}

// One mysql动态查询一条数据
func (receiver *MySQLDao[E]) One(entity E) (*E, error) {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	conn, err := db.Mysql.GetConnection(receiver.Tag())
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return nil, errors.New("数据库连接失败")
	}
	var result E
	key, cacheable := receiver.cacheKey("one", entity)
	if cacheable && receiver.loadCached(key, &result) {
		return &result, nil
	}
	if receiver.debug {
		conn = conn.Debug()
	}
	if receiver.ctx != nil {
		conn = conn.WithContext(*receiver.ctx)
	}
	err = conn.Where(entity).First(&result).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		logger.Error("数据库查询失败: " + err.Error())
		return nil, errors.New("数据库查询失败")
	}
	if cacheable {
		receiver.storeCached(key, result)
	}
	return &result, nil
}

// Exists mysql动态查询是否存在数据
func (receiver *MySQLDao[E]) Exists(entity E) bool {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	conn, err := db.Mysql.GetConnection(receiver.Tag())
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return false
	}
	var result E
	key, cacheable := receiver.cacheKey("exists", entity)
	var exists bool
	if cacheable && receiver.loadCached(key, &exists) {
		return exists
	}
	if receiver.debug {
		conn = conn.Debug()
	}
	if receiver.ctx != nil {
		conn = conn.WithContext(*receiver.ctx)
	}
	err = conn.Where(entity).First(&result).Error
	exists = err == nil
	if cacheable {
		receiver.storeCached(key, exists)
	}
	return exists
}

// Count mysql统计记录数
func (receiver *MySQLDao[E]) Count(entity E) (int64, error) {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	conn, err := db.Mysql.GetConnection(receiver.Tag())
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return 0, errors.New("数据库连接失败")
	}
	var count int64
	key, cacheable := receiver.cacheKey("count", entity)
	if cacheable && receiver.loadCached(key, &count) {
		return count, nil
	}
	if receiver.debug {
		conn = conn.Debug()
	}
	if receiver.ctx != nil {
		conn = conn.WithContext(*receiver.ctx)
	}
	err = conn.Model(&entity).Where(entity).Count(&count).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil
		}
		logger.Error("数据库查询失败: " + err.Error())
		return 0, errors.New("数据库查询失败")
	}
	if cacheable {
		receiver.storeCached(key, count)
	}
	return count, nil
}

// Pager mysql简单分页查询数据
func (receiver *MySQLDao[E]) Pager(conn *gorm.DB, page, size int) ([]E, *models.ResultPage, error) {
	if conn == nil {
		return nil, nil, errors.New("数据库连接失败")
	}
	// 默认分页大小为20条
	if size == 0 {
		size = 20
	}
	var result = make([]E, 0)
	var count int64
	var p = models.ResultPage{
		Index: page,
		Size:  size,
	}
	if receiver.debug {
		conn = conn.Debug()
	}
	if receiver.ctx != nil {
		conn = conn.WithContext(*receiver.ctx)
	}
	var e E
	err := conn.Model(e).Count(&count).Error
	if err != nil {
		logger.Error("数据库查询失败: " + err.Error())
		return nil, nil, errors.New("数据库查询失败")
	}
	p.Total = int(count)
	p.Count = int(math.Ceil(float64(count) / float64(size)))
	if count == 0 || count < int64((page-1)*size) {
		return result, &p, err
	}
	err = conn.Offset((page - 1) * size).Limit(size).Find(&result).Error
	if err != nil {
		logger.Error("数据库查询失败: " + err.Error())
		return nil, nil, errors.New("数据库查询失败")
	}
	return result, &p, nil
}

func (receiver *MySQLDao[E]) Alias(alias string) *gorm.DB {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	conn, err := db.Mysql.GetConnection(receiver.Tag())
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return nil
	}
	var e E
	return conn.Table(fmt.Sprintf("%s AS %s", e.TableName(), alias))
}

func (receiver *MySQLDao[E]) DB() *gorm.DB {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	conn, err := db.Mysql.GetConnection(receiver.Tag())
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return nil
	}
	var e E
	return conn.Model(&e)
}

func (receiver *MySQLDao[E]) JOIN(alias, query string, args ...interface{}) *gorm.DB {
	return receiver.Alias(alias).Joins(query, args...)
}
