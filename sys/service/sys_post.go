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

type sysPostService struct {
	ctx *gin.Context
}

// WithContext 注入gin.Context
func (s *sysPostService) WithContext(c *gin.Context) *sysPostService {
	s.ctx = c
	return s
}

// List 获取��位列表

// Add 新增岗位
func (s *sysPostService) Add(req request.CreatePostReq) (*sys.SysPost, error) {
	//检查岗位名称是否存在
	if dao.SysPostDao.Exists(sys.SysPost{PostName: req.PostName}) {
		return nil, errors.New("岗位名称已存在")
	}
	//检查岗位编码是否存在
	if dao.SysPostDao.Exists(sys.SysPost{PostCode: req.PostCode}) {
		return nil, errors.New("岗位编码已存在")
	}
	post := &sys.SysPost{
		PostCode: req.PostCode,
		PostName: req.PostName,
		DeptId:   req.DeptId,
		Status:   1,
	}
	post.CreateAt = time.Now()
	post.UpdateAt = time.Now()
	if s.ctx != nil {
		post.CreateBy = getCurrentNickName(s.ctx)
	}
	err := dao.SysPostDao.Create(post)
	return post, err
}

// Get 获取岗位信息
func (s *sysPostService) Get(req request.GetPostReq) (*sys.SysPost, error) {
	var post *sys.SysPost
	var err error
	if req.Id > 0 {
		post, err = dao.SysPostDao.One(sys.SysPost{Id: req.Id})
	} else if req.PostCode != "" {
		post, err = dao.SysPostDao.One(sys.SysPost{PostCode: req.PostCode})
	} else if req.PostName != "" {
		post, err = dao.SysPostDao.One(sys.SysPost{PostName: req.PostName})
	}
	return post, err
}

// Update 更新岗位信息
func (s *sysPostService) Update(req *sys.SysPost) error {
	if req.Id == 0 {
		return errors.New("岗位ID不能为空")
	}
	post, err := s.Get(request.GetPostReq{Id: req.Id})
	if err != nil {
		logs.Error("获取岗位信息失败: {}", err.Error())
		return err
	}
	if post.PostCode != req.PostCode {
		if dao.SysPostDao.Exists(sys.SysPost{PostCode: req.PostCode}) {
			return errors.New("岗位编码已存在")
		}
	}
	if post.PostName != req.PostName {
		if dao.SysPostDao.Exists(sys.SysPost{PostName: req.PostName}) {
			return errors.New("岗位名称已存在")
		}
	}
	post.PostCode = req.PostCode
	post.PostName = req.PostName
	post.DeptId = req.DeptId
	post.Status = req.Status
	post.UpdateAt = time.Now()
	if s.ctx != nil {
		post.UpdateBy = getCurrentNickName(s.ctx)
	}
	err = dao.SysPostDao.Save(post)
	return err
}

// Delete 删除岗位
func (s *sysPostService) Delete(id uint) error {
	return dao.SysPostDao.Delete(sys.SysPost{Id: int64(id)})
}

// List 获取岗位列表
func (s *sysPostService) List(req request.ListPostReq) ([]sys.SysPost, *models.ResultPage, error) {
	var mysql = dao.SysPostDao.Where("del_flag = 0")
	if req.DeptId > 0 {
		mysql = dao.SysPostDao.Where("dept_id =?", req.DeptId)
	}
	return dao.SysPostDao.Pager(mysql, req.Page, req.PageSize)
}
