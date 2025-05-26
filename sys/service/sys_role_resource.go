package service

import (
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/dao"
	"time"
)

type sysRoleResourceService struct {
	ctx *gin.Context
}

func (s *sysRoleResourceService) WithContext(ctx *gin.Context) *sysRoleResourceService {
	s.ctx = ctx
	return s
}

// BindRoleResource 绑定角色和资源,每次都是全量绑定，先删除再新增
func (s *sysRoleResourceService) BindRoleResource(req request.BindRoleResourceReq) error {
	//删除角色的所有资源
	err := dao.SysRoleResourceDao.Delete(sys.SysRoleResource{RoleId: uint(req.RoleId)})
	if err != nil {
		logs.Error("删除角色{}的所有资源时发生错误：{}", req.RoleId, err.Error())
		return err
	}
	var nickName string
	if s.ctx != nil {
		nickName = s.ctx.GetString("nickName")
	}
	for _, resourceId := range req.ResourceIds {
		err := dao.SysRoleResourceDao.Save(&sys.SysRoleResource{RoleId: uint(req.RoleId), ResourceId: uint(resourceId), BaseModel: sys.BaseModel{CreateBy: nickName, UpdateBy: nickName, CreateAt: time.Now(), UpdateAt: time.Now()}})
		if err != nil {
			logs.Error("绑定角色{}和资源 {}时发生错误：{}", req.RoleId, resourceId, err.Error())
		}
	}
	return nil
}

// ListRoleResource 获取角色的资源列表
func (s *sysRoleResourceService) ListRoleResource(req request.ListRoleResourceReq) ([]sys.SysRoleResource, error) {
	list, err := dao.SysRoleResourceDao.All(sys.SysRoleResource{RoleId: uint(req.RoleId)})
	if err != nil {
		logs.Error("获取角色{}的资源列表时发生错误：{}", req.RoleId, err.Error())
		return nil, err
	}
	role, err := dao.SysRoleDao.One(sys.SysRole{ID: uint(req.RoleId)})
	if err != nil {
		logs.Error("获取角色{}的信息时发生错误：{}", req.RoleId, err.Error())
	}
	for i, resource := range list {
		list[i].Resource, err = dao.SysResourceDao.One(sys.SysResource{ID: resource.ResourceId})
		if err != nil {
			logs.Error("获取资源{}的信息时发生错误：{}", resource.ResourceId, err.Error())
		}
		list[i].Role = role
	}
	return list, nil
}
