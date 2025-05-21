package service

import (
	"errors"
	"fmt"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/dao"
	"strings"
	"time"
)

type sysDeptService struct{}

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
	err = dao.SysDeptDao.Save(dept)
	return dept, err
}

// Get 获取部门信息
func (s *sysDeptService) Get(req request.GetDeptReq) (*sys.SysDept, error) {
	var dept *sys.SysDept
	var err error
	if req.Id > 0 {
		dept, err = dao.SysDeptDao.One(sys.SysDept{Id: req.Id})
	} else if req.Name != "" {
		dept, err = dao.SysDeptDao.One(sys.SysDept{Name: req.Name})
	}
	return dept, err
}

// Update 更新部门信息
func (s *sysDeptService) Update(req *sys.SysDept) error {
	dept, err := s.Get(request.GetDeptReq{Id: req.Id})
	if err != nil {
		return err
	}
	if dept == nil {
		dept = &sys.SysDept{Id: req.Id}
	}
	dept.ParentId = req.ParentId
	dept.Ancestors = req.Ancestors
	dept.Name = req.Name
	dept.Sort = req.Sort
	dept.Leader = req.Leader
	dept.Mobile = req.Mobile
	dept.DeptType = req.DeptType
	dept.UpdateAt = time.Now()
	err = dao.SysDeptDao.Save(dept)
	return err
}

// Delete 删除部门
func (s *sysDeptService) Delete(id uint) error {
	dept, err := s.Get(request.GetDeptReq{Id: id})
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
	var rootDepts []sys.SysDept
	deptMap := make(map[uint]*sys.SysDept)

	// 查询所有部门
	if err := dao.SysDeptDao.Where("status = 1 AND del_flag = 0").Order("sort asc").Find(&depts).Error; err != nil {
		return nil, err
	}

	// 构建部门映射表
	for i := range depts {
		deptMap[depts[i].Id] = &depts[i]
	}

	// 构建树结构
	for i := range depts {
		dept := &depts[i]
		parentId := dept.ParentId

		// 如果父部门不存在或为自身，则视为根部门
		if parentId == 0 || parentId == dept.Id || deptMap[parentId] == nil {
			if id == 0 || int64(dept.Id) == id {
				rootDepts = append(rootDepts, *dept)
			}
			continue
		}

		// 将当前部门添加到父部门的子节点列表
		parent := deptMap[parentId]
		if parent.Children == nil {
			parent.Children = make([]*sys.SysDept, 0)
		}
		parent.Children = append(parent.Children, dept)
	}

	// 如果指定了id且未找到对应部门树，则尝试查找以该id为祖先的部门树
	if id != 0 && len(rootDepts) == 0 {
		for i := range depts {
			dept := &depts[i]
			if strings.Contains(dept.Ancestors, fmt.Sprintf(",%d,", id)) {
				rootDepts = append(rootDepts, *dept)
				break
			}
		}
	}

	return rootDepts, nil
}
