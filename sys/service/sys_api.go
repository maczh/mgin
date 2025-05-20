package service

import (
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/sys/dao"
	"time"
)

type sysApiService struct{}

func (d *sysApiService) CreateApi(api *sys.SysApi) error {
	return dao.SysApiDao.Create(api)
}

func (d *sysApiService) GetApi(id uint) (*sys.SysApi, error) {
	api, err := dao.SysApiDao.One(sys.SysApi{ID: id})
	if err != nil {
		logs.Error("获取API接口失败: {}", err.Error())
		return nil, err
	}
	return api, nil
}
func (d *sysApiService) GetUri(uri string) (*sys.SysApi, error) {
	api, err := dao.SysApiDao.One(sys.SysApi{APIPath: uri})

	if err != nil {
		logs.Error("获取API接口失败: {}", err.Error())
		return nil, err
	}
	return api, nil
}

func (d *sysApiService) UpdateApi(api *sys.SysApi) error {
	dep, err := dao.SysApiDao.One(sys.SysApi{ID: api.ID})
	if err != nil {
		logs.Error("获取API接口失败: {}", err.Error())
		return err
	}
	api.CreateAt = dep.CreateAt
	api.UpdateAt = time.Now()
	return dao.SysApiDao.Updates(api)
}

func (d *sysApiService) DeleteApi(id uint) error {
	return dao.SysApiDao.Delete(sys.SysApi{ID: id})
}

func (d *sysApiService) ListApi(page, pageSize int, group string, needAuth int) ([]sys.SysApi, *models.ResultPage, error) {
	dbs, err := db.Mysql.GetConnection()
	if err != nil {
		logs.Error("获取数据库连接失败: {}", err.Error())
		return nil, nil, err
	}
	if group != "" {
		dbs = dbs.Where("api_group = ?", group)
	}
	if needAuth > -1 {
		dbs = dbs.Where("need_auth = ?", needAuth)
	}
	dbs = dbs.Order("id ASC")
	apis, pages, err := dao.SysApiDao.Pager(dbs, page, pageSize)

	if err != nil {
		logs.Error("获取API接口信息失败: {}", err.Error())
		return nil, nil, err
	}
	return apis, pages, err
}
