package dao

import (
	"errors"
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/models"
	"github.com/sadlil/gologger"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"math"
	"strings"
)

type MySQLDao[E schema.Tabler] struct {
	debug bool
	Tag   func() string
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
	err = conn.Create(entity).Error
	if err != nil {
		logger.Error("数据库插入失败: " + err.Error())
		return errors.New("数据库插入失败")
	}
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
	err = conn.Create(entities).Error
	if err != nil {
		logger.Error("数据库插入失败: " + err.Error())
		return errors.New("数据库插入失败")
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
	err = conn.Where(entity).Delete(&e).Error
	if err != nil {
		logger.Error("数据库删除失败: " + err.Error())
		return errors.New("数据库删除失败")
	}
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
	err = conn.Updates(entity).Error
	if err != nil {
		logger.Error("数据库更新失败: " + err.Error())
		return errors.New("数据库更新失败")
	}
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
	err = conn.Save(entity).Error
	if err != nil {
		logger.Error("数据库保存失败: " + err.Error())
		return errors.New("数据库保存失败")
	}
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
	if receiver.debug {
		conn = conn.Debug()
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
	if receiver.debug {
		conn = conn.Debug()
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
