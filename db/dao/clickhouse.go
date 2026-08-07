package dao

import (
	"context"
	"errors"
	"math"
	"strings"

	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/models"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type ClickhouseDao[E schema.Tabler] struct {
	debug bool
	ctx   *context.Context
	Tag   func() string
}

func (m *ClickhouseDao[E]) Debug() *ClickhouseDao[E] {
	return &ClickhouseDao[E]{
		debug: true,
		Tag:   m.Tag,
	}
}

func (m *ClickhouseDao[E]) WithContext(ctx *context.Context) *ClickhouseDao[E] {
	m.ctx = ctx
	return m
}

// Where clickhouse动态查询数据
func (m *ClickhouseDao[E]) Where(query interface{}, args ...interface{}) *gorm.DB {
	if m.Tag == nil {
		m.Tag = notag
	}
	conn, err := db.Clickhouse.GetConnection(m.Tag())
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

// Create clickhouse动态插入数据
func (receiver *ClickhouseDao[E]) Create(entity *E) error {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	conn, err := db.Clickhouse.GetConnection(receiver.Tag())
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
	return nil
}

// MultiCreate clickhouse动态插入多条数据
func (receiver *ClickhouseDao[E]) MultiCreate(entities []*E) error {
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
	return nil
}

// Delete clickhouse动态删除数据
func (receiver *ClickhouseDao[E]) Delete(entity E) error {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	conn, err := db.Clickhouse.GetConnection(receiver.Tag())
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
	return nil
}

// Updates clickhouse动态更新数据
func (receiver *ClickhouseDao[E]) Updates(entity *E) error {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	conn, err := db.Clickhouse.GetConnection(receiver.Tag())
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
	return nil
}

// Save clickhouse动态保存数据
func (receiver *ClickhouseDao[E]) Save(entity *E) error {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	conn, err := db.Clickhouse.GetConnection(receiver.Tag())
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
	return nil
}

// All clickhouse动态查询数据
func (receiver *ClickhouseDao[E]) All(entity E, opts ...QueryOption) ([]E, error) {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	conn, err := db.Clickhouse.GetConnection(receiver.Tag())
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
	return result, nil
}

// One clickhouse动态查询一条数据
func (receiver *ClickhouseDao[E]) One(entity E) (*E, error) {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	conn, err := db.Clickhouse.GetConnection(receiver.Tag())
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
	return &result, nil
}

// Exists clickhouse动态查询是否存在数据
func (receiver *ClickhouseDao[E]) Exists(entity E) bool {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	conn, err := db.Clickhouse.GetConnection(receiver.Tag())
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

// Count clickhouse统计记录数
func (receiver *ClickhouseDao[E]) Count(entity E) (int64, error) {
	if receiver.Tag == nil {
		receiver.Tag = notag
	}
	conn, err := db.Clickhouse.GetConnection(receiver.Tag())
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

// Pager clickhouse简单分页查询数据
func (receiver *ClickhouseDao[E]) Pager(conn *gorm.DB, page, size int) ([]E, *models.ResultPage, error) {
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
