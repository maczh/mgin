package service

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/dao"
	"time"
)

type sysDictService struct {
	ctx *gin.Context
}

func (s *sysDictService) WithContext(c *gin.Context) *sysDictService {
	s.ctx = c
	return s
}

// Add 新增字典项
func (s *sysDictService) Add(req request.CreateDictReq) (dict *sys.SysDict, err error) {
	dic, err := s.Get(request.GetDictReq{Type: req.Type, Name: req.Name, Key: req.Key})
	if err != nil {
		logs.Error("查询字典项失败: {}", err.Error())
		return nil, err
	}
	if dic != nil {
		return nil, errors.New("字典项已存在")
	}
	dict = &sys.SysDict{
		Type:     req.Type,
		ParentId: req.ParentId,
		Name:     req.Name,
		Key:      req.Key,
		Value:    req.Value,
		Sort:     req.Sort,
		Remark:   req.Remark,
	}
	dict.CreateAt = time.Now()
	dict.UpdateAt = time.Now()
	if s.ctx != nil {
		dict.CreateBy = getCurrentNickName(s.ctx)
	}
	err = dao.SysDictDao.Create(dict)
	return dict, err
}

// Get 获取字典项
func (s *sysDictService) Get(req request.GetDictReq) (dict *sys.SysDict, err error) {
	if req.ID > 0 {
		dict, err = dao.SysDictDao.One(sys.SysDict{ID: int(req.ID)})
	} else {
		dict, err = dao.SysDictDao.One(sys.SysDict{Type: req.Type, Name: req.Name, Key: req.Key})
	}
	return dict, err
}

// Update 更新字典项
func (s *sysDictService) Update(dict *sys.SysDict) error {
	dict.UpdateAt = time.Now()
	if s.ctx != nil {
		dict.UpdateBy = getCurrentNickName(s.ctx)
	}
	err := dao.SysDictDao.Save(dict)
	if err != nil {
		logs.Error("更新字典项失败: {}", err.Error())
		return err
	}
	return nil
}

// Delete 删除字典项
func (s *sysDictService) Delete(id int64) error {
	dict, err := dao.SysDictDao.One(sys.SysDict{ID: int(id)})
	if err != nil {
		logs.Error("查询字典项失败: {}", err.Error())
		return err
	}
	if dict == nil {
		return errors.New("字典项不存在")
	}
	err = dao.SysDictDao.Delete(*dict)
	return err
}

// List 获取字典项列表
func (s *sysDictService) List(req request.ListDictReq) ([]sys.SysDict, *models.ResultPage, error) {
	var mysql = dao.SysDictDao.Where("del_flag = 0")
	if req.ParentId > 0 {
		mysql = dao.SysDictDao.Where("parent_id = ?", req.ParentId)
	}
	if req.Type != "" {
		mysql = mysql.Where("type =?", req.Type)
	}
	if req.Name != "" {
		mysql = mysql.Where("name =?", req.Name)
	}
	result, ppages, err := dao.SysDictDao.Pager(mysql, req.Page, req.PageSize)
	return result, ppages, err
}
