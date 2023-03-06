package dao

import (
	"errors"
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/models"
	"github.com/sadlil/gologger"
	"gorm.io/gorm/schema"
)

type MySQLDao[E schema.Tabler] struct{}

var logger = gologger.GetLogger()

// Create mysql动态插入数据
func (receiver *MySQLDao[E]) Create(entity *E, tag ...string) error {
	conn, err := db.Mysql.GetConnection(tag...)
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return errors.New("数据库连接失败")
	}
	err = conn.Debug().Create(entity).Error
	if err != nil {
		logger.Error("数据库插入失败: " + err.Error())
		return errors.New("数据库插入失败")
	}
	return nil
}

// MultiCreate mysql动态插入多条数据
func (receiver *MySQLDao[E]) MultiCreate(entities []*E, tag ...string) error {
	conn, err := db.Mysql.GetConnection(tag...)
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return errors.New("数据库连接失败")
	}
	err = conn.Debug().Create(entities).Error
	if err != nil {
		logger.Error("数据库插入失败: " + err.Error())
		return errors.New("数据库插入失败")
	}
	return nil
}

// Delete mysql动态删除数据
func (receiver *MySQLDao[E]) Delete(entity E, tag ...string) error {
	conn, err := db.Mysql.GetConnection(tag...)
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return errors.New("数据库连接失败")
	}
	var e E
	err = conn.Debug().Where(entity).Delete(&e).Error
	if err != nil {
		logger.Error("数据库删除失败: " + err.Error())
		return errors.New("数据库删除失败")
	}
	return nil
}

// Updates mysql动态更新数据
func (receiver *MySQLDao[E]) Updates(entity *E, tag ...string) error {
	conn, err := db.Mysql.GetConnection(tag...)
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return errors.New("数据库连接失败")
	}
	err = conn.Debug().Updates(entity).Error
	if err != nil {
		logger.Error("数据库更新失败: " + err.Error())
		return errors.New("数据库更新失败")
	}
	return nil
}

// Save mysql动态保存数据
func (receiver *MySQLDao[E]) Save(entity *E, tag ...string) error {
	conn, err := db.Mysql.GetConnection(tag...)
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return errors.New("数据库连接失败")
	}
	err = conn.Debug().Save(entity).Error
	if err != nil {
		logger.Error("数据库保存失败: " + err.Error())
		return errors.New("数据库保存失败")
	}
	return nil
}

// All mysql动态查询数据
func (receiver *MySQLDao[E]) All(entity E, tag ...string) ([]E, error) {
	conn, err := db.Mysql.GetConnection(tag...)
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return nil, errors.New("数据库连接失败")
	}

	var result = make([]E, 0)
	err = conn.Debug().Where(entity).Find(&result).Error
	if err != nil {
		logger.Error("数据库查询失败: " + err.Error())
		return nil, errors.New("数据库查询失败")
	}
	return result, nil
}

// One mysql动态查询一条数据
func (receiver *MySQLDao[E]) One(entity E, tag ...string) (*E, error) {
	conn, err := db.Mysql.GetConnection(tag...)
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return nil, errors.New("数据库连接失败")
	}
	var result *E
	err = conn.Debug().Where(entity).Find(&result).Error
	if err != nil {
		logger.Error("数据库查询失败: " + err.Error())
		return nil, errors.New("数据库查询失败")
	}
	return result, nil
}

// Pager mysql简单分页查询数据
func (receiver *MySQLDao[E]) Pager(entity E, page, size int, tag ...string) ([]E, *models.ResultPage, error) {
	conn, err := db.Mysql.GetConnection(tag...)
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
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
	err = conn.Debug().Where(entity).Count(&count).Error
	if err != nil {
		logger.Error("数据库查询失败: " + err.Error())
		return nil, nil, errors.New("数据库查询失败")
	}
	p.Total = int(count)
	p.Count = int(count/int64(size)) + 1
	if count == 0 || count < int64((page-1)*size) {
		return result, &p, err
	}
	err = conn.Debug().Where(entity).Offset((page - 1) * size).Limit(size).Find(&result).Error
	if err != nil {
		logger.Error("数据库查询失败: " + err.Error())
		return nil, nil, errors.New("数据库查询失败")
	}
	return result, &p, nil
}
