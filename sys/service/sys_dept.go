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

type sysDeptService struct {
	ctx *gin.Context
}

func (s *sysDeptService) WithContext(c *gin.Context) *sysDeptService {
	s.ctx = c
	return s
}

// Add 新增部门
func (s *sysDeptService) Add(req request.CreateDeptReq) (*sys.SysDept, error) {
	// 检查部门名称是否存在
	dept, err := s.Get(request.GetDeptReq{Name: req.Name})
	if err != nil {
		return nil, err
	}
	if dept != nil {
		return nil, errors.New("部门名称已存在")
	}

	// 构建并保存部门
	dept = &sys.SysDept{
		ParentId:  req.ParentId,
		Ancestors: req.Ancestors,
		Name:      req.Name,
		Sort:      req.Sort,
		Leader:    req.Leader,
		Mobile:    req.Mobile,
		DeptType:  req.Type,
		Status:    1,
	}
	dept.CreateAt = time.Now()
	dept.UpdateAt = time.Now()
	if s.ctx != nil {
		dept.CreateBy = getCurrentNickName(s.ctx)
	}
	err = dao.SysDeptDao.Save(dept)
	return dept, err
}

// Get 获取部门信息
func (s *sysDeptService) Get(req request.GetDeptReq) (*sys.SysDept, error) {
	var dept *sys.SysDept
	var err error
	if req.ID > 0 {
		dept, err = dao.SysDeptDao.One(sys.SysDept{ID: req.ID})
	} else if req.Name != "" {
		dept, err = dao.SysDeptDao.One(sys.SysDept{Name: req.Name})
	}
	return dept, err
}

// Update 更新部门信息
func (s *sysDeptService) Update(req *sys.SysDept) error {
	dept, err := s.Get(request.GetDeptReq{ID: req.ID})
	if err != nil {
		return err
	}
	if dept == nil {
		dept = &sys.SysDept{ID: req.ID}
	}
	dept.ParentId = req.ParentId
	dept.Ancestors = req.Ancestors
	dept.Name = req.Name
	dept.Sort = req.Sort
	dept.Leader = req.Leader
	dept.Mobile = req.Mobile
	dept.DeptType = req.DeptType
	dept.UpdateAt = time.Now()
	if s.ctx != nil {
		dept.UpdateBy = getCurrentNickName(s.ctx)
	}
	err = dao.SysDeptDao.Save(dept)
	return err
}

// Delete 删除部门
func (s *sysDeptService) Delete(id uint) error {
	// 检查是否存在下级部门
	count, _ := dao.SysDeptDao.Count(sys.SysDept{ParentId: id})
	if count > 0 {
		return errors.New("存在下级部门，无法删除")
	}
	dept, err := s.Get(request.GetDeptReq{ID: id})
	if err != nil {
		return err
	}
	if dept == nil {
		return errors.New("该部门不存在")
	}
	err = dao.SysDeptDao.Delete(*dept)
	return err
}

// List 获取部门列表
func (s *sysDeptService) List(req request.ListDeptReq) ([]sys.SysDept, *models.ResultPage, error) {
	var mysql = dao.SysDeptDao.Where("del_flag = 0")
	if req.ParentId > 0 {
		mysql = dao.SysDeptDao.Where("parent_id =?", req.ParentId)
	}
	return dao.SysDeptDao.Pager(mysql, req.Page, req.PageSize)
}

// GetTree 获取部门树
func (s *sysDeptService) GetTree(id int64) ([]sys.SysDept, error) {
	var depts []sys.SysDept
	// 查询所有状态正常且未删除的部门
	if err := dao.SysDeptDao.Where("status = 1 AND del_flag = 0").Order("sort asc").Find(&depts).Error; err != nil {
		return nil, err
	}

	deptMap := make(map[uint]*sys.SysDept)
	// 构建部门映射表
	for i := range depts {
		deptMap[depts[i].ID] = &depts[i]
	}

	var rootDepts []sys.SysDept
	// 找出根部门
	for i := range depts {
		dept := &depts[i]
		// 如果父部门 ID 为 0 或者指定的 id 等于当前部门 ID，则视为根部门
		if id > 0 {
			if int64(dept.ID) == id {
				rootDepts = append(rootDepts, *buildTree(dept, deptMap))
			} else {
				continue
			}
		} else if dept.ParentId == 0 {
			rootDepts = append(rootDepts, *buildTree(dept, deptMap))
		}
	}

	return rootDepts, nil
}

// buildTree 递归构建部门树
func buildTree(dept *sys.SysDept, deptMap map[uint]*sys.SysDept) *sys.SysDept {
	for _, childDept := range deptMap {
		if childDept.ParentId == dept.ID {
			if dept.Children == nil {
				dept.Children = make([]*sys.SysDept, 0)
			}
			// 递归构建子部门树
			dept.Children = append(dept.Children, buildTree(childDept, deptMap))
		}
	}
	return dept
}

// ... 已有代码 ...
