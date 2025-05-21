package service

import (
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/dao"
)

type sysUserExtService struct{}

func (s *sysUserExtService) Save(req request.CreateSysUserExtReq) (*sys.SysUserExt, error) {
	userExt := &sys.SysUserExt{
		UserId:       int64(req.UserId),
		DepartmentId: int64(req.DepartmentId),
		RoleId:       int64(req.RoleId),
		PositionId:   int64(req.PositionId),
	}
	err := dao.SysUserExtDao.Save(userExt)
	if err != nil {
		logs.Error("Save userExt failed: {}", err.Error())
		return nil, err
	}
	return userExt, nil
}

func (s *sysUserExtService) Update(userExt *sys.SysUserExt) error {
	err := dao.SysUserExtDao.Save(userExt)
	if err != nil {
		logs.Error("Update userExt failed: {}", err.Error())
		return err
	}
	return nil
}

func (s *sysUserExtService) Get(userId int64) (*sys.SysUserExt, error) {
	sysUserExt, err := dao.SysUserExtDao.One(sys.SysUserExt{UserId: userId})
	if err != nil {
		logs.Error("Get userExt failed: {}", err.Error())
		return nil, err
	}
	return sysUserExt, nil
}

func (s *sysUserExtService) Delete(userId int64) error {
	err := dao.SysUserExtDao.Delete(sys.SysUserExt{UserId: userId})
	if err != nil {
		logs.Error("Delete userExt failed: {}", err.Error())
		return err
	}
	return nil
}

func (s *sysUserExtService) List(req request.ListSysUserExtReq) ([]sys.SysUserExt, *models.ResultPage, error) {
	var mysql = dao.SysUserExtDao.Where("1 = 1")
	if req.DepartmentId != 0 {
		mysql = dao.SysUserExtDao.Where("department_id =?", req.DepartmentId)
	}
	if req.RoleId != 0 {
		mysql = mysql.Where("role_id =?", req.RoleId)
	}
	if req.PositionId != 0 {
		mysql = mysql.Where("position_id =?", req.PositionId)
	}
	sysUserExt, pages, err := dao.SysUserExtDao.Pager(mysql, req.Page, req.PageSize)
	if err != nil {
		logs.Error("List userExt failed: {}", err.Error())
		return nil, nil, err
	}
	return sysUserExt, pages, nil
}
