package service

import (
	"errors"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/dao"
)

type sysConfigService struct {
}

// Create 新增系统配置
func (s *sysConfigService) Create(req sys.SysConfig) (*sys.SysConfig, error) {
	//检查key是否存在
	count, err := dao.SysConfigDao.Count(sys.SysConfig{Key: req.Key})
	if err != nil {
		logs.Error("查询配置信息失败:{}", err.Error())
		return nil, err
	}
	if count > 0 {
		logs.Error("配置键名重复:{}", req.Key)
		return nil, err
	}
	err = dao.SysConfigDao.Create(&req)
	return &req, err
}

// Update 修改系统配置,只允许修改value和name、description字段
func (s *sysConfigService) Update(req sys.SysConfig) error {
	config, err := dao.SysConfigDao.One(sys.SysConfig{ID: req.ID})
	if err != nil {
		logs.Error("查询配置信息失败:{}", err.Error())
		return err
	}
	if config == nil {
		logs.Error("配置不存在:{}", req.ID)
		return err
	}
	if config.Key != req.Key {
		logs.Error("配置键名不能修改:{}", req.Key)
		return err
	}
	config.Value = req.Value
	config.Name = req.Name
	config.Description = req.Description
	return dao.SysConfigDao.Save(config)
}

// Delete 删除系统配置
func (s *sysConfigService) Delete(id int) error {
	return dao.SysConfigDao.Delete(sys.SysConfig{ID: int64(id)})
}

// Get 获取系统配置
func (s *sysConfigService) Get(req request.GetSysConfigReq) (*sys.SysConfig, error) {
	if req.ID > 0 { //根据ID查询
		return dao.SysConfigDao.One(sys.SysConfig{ID: int64(req.ID)})
	}
	if req.Key != "" { //根据Key查询
		return dao.SysConfigDao.One(sys.SysConfig{Key: req.Key})
	}
	return nil, nil
}

// List 分页查询系统配置
func (s *sysConfigService) List(req request.ListSysConfigReq) ([]sys.SysConfig, *models.ResultPage, error) {
	mysql := dao.SysConfigDao.Where("1 = 1").Order("module,id ASC")
	if req.Module != "" {
		mysql = mysql.Where("module = ?", req.Module)
	}
	return dao.SysConfigDao.Pager(mysql, req.Page, req.PageSize)
}

// MultiGet 获取多个系统配置
func (s *sysConfigService) MultiGet(keys []string) ([]sys.SysConfig, error) {
	if len(keys) == 0 { //没有传入key，则返回空
		return nil, errors.New("没有传入key")
	}
	var configList []sys.SysConfig
	err := dao.SysConfigDao.Where("key in (?)", keys).Find(&configList).Error
	return configList, err
}
