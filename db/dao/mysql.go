package dao

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/maczh/mgin/cache"
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/models"
	"github.com/sadlil/gologger"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type MySQLDao[E schema.Tabler] struct {
	debug            bool
	ctx              *context.Context
	Tag              func() string
	CacheKeyPatterns map[string]bool
}

type QueryOption struct {
	Preloads []string
	OrderBy  []string
}

var logger = gologger.GetLogger()

func (m *MySQLDao[E]) Debug() *MySQLDao[E] {
	return &MySQLDao[E]{
		debug: true,
		Tag:   m.Tag,
	}
}

func (m *MySQLDao[E]) WithContext(ctx *context.Context) *MySQLDao[E] {
	m.ctx = ctx
	return m
}

func (m *MySQLDao[E]) WithCacheKey(cacheKeyPattern string) *MySQLDao[E] {
	if m.CacheKeyPatterns == nil {
		m.CacheKeyPatterns = make(map[string]bool)
	}
	m.CacheKeyPatterns[cacheKeyPattern] = true
	if m.ctx == nil {
		ctx := context.Background()
		m.ctx = &ctx
	}
	*m.ctx = context.WithValue(*m.ctx, "cacheKeyPattern", cacheKeyPattern)
	return m
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
	receiver.ClearCache(*entity)
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
	for _, entity := range entities {
		receiver.ClearCache(*entity)
	}
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
	receiver.ClearCache(entity)
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
	receiver.ClearCache(*entity)
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
	receiver.ClearCache(*entity)
	return nil
}

func (m *MySQLDao[E]) genCacheKey(entity E) string {
	if m.ctx == nil {
		return ""
	}
	ctx := *m.ctx
	cacheKeyPattern := ctx.Value("cacheKeyPattern")
	if cacheKeyPattern == nil {
		return ""
	}
	pattern := cacheKeyPattern.(string)
	return m.getCacheKey(pattern, entity)
}
func (m *MySQLDao[E]) getCacheKey(pattern string, entity E) string {
	placeholderRegex := regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)
	matches := placeholderRegex.FindAllStringSubmatch(pattern, -1)
	if len(matches) == 0 {
		return ""
	}

	val := reflect.ValueOf(entity)
	typ := reflect.TypeOf(entity)

	if typ.Kind() == reflect.Ptr {
		if val.IsNil() {
			return ""
		}
		val = val.Elem()
		typ = typ.Elem()
	}

	result := pattern
	allValid := true

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		fieldName := match[1]
		fieldValue, found := m.getFieldValue(val, typ, fieldName)
		if !found {
			allValid = false
			break
		}
		result = strings.ReplaceAll(result, "{"+fieldName+"}", fieldValue)
	}

	if !allValid {
		return ""
	}

	return result
}

func (m *MySQLDao[E]) getFieldValue(val reflect.Value, typ reflect.Type, fieldName string) (string, bool) {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		if field.Name == fieldName {
			if fieldVal.IsZero() {
				return "", false
			}
			return convertValueToString(fieldVal), true
		}

		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			tagName := strings.Split(jsonTag, ",")[0]
			if tagName == fieldName {
				if fieldVal.IsZero() {
					return "", false
				}
				return convertValueToString(fieldVal), true
			}
		}

		if gormTag := field.Tag.Get("gorm"); gormTag != "" {
			parts := strings.Split(gormTag, ";")
			for _, p := range parts {
				if strings.HasPrefix(p, "column:") {
					colName := strings.TrimPrefix(p, "column:")
					if colName == fieldName {
						if fieldVal.IsZero() {
							return "", false
						}
						return convertValueToString(fieldVal), true
					}
				}
			}
		}
	}
	return "", false
}

func convertValueToString(v reflect.Value) string {
	switch v.Kind() {
	case reflect.String:
		return v.String()

	case reflect.Bool:
		return strconv.FormatBool(v.Bool())

	case reflect.Int:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Int8:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Int16:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Int32:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)

	case reflect.Uint:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Uint8:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Uint16:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Uint32:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10)

	case reflect.Float32:
		return strconv.FormatFloat(v.Float(), 'f', -1, 32)
	case reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64)

	case reflect.Ptr:
		if v.IsNil() {
			return ""
		}
		return convertValueToString(v.Elem())

	case reflect.Interface:
		if v.IsNil() {
			return ""
		}
		return convertValueToString(v.Elem())

	default:
		iface := v.Interface()
		switch t := iface.(type) {
		case time.Time:
			return t.Format(time.RFC3339Nano)
		case fmt.Stringer:
			return t.String()
		default:
			return fmt.Sprintf("%v", iface)
		}
	}
}

func (m *MySQLDao[E]) ClearCache(entity E) {
	for cacheKeypattern, _ := range m.CacheKeyPatterns {
		cacheKey := m.getCacheKey(cacheKeypattern, entity)
		if strings.Contains(cacheKey, "{") {
			continue
		}
		cache.OnMemCache(entity.TableName()).Delete(cacheKey)
		cache.OnMemCache(entity.TableName()).Delete("list:" + cacheKey)
	}
}

// All mysql动态查询数据
func (receiver *MySQLDao[E]) All(entity E, opts ...QueryOption) ([]E, error) {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	cacheKey := "list:" + receiver.genCacheKey(entity)
	if cacheKey != "" {
		if result, ok := cache.OnMemCache(entity.TableName()).Get(cacheKey); ok && result != nil {
			return result.([]E), nil
		}
	}
	conn, err := db.Mysql.GetConnection(receiver.Tag())
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return nil, errors.New("数据库连接失败")
	}

	var result = make([]E, 0)
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
	if cacheKey != "" {
		cache.OnMemCache(entity.TableName()).Set(cacheKey, result, time.Hour)
	}
	return result, nil
}

// One mysql动态查询一条数据
func (receiver *MySQLDao[E]) One(entity E) (*E, error) {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	cacheKey := receiver.genCacheKey(entity)
	if cacheKey != "" {
		if result, ok := cache.OnMemCache(entity.TableName()).Get(cacheKey); ok && result != nil {
			res := result.(E)
			return &res, nil
		}
	}
	conn, err := db.Mysql.GetConnection(receiver.Tag())
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return nil, errors.New("数据库连接失败")
	}
	var result E
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
	if cacheKey != "" {
		cache.OnMemCache(entity.TableName()).Set(cacheKey, result, time.Hour)
	}
	return &result, nil
}

// Exists mysql动态查询是否存在数据
func (receiver *MySQLDao[E]) Exists(entity E) bool {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	cacheKey := receiver.genCacheKey(entity)
	if cacheKey != "" {
		if _, ok := cache.OnMemCache(entity.TableName()).Get(cacheKey); ok {
			return ok
		}
	}
	conn, err := db.Mysql.GetConnection(receiver.Tag())
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return false
	}
	var result *E
	if receiver.debug {
		conn = conn.Debug()
	}
	if receiver.ctx != nil {
		conn = conn.WithContext(*receiver.ctx)
	}
	_ = conn.Where(entity).First(result).Error
	return result != nil
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
	return count, nil
}

// Pager mysql简单分页查询数据
func (receiver *MySQLDao[E]) Pager(conn *gorm.DB, page, size int) ([]E, *models.ResultPage, error) {
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
