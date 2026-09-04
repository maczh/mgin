package casbin

import (
	"errors"
	"fmt"
	"github.com/casbin/casbin/v2"
	"github.com/maczh/mgin/pkg/config"
	"github.com/maczh/mgin/pkg/db"
	"github.com/maczh/mgin/pkg/logs"
	"strconv"
	"sync"

	"github.com/casbin/casbin/v2/model"
	gormAdapter "github.com/casbin/gorm-adapter/v3"
)

type casbinService struct {
	SyncedCachedEnforcer *casbin.SyncedCachedEnforcer
	Once                 sync.Once
	UnAuthPath           []CasbinInfo
}

var Casbin = &casbinService{
	UnAuthPath: make([]CasbinInfo, 0),
}

type CasbinInfo struct {
	Path   string
	Method string
}

// UpdateCasbin
// @author: [Shansec](https://github.com/shansec)
// @function: UpdateCasbin
// @description: 更新 casbin
// @param: RoleID uint, casbinInfos []request.CasbinInfo
// @return: error
func (s *casbinService) UpdateCasbin(RoleID uint, casbinInfos []CasbinInfo) error {
	roleId := fmt.Sprintf("%d", RoleID)
	s.ClearCasbin(0, roleId)
	rules := [][]string{}
	// 权限去重
	deDuplicateMap := make(map[string]bool)
	for _, v := range casbinInfos {
		key := roleId + v.Path + v.Method
		if _, ok := deDuplicateMap[key]; !ok {
			deDuplicateMap[key] = true
			rules = append(rules, []string{roleId, v.Path, v.Method})
		}
	}

	enforcer := s.GetEnforcer()
	success, _ := enforcer.AddPolicies(rules)
	if !success {
		return errors.New("添加失败")
	}
	return nil
}

// UpdateCasbinApi
// @author: [Shansec](https://github.com/shansec)
// @function: UpdateCasbinApi
// @description: api 更新
// @param: oldPath, oldMethod, path, method string
// @return: error
func (s *casbinService) UpdateCasbinApi(oldPath, oldMethod, path, method string) error {
	conn, err := db.Mysql.GetConnection()
	if err != nil {
		logs.Error("获取数据库连接失败:{}", err.Error())
		return err
	}
	err = conn.Model(&gormAdapter.CasbinRule{}).Where("v1 = ? AND v2 = ?", oldPath, oldMethod).Updates(map[string]interface{}{
		"v1": path,
		"v2": method,
	}).Error
	enforcer := s.GetEnforcer()
	err = enforcer.LoadPolicy()
	if err != nil {
		return err
	}
	return err
}

// GetPolicyPathByRoleId
// @author: [Shansec](https://github.com/shansec)
// @function: GetPolicyPathByRoleId
// @description: 获取 casbin 列表
// @param: RoleId uint
// @return: pathMap []request.CasbinInfo
func (s *casbinService) GetPolicyPathByRoleId(RoleId uint) (pathMap []CasbinInfo, err error) {
	enforcer := s.GetEnforcer()
	roleId := strconv.Itoa(int(RoleId))
	// 处理可能出现的错误
	policies, err := enforcer.GetFilteredPolicy(0, roleId)
	if err != nil {
		return nil, err
	}
	for _, policy := range policies {
		// 检查 policy 长度是否足够
		if len(policy) >= 3 {
			pathMap = append(pathMap, CasbinInfo{
				Path:   policy[1],
				Method: policy[2],
			})
		}
	}
	return pathMap, nil
}

// ClearCasbin
// @author: [Shansec](https://github.com/shansec)
// @function: ClearCasbin
// @description: 清除 casbin
// @param: v int, p ...string
// @return: bool
func (s *casbinService) ClearCasbin(v int, p ...string) bool {
	enforcer := s.GetEnforcer()
	result, err := enforcer.RemoveFilteredPolicy(v, p...)
	if err != nil {
		logs.Error("清除失败:{}", err.Error())
		return false
	}
	return result
}

// RemoveFilteredPolicy
// @author: [Shansec](https://github.com/shansec)
// @function: RemoveFilteredPolicy
// @description: 清除指定的 casbin
// @param: db *gorm.DB, roleId string
// @return: error
func (s *casbinService) RemoveFilteredPolicy(roleId string) error {
	conn, err := db.Mysql.GetConnection()
	if err != nil {
		logs.Error("获取数据库连接失败:{}", err.Error())
		return err
	}
	return conn.Delete(&gormAdapter.CasbinRule{}, "v0 = ?", roleId).Error
}

func (s *casbinService) SyncPolicy(roleId string, rule [][]string) error {
	err := s.RemoveFilteredPolicy(roleId)
	if err != nil {
		return err
	}
	return s.AddPolicy(rule)
}

func (s *casbinService) AddPolicy(rules [][]string) error {
	var casbinRules []gormAdapter.CasbinRule
	for i := range rules {
		casbinRules = append(casbinRules, gormAdapter.CasbinRule{
			Ptype: "p",
			V0:    rules[i][0],
			V1:    rules[i][1],
			V2:    rules[i][2],
		})
	}
	conn, err := db.Mysql.GetConnection()
	if err != nil {
		logs.Error("获取数据库连接失败:{}", err.Error())
		return err
	}
	return conn.Create(&casbinRules).Error
}

func (s *casbinService) FreshCasbin() (err error) {
	e := s.GetEnforcer()
	err = e.LoadPolicy()
	return err
}

func (s *casbinService) GetEnforcer() *casbin.SyncedCachedEnforcer {
	s.Once.Do(func() {
		conn, err := db.Mysql.GetConnection()
		if err != nil {
			logs.Error("获取数据库连接失败!", err)
			return
		}
		a, err := gormAdapter.NewAdapterByDB(conn)
		if err != nil {
			logs.Error("适配数据库失败请检查casbin表是否为InnoDB引擎!", err)
			return
		}
		modelFile := config.Config.Casbin.ModelFile
		if modelFile == "" {
			modelFile = "rbac_model.conf"
		}
		m, err := model.NewModelFromFile(modelFile)
		if err != nil {
			logs.Error("加载casbin模型失败!{}", err.Error())
			return
		}
		s.SyncedCachedEnforcer, _ = casbin.NewSyncedCachedEnforcer(m, a)
		s.SyncedCachedEnforcer.SetExpireTime(60 * 60)
		_ = s.SyncedCachedEnforcer.LoadPolicy()
	})
	return s.SyncedCachedEnforcer
}
