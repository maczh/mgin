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

type sysResourceService struct {
	ctx *gin.Context
}

func (s *sysResourceService) WithContext(c *gin.Context) *sysResourceService {
	s.ctx = c
	return s
}

// Add 新增资源
func (s *sysResourceService) Add(req request.CreateResourceReq) (*sys.SysResource, error) {
	//检查资源名称是否存在
	if dao.SysResourceDao.Exists(sys.SysResource{Name: req.Name}) {
		return nil, errors.New("资源名称已存在")
	}
	//检查资源路径是否存在
	if dao.SysResourceDao.Exists(sys.SysResource{Path: req.Path}) {
		return nil, errors.New("资源编码已存在")
	}
	resource := &sys.SysResource{
		ParentId:       req.ParentId,
		Name:           req.Name,
		Path:           req.Path,
		Component:      req.Component,
		Redirect:       req.Redirect,
		Icon:           req.Icon,
		Title:          req.Title,
		Hidden:         req.Hidden,
		AlwaysShow:     req.AlwaysShow,
		ActiveResource: req.ActiveResource,
		KeepAlive:      req.KeepAlive,
		Breadcrumb:     req.Breadcrumb,
		Affix:          req.Affix,
		Sort:           req.Sort,
		Status:         1,
	}
	resource.CreateAt = time.Now()
	resource.UpdateAt = time.Now()
	if s.ctx != nil {
		resource.CreateBy = getCurrentNickName(s.ctx)
	}
	err := dao.SysResourceDao.Create(resource)
	return resource, err
}

// Get 获取资源信息
func (s *sysResourceService) Get(req request.GetResourceReq) (*sys.SysResource, error) {
	var resource *sys.SysResource
	var err error
	if req.ID > 0 {
		resource, err = dao.SysResourceDao.One(sys.SysResource{ID: req.ID})
	} else if req.Title != "" {
		resource, err = dao.SysResourceDao.One(sys.SysResource{Title: req.Title})
	}
	return resource, err
}

// Update 更新资源信息
func (s *sysResourceService) Update(req *sys.SysResource) error {
	if req.ID == 0 {
		return errors.New("资源ID不能为空")
	}
	resource, err := s.Get(request.GetResourceReq{ID: req.ID})
	if err != nil {
		logs.Error("获取资源信息失败: {}", err.Error())
		return err
	}
	//检查资源名称是否存在
	if resource.Name != req.Name && dao.SysResourceDao.Exists(sys.SysResource{Name: req.Name}) {
		return errors.New("资源名称已存在")
	}
	//检查资源路径是否存在
	if resource.Path != req.Path && dao.SysResourceDao.Exists(sys.SysResource{Path: req.Path}) {
		return errors.New("资源编码已存在")
	}
	req.UpdateAt = time.Now()
	if s.ctx != nil {
		resource.UpdateBy = getCurrentNickName(s.ctx)
	}
	err = dao.SysResourceDao.Save(req)
	return err
}

// Delete 删除资源
func (s *sysResourceService) Delete(id int64) error {
	return dao.SysResourceDao.Delete(sys.SysResource{ID: uint(id)})
}

// List 获取资源列表
func (s *sysResourceService) List(req request.ListResourceReq) ([]sys.SysResource, *models.ResultPage, error) {
	var mysql = dao.SysResourceDao.Where("del_flag = 0")
	if req.ParentId > 0 {
		mysql = mysql.Where("parent_id = ?", req.ParentId)
	}
	if req.Title != "" {
		mysql = mysql.Where("title like ?", "%"+req.Title+"%")
	}
	if req.Name != "" {
		mysql = mysql.Where("name like?", "%"+req.Name+"%")
	}
	if req.Path != "" {
		mysql = mysql.Where("path like?", "%"+req.Path+"%")
	}
	if req.Component != "" {
		mysql = mysql.Where("component like?", "%"+req.Component+"%")
	}
	if req.Status > 0 {
		mysql = mysql.Where("status =?", req.Status)
	}
	return dao.SysResourceDao.Pager(mysql, req.Page, req.PageSize)
}

// GetTree 获取资源树
func (s *sysResourceService) GetTree(req request.GetTreeResourceReq) ([]sys.SysResource, error) {
	var err error
	var allResources []sys.SysResource
	if req.ByRole { // 查询当前用户所拥有的资源
		roleId := getCurrentRoleId(s.ctx)
		mysql := dao.SysResourceDao.Where("sys_resource.del_flag = 0")
		if req.ParentId > 0 {
			mysql = mysql.Where("sys_resource.parent_id =?", req.ParentId)
		}
		err = mysql.Joins("LEFT JOIN sys_role_resource rm ON rm.resource_id = sys_resource.id").
			Where("rm.role_id =?", roleId).
			Find(&allResources).Error
		if err != nil {
			logs.Error("查询资源列表失败: %v", err)
			return nil, err
		}
	} else {
		// 查询所有未删除的资源
		allResources, _, err = s.List(request.ListResourceReq{ParentId: uint(int(req.ParentId)), Status: 1, Page: 1, PageSize: 10000})
		if err != nil {
			logs.Error("获取资源列表失败: %v", err)
			return nil, err
		}
	}

	// 构建资源树
	return buildResourceTree(allResources, req.ParentId), nil
}

// buildResourceTree 递归构建资源树
func buildResourceTree(resources []sys.SysResource, parentId uint) []sys.SysResource {
	var resourceTree []sys.SysResource
	for _, resource := range resources {
		if resource.ParentId == parentId {
			// 递归查找子资源
			resource.Children = buildResourceTree(resources, resource.ID)
			resourceTree = append(resourceTree, resource)
		}
	}
	return resourceTree
}
