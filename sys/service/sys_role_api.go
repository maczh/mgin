package service

import (
	"fmt"
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/dao"
)

type sysRoleApiService struct{}

// BindRoleApi 绑定角色和API,每次都是全量绑定，先删除再新增
func (s *sysRoleApiService) BindRoleApi(req request.BindRoleApiReq) error {
	//删除角色的所有API
	err := dao.SysRoleApiDao.Delete(sys.SysRoleApi{RoleId: uint(req.RoleId)})
	if err != nil {
		logs.Error("删除角色{}的所有API时发生错误：{}", req.RoleId, err.Error())
		return err
	}
	for _, apiId := range req.ApiIds {
		err := dao.SysRoleApiDao.Save(&sys.SysRoleApi{RoleId: uint(req.RoleId), ApiId: uint(apiId)})
		logs.Error("绑定角色{}和API {}时发生错误：{}", req.RoleId, apiId, err.Error())
	}
	redis, err := db.Redis.GetConnection()
	cacheKey := fmt.Sprintf("sys:role:api:%d", req.RoleId)
	if err == nil {
		redis.Del(cacheKey)
	}
	return nil
}

// ListRoleApi 获取角色的API列表
func (s *sysRoleApiService) ListRoleApi(req request.ListRoleApiReq) ([]sys.SysRoleApi, error) {
	list, err := dao.SysRoleApiDao.All(sys.SysRoleApi{RoleId: uint(req.RoleId)})
	if err != nil {
		logs.Error("获取角色{}的API列表时发生错误：{}", req.RoleId, err.Error())
		return nil, err
	}
	role, err := dao.SysRoleDao.One(sys.SysRole{ID: uint(req.RoleId)})
	if err != nil {
		logs.Error("获取角色{}的信息时发生错误：{}", req.RoleId, err.Error())
	}
	for i, api := range list {
		list[i].Api, err = dao.SysApiDao.One(sys.SysApi{ID: api.ApiId})
		if err != nil {
			logs.Error("获取API{}的信息时发生错误：{}", api.ApiId, err.Error())
		}
		list[i].Role = role
	}
	return list, nil
}

func (s *sysRoleApiService) matchRoleApiPath(roleId int64, path string) bool {
	//先从缓存中获取
	redis, err := db.Redis.GetConnection()
	cacheKey := fmt.Sprintf("sys:role:api:%d", roleId)
	if err == nil {
		if exists, _ := redis.Exists(cacheKey).Result(); exists > 0 {
			return redis.SIsMember(cacheKey, path).Val()
		} else {
			//从数据库中获取
			var apiPaths []string
			err = dao.SysRoleApiDao.Where(sys.SysRoleApi{RoleId: uint(roleId)}).Table("sys_role_api").Select("sys_api.api_path").Joins("LEFT JOIN sys_api ON sys_role_api.api_id = sys_api.id").Find(&apiPaths).Error
			if err != nil {
				logs.Error("获取角色{}的API列表时发生错误：{}", roleId, err.Error())
				return false
			}
			//存入缓存
			redis.SAdd(cacheKey, apiPaths)
			return redis.SIsMember(cacheKey, path).Val()
		}
	} else {
		//从数据库中获取
		sysApi, err := dao.SysApiDao.One(sys.SysApi{APIPath: path})
		if err != nil {
			logs.Error("获取API{}的信息时发生错误：{}", path, err.Error())
			return false
		}
		return sysApi != nil && sysApi.ID > 0 && dao.SysRoleApiDao.Exists(sys.SysRoleApi{RoleId: uint(roleId), ApiId: sysApi.ID})
	}
}

func (s sysRoleApiService) HasApiPermission(userId int64, apiPath string) (bool, error) {
	userExt, err := dao.SysUserExtDao.One(sys.SysUserExt{UserId: userId})
	if err != nil {
		logs.Error("获取用户{}的信息时发生错误：{}", userId, err.Error())
		return false, err
	}
	if userExt == nil {
		logs.Error("该用户角色未绑定")
		return false, nil
	}
	if userExt.RoleId == 0 {
		logs.Error("该用户角色未绑定")
		return false, nil
	}
	return s.matchRoleApiPath(userExt.RoleId, apiPath), nil
}
