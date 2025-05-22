package service

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/dao"
	"time"
)

type sysRoleService struct {
	ctx *gin.Context
}

// WithContext 注入gin.Context
func (s *sysRoleService) WithContext(c *gin.Context) *sysRoleService {
	s.ctx = c
	return s
}

// Add 新增角色
func (s *sysRoleService) Add(req request.CreateRoleReq) (*sys.SysRole, error) {
	if dao.SysRoleDao.Exists(sys.SysRole{RoleName: req.Name}) {
		return nil, errors.New("角色名称已存在")
	}
	if dao.SysRoleDao.Exists(sys.SysRole{RoleIdent: req.Ident}) {
		return nil, errors.New("角色标识符已存在")
	}
	role := &sys.SysRole{
		RoleName:    req.Name,
		RoleIdent:   req.Ident,
		IsEnable:    1,
		Description: req.Description,
	}
	role.CreateAt = time.Now()
	role.UpdateAt = time.Now()
	if s.ctx != nil {
		role.CreateBy = getCurrentNickName(s.ctx)
	}
	err := dao.SysRoleDao.Create(role)
	if err != nil {
		return nil, err
	}
	return role, nil
}

// Get 获取角色
func (s *sysRoleService) Get(req request.GetRoleReq) (*sys.SysRole, error) {
	if req.Id > 0 {
		return dao.SysRoleDao.One(sys.SysRole{ID: req.Id})
	} else if req.Ident != "" {
		return dao.SysRoleDao.One(sys.SysRole{RoleIdent: req.Ident})
	} else if req.Name != "" {
		return dao.SysRoleDao.One(sys.SysRole{RoleName: req.Name})
	}
	return nil, errors.New("参数不能为空")
}

// Update 更新角色
func (s *sysRoleService) Update(req *sys.SysRole) error {
	if req.ID <= 0 {
		return errors.New("角色ID参数不能为空")
	}
	role, err := s.Get(request.GetRoleReq{Id: req.ID})
	if err != nil {
		return err
	}
	if role == nil {
		return errors.New("角色不存在")
	}
	role.RoleName = req.RoleName
	role.RoleIdent = req.RoleIdent
	role.IsEnable = req.IsEnable
	role.Description = req.Description
	role.UpdateAt = time.Now()
	if s.ctx != nil {
		role.UpdateBy = getCurrentNickName(s.ctx)
	}
	err = dao.SysRoleDao.Save(role)
	if err != nil {
		return err
	}
	return nil
}

// Delete 删除角色
func (s *sysRoleService) Delete(id uint) error {
	role, err := s.Get(request.GetRoleReq{Id: id})
	if err != nil {
		return err
	}
	if role == nil {
		return errors.New("角色不存在")
	}
	err = dao.SysRoleDao.Delete(*role)
	if err != nil {
		return err
	}
	return nil
}

// List 获取角色列表
func (s *sysRoleService) List(req request.ListRoleReq) ([]sys.SysRole, *models.ResultPage, error) {
	mysql := dao.SysRoleDao.Where("del_flag = 0")
	return dao.SysRoleDao.Pager(mysql, req.Page, req.PageSize)
}
