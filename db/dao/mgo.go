package dao

import (
	"errors"
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/models"
	"gopkg.in/mgo.v2/bson"
)

// MgoDao 注意使用前必须先将CollectionName赋值
type MgoDao[E any] struct {
	CollectionName string
}

// Insert mongo动态插入数据
func (m *MgoDao[E]) Insert(entity *E, tag ...string) error {
	if m.CollectionName == "" {
		return errors.New("CollectionName未定义")
	}
	conn, err := db.Mongo.GetConnection(tag...)
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return errors.New("数据库连接失败")
	}
	err = conn.C(m.CollectionName).Insert(entity)
	if err != nil {
		logger.Error("数据库插入失败: " + err.Error())
		return errors.New("数据库插入失败")
	}
	return nil
}

// Delete mongo动态删除数据
func (m *MgoDao[E]) Delete(query bson.M, tag ...string) error {
	if m.CollectionName == "" {
		return errors.New("CollectionName未定义")
	}
	conn, err := db.Mongo.GetConnection(tag...)
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return errors.New("数据库连接失败")
	}
	err = conn.C(m.CollectionName).Remove(query)
	if err != nil {
		logger.Error("数据库删除失败: " + err.Error())
		return errors.New("数据库删除失败")
	}
	return nil
}

// Updates mongo动态更新数据
func (m *MgoDao[E]) Updates(id bson.ObjectId, fields bson.M, tag ...string) error {
	if m.CollectionName == "" {
		return errors.New("CollectionName未定义")
	}
	conn, err := db.Mongo.GetConnection(tag...)
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return errors.New("数据库连接失败")
	}
	err = conn.C(m.CollectionName).UpdateId(id, fields)
	if err != nil {
		logger.Error("数据库更新失败: " + err.Error())
		return errors.New("数据库更新失败")
	}
	return nil
}

// All mongo动态查询数据
func (m *MgoDao[E]) All(query bson.M, tag ...string) ([]E, error) {
	if m.CollectionName == "" {
		return nil, errors.New("CollectionName未定义")
	}
	conn, err := db.Mongo.GetConnection(tag...)
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return nil, errors.New("数据库连接失败")
	}

	var result = make([]E, 0)
	err = conn.C(m.CollectionName).Find(query).All(&result)
	if err != nil {
		logger.Error("数据库查询失败: " + err.Error())
		return nil, errors.New("数据库查询失败")
	}
	return result, nil
}

// One mongo动态查询一条数据
func (m *MgoDao[E]) One(query bson.M, tag ...string) (*E, error) {
	if m.CollectionName == "" {
		return nil, errors.New("CollectionName未定义")
	}
	conn, err := db.Mongo.GetConnection(tag...)
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return nil, errors.New("数据库连接失败")
	}
	var result *E
	err = conn.C(m.CollectionName).Find(query).One(result)
	if err != nil {
		logger.Error("数据库查询失败: " + err.Error())
		return nil, errors.New("数据库查询失败")
	}
	return result, nil
}

// Pager mongo简单分页查询数据
func (m *MgoDao[E]) Pager(query bson.M, page, size int, tag ...string) ([]E, *models.ResultPage, error) {
	if m.CollectionName == "" {
		return nil, nil, errors.New("CollectionName未定义")
	}
	conn, err := db.Mongo.GetConnection(tag...)
	if err != nil {
		logger.Error("数据库连接失败: " + err.Error())
		return nil, nil, errors.New("数据库连接失败")
	}
	// 默认分页大小为20条
	if size == 0 {
		size = 20
	}
	var result = make([]E, 0)
	var count int
	var p = models.ResultPage{
		Index: page,
		Size:  size,
	}
	count, err = conn.C(m.CollectionName).Count()
	if err != nil {
		logger.Error("数据库查询失败: " + err.Error())
		return nil, nil, errors.New("数据库查询失败")
	}
	p.Total = count
	p.Count = (count / size) + 1
	if count == 0 || count < (page-1)*size {
		return result, &p, err
	}
	err = conn.C(m.CollectionName).Find(query).Skip((page - 1) * size).Limit(size).All(&result)
	if err != nil {
		logger.Error("数据库查询失败: " + err.Error())
		return nil, nil, errors.New("数据库查询失败")
	}
	return result, &p, nil
}
