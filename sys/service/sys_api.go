package service

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/vo"
	"github.com/maczh/mgin/sys/dao"
	"time"
)

type sysApiService struct {
	ctx *gin.Context
}

func (a *sysApiService) WithContext(c *gin.Context) *sysApiService {
	a.ctx = c
	return a
}

func (d *sysApiService) CreateApi(api *sys.SysApi) error {
	api.CreateAt = time.Now()
	api.UpdateAt = time.Now()
	if d.ctx != nil {
		api.CreateBy = getCurrentNickName(d.ctx)
	}
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
	if d.ctx != nil {
		api.UpdateBy = getCurrentNickName(d.ctx)
	}
	return dao.SysApiDao.Save(api)
}

func (d *sysApiService) DeleteApi(id uint) error {
	return dao.SysApiDao.Delete(sys.SysApi{ID: id})
}

func (d *sysApiService) ListApi(page, pageSize int, group string, needAuth int) ([]sys.SysApi, *models.ResultPage, error) {
	dbs := dao.SysApiDao.Where("del_flag = 0")
	if group != "" {
		dbs = dbs.Where("api_group = ?", group)
	}
	if needAuth > 0 {
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

func (d *sysApiService) ListApiByGroup() ([]vo.ListSysApiByGroupResp, error) {
	var list []sys.SysApi
	err := dao.SysApiDao.Where("del_flag = 0").Order("api_group ASC").Find(&list).Error
	if err != nil {
		logs.Error("获取API分组信息失败: {}", err.Error())
		return nil, err
	}
	var resp = make([]vo.ListSysApiByGroupResp, 0)
	group := list[0].APIGroup
	apis := make([]sys.SysApi, 0)
	for _, api := range list {
		if api.APIGroup != group {
			resp = append(resp, vo.ListSysApiByGroupResp{
				Group: group,
				Apis:  apis,
			})
			group = api.APIGroup
			apis = make([]sys.SysApi, 0)
		}
		apis = append(apis, api)
	}
	resp = append(resp, vo.ListSysApiByGroupResp{
		Group: group,
		Apis:  apis,
	})
	return resp, nil
}
