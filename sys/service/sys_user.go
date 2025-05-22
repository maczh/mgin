package service

import (
	"errors"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/maczh/mgin/db"
	"github.com/maczh/mgin/logs"
	"github.com/maczh/mgin/models"
	"github.com/maczh/mgin/models/sys"
	"github.com/maczh/mgin/models/sys/request"
	"github.com/maczh/mgin/sys/dao"
	"github.com/maczh/mgin/utils"
	"gorm.io/gorm"
	"time"
)

type sysUserService struct {
	ctx *gin.Context
}

func (s *sysUserService) WithContext(c *gin.Context) *sysUserService {
	s.ctx = c
	return s
}

// Register 用户注册
func (s *sysUserService) Register(req request.RegisterReq) (*sys.SysUser, error) {
	user := sys.SysUser{
		LoginName: req.LoginName,
		Password:  utils.MD5Encode(req.Password),
		Email:     req.Email,
		Mobile:    req.Mobile,
		NickName:  req.NickName,
		Status:    1,
		Sex:       req.Sex,
		Avatar:    req.Avatar,
	}
	//检查用户名是否已存在
	if dao.SysUserDao.Exists(sys.SysUser{LoginName: req.LoginName}) {
		return nil, errors.New("用户名已存在")
	}
	//检查邮箱是否已存在
	if req.Email != "" && dao.SysUserDao.Exists(sys.SysUser{Email: req.Email}) {
		return nil, errors.New("邮箱已存在")
	}
	//检查手机号是否已存在
	if req.Mobile != "" && dao.SysUserDao.Exists(sys.SysUser{Mobile: req.Mobile}) {
		return nil, errors.New("手机号已存在")
	}
	// 创建用户
	user.CreateAt = time.Now()
	user.UpdateAt = time.Now()
	if s.ctx != nil {
		user.CreateBy = getCurrentNickName(s.ctx)
	}
	err := dao.SysUserDao.Create(&user)
	if err != nil {
		logs.Error("注册失败: {}", err.Error())
		return nil, err
	}
	//重新获取用户信息
	user1, err := dao.SysUserDao.One(sys.SysUser{LoginName: req.LoginName})
	if err != nil {
		logs.Error("注册失败: {}", err.Error())
		return nil, err
	}
	// 自动创建扩展信息
	userExt := sys.SysUserExt{
		UserId:       user1.Id,
		RoleId:       2, // 默认角色ID
		DepartmentId: 1, // 默认部门ID
		PositionId:   1, // 默认岗位ID
	}
	err = dao.SysUserExtDao.Save(&userExt)
	if err != nil {
		logs.Error("生成用户扩展属性记录失败: {}", err.Error())
		return nil, err
	}
	user1.Password = "******"
	return user1, nil
}

// Login 用户登录
func (s *sysUserService) Login(req request.LoginReq) (string, error) {
	redis, err := db.Redis.GetConnection()
	if err != nil {
		logs.Error("Redis连接失败: {}", err.Error())
		return "", err
	}
	// 先从缓存中查找
	v := redis.Get(fmt.Sprintf("sys:user:%s", req.LoginName)).Val() //获取到userId
	var user sys.SysUser
	if v != "" {
		u := redis.Get(fmt.Sprintf("sys:user:id:%s", v)).Val() //获取到用户信息
		utils.FromJSON(u, &user)
	} else {
		err = dao.SysUserDao.Where("(login_name = ? OR email = ? OR mobile = ?) AND del_flag = 0", req.LoginName, req.LoginName, req.LoginName).First(&user).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return "", errors.New("用户不存在")
			}
			return "", err
		}
		if user.Id == 0 {
			return "", errors.New("用户不存在")
		}
		if user.Status != 1 {
			return "", errors.New("账号已被禁用")
		}
		//将用户信息存入缓存
		redis.Set(fmt.Sprintf("sys:user:%s", req.LoginName), utils.ToJSON(user), 0)
	}
	if user.Password != utils.MD5Encode(req.Password) {
		return "", errors.New("密码错误")
	}

	// 更新登录信息
	now := time.Now()
	user.LoginIp = s.ctx.RemoteIP() // 这里需要根据实际情况获取登录 IP
	user.LoginDate = &now
	err = dao.SysUserDao.Updates(&user)
	if err != nil {
		logs.Error("登录失败: {}", err.Error())
		return "", err
	}

	// 获取角色信息
	var roleId int64 = 0
	ext, err := dao.SysUserExtDao.One(sys.SysUserExt{UserId: user.Id})
	if ext != nil {
		roleId = ext.RoleId
	}
	// 生成JWT token
	claims := jwt.MapClaims{
		"id":        user.Id,
		"loginName": user.LoginName,
		"nickName":  user.NickName,
		"avatar":    user.Avatar,
		"roleId":    roleId,
		"exp":       time.Now().Add(time.Hour * 24).Unix(),
	}
	token, err := utils.GenerateToken(claims)
	if err != nil {
		logs.Error("生成Token失败: {}", err.Error())
		return "", err
	}
	redis.Set(fmt.Sprintf("sys:user:token:%d", user.Id), token, 7*24*time.Hour)
	return token, nil
}

// Logout 用户退出登录
func (s *sysUserService) Logout() error {
	claims := s.ctx.MustGet("claims").(jwt.MapClaims)
	userId := uint(claims["id"].(float64))
	user, err := s.GetSysUser(request.GetSysUserReq{Id: uint64(userId)})
	if err != nil {
		return err
	}
	redis, err := db.Redis.GetConnection()
	if err != nil {
		logs.Error("Redis连接失败: {}", err.Error())
		return err
	}
	//删除token缓存
	redis.Del(fmt.Sprintf("sys:user:token:%d", user.Id))
	return nil
}

// VerifyToken 验证 Token
func (s *sysUserService) VerifyJwt(jwtToken string) (bool, *jwt.MapClaims, error) {
	token, err := utils.ValidateToken(jwtToken)
	if err != nil || !token.Valid {
		return false, nil, err
	}
	claims, _ := token.Claims.(jwt.MapClaims)
	userId := uint(claims["id"].(float64))
	sysUser, err := s.GetSysUser(request.GetSysUserReq{Id: uint64(userId)})
	if err != nil {
		return false, &claims, err
	}
	if sysUser.Status != 1 {
		return false, &claims, errors.New("账号已被禁用")
	}
	redis, err := db.Redis.GetConnection()
	if err != nil {
		logs.Error("获取 Redis 连接失败: {}", err.Error())
		return false, &claims, err
	}
	tokenStr := redis.Get(fmt.Sprintf("sys:user:token:%d", sysUser.Id)).Val()
	if tokenStr != jwtToken {
		return false, &claims, errors.New("Token 无效")
	}
	return true, &claims, nil
}

// GetSysUser 获取用户信息
func (s *sysUserService) GetSysUser(req request.GetSysUserReq) (*sys.SysUser, error) {
	redis, err := db.Redis.GetConnection()
	if err != nil {
		logs.Error("获取 Redis 连接失败: {}", err.Error())
		return nil, err
	}
	// 先从缓存中查找
	var user sys.SysUser
	if req.Id == 0 {
		userStr := redis.Get(fmt.Sprintf("sys:user:id:%d", req.Id)).Val()
		if userStr != "" {
			utils.FromJSON(userStr, &user)
		}
	} else {
		userId := int64(0)
		if req.LoginName != "" {
			userId, _ = redis.Get(fmt.Sprintf("sys:user:%s", req.LoginName)).Int64()
		} else if req.Email != "" {
			userId, _ = redis.Get(fmt.Sprintf("sys:user:%s", req.Email)).Int64()
		} else if req.Mobile != "" {
			userId, _ = redis.Get(fmt.Sprintf("sys:user:%s", req.Mobile)).Int64()
		}
		if userId != 0 {
			userStr := redis.Get(fmt.Sprintf("sys:user:id:%d", userId)).Val()
			if userStr != "" {
				utils.FromJSON(userStr, &user)
			}
		}
	}
	if user.Id == 0 { //缓存中没有，从数据库中查找
		err = dao.SysUserDao.Where("(id =? or login_name = ? or email = ? or mobile = ?) AND del_flag = 0", req.Id, req.LoginName, req.Email, req.Mobile).First(&user).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("用户不存在")
			}
		}
		if user.Id != 0 { //将用户信息存入缓存
			redis.Set(fmt.Sprintf("sys:user:id:%d", user.Id), utils.ToJSON(user), 0)
			redis.Set(fmt.Sprintf("sys:user:%s", user.LoginName), user.Id, 0)
			if user.Email != "" {
				redis.Set(fmt.Sprintf("sys:user:%s", user.Email), user.Id, 0)
			}
			if user.Mobile != "" {
				redis.Set(fmt.Sprintf("sys:user:%s", user.Mobile), user.Id, 0)
			}
		}
	}
	return &user, nil
}

// ListSysUser 列出用户列表
func (s *sysUserService) ListSysUser(req request.ListSysUserReq) ([]sys.SysUser, *models.ResultPage, error) {
	var query = dao.SysUserDao.Where("del_flag = 0")
	if req.Keyword != "" {
		query = dao.SysUserDao.Where("login_name LIKE ? OR nick_name LIKE ? OR email LIKE ? OR mobile LIKE ?", "%"+req.Keyword+"%", "%"+req.Keyword+"%", "%"+req.Keyword+"%", "%"+req.Keyword+"%")
	}
	if req.Status != 0 {
		query = query.Where("status = ?", req.Status)
	}
	if req.UserType != "" {
		query = query.Where("user_type = ?", req.UserType)
	}
	// 这里假设需要关联 sys_user_ext 表查询部门、角色、岗位信息
	if req.DeptId != 0 || req.RoleId != 0 || req.PostId != 0 {
		query = query.Joins("LEFT JOIN sys_user_ext ON sys_user.id = sys_user_ext.user_id")
		if req.DeptId != 0 {
			query = query.Where("sys_user_ext.department_id = ?", req.DeptId)
		}
		if req.RoleId != 0 {
			query = query.Where("sys_user_ext.role_id = ?", req.RoleId)
		}
		if req.PostId != 0 {
			query = query.Where("sys_user_ext.position_id = ?", req.PostId)
		}
	}
	sysUsers, pages, err := dao.SysUserDao.Pager(query, req.Page, req.PageSize)
	if err != nil {
		logs.Error("查询用户列表失败: {}", err.Error())
		return nil, nil, err
	}
	return sysUsers, pages, nil
}

// Update 更新用户信息
func (s *sysUserService) Update(user *sys.SysUser) error {
	// 检查用户是否存在
	u, _ := dao.SysUserDao.One(sys.SysUser{Id: user.Id})
	if u == nil {
		return errors.New("用户不存在")
	}
	if u.LoginName != user.LoginName {
		return errors.New("登录名不能修改")
	}
	if (u.Email != user.Email) && dao.SysUserDao.Exists(sys.SysUser{Email: user.Email}) {
		return errors.New("邮箱地址已经被使用")
	}
	if (u.Mobile != user.Mobile) && dao.SysUserDao.Exists(sys.SysUser{Mobile: user.Mobile}) {
		return errors.New("手机号码已经被使用")
	}
	user.Password = u.Password
	user.CreateBy = u.CreateBy
	if s.ctx != nil {
		user.UpdateBy = getCurrentNickName(s.ctx)
	}
	user.UpdateAt = time.Now()
	err := dao.SysUserDao.Save(user)
	if err != nil {
		logs.Error("更新用户信息失败: {}", err.Error())
		return err
	}

	// 更新成功后删除缓存
	redis, err := db.Redis.GetConnection()
	if err != nil {
		logs.Error("获取 Redis 连接失败: {}", err.Error())
		return err
	}
	redis.Del(fmt.Sprintf("sys:user:id:%d", user.Id))
	redis.Del(fmt.Sprintf("sys:user:%s", user.LoginName))
	if user.Email != "" {
		redis.Del(fmt.Sprintf("sys:user:%s", user.Email))
	}
	if user.Mobile != "" {
		redis.Del(fmt.Sprintf("sys:user:%s", user.Mobile))
	}
	return nil
}

// ChangePassword 修改密码
func (s *sysUserService) ChangePassword(req request.ChangePasswordReq) error {
	user, err := s.GetSysUser(request.GetSysUserReq{LoginName: req.LoginName})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("用户不存在")
		}
		return err
	}
	if user == nil {
		return errors.New("用户不存在")
	}

	if user.Password != utils.MD5Encode(req.OldPassword) {
		return errors.New("旧密码错误")
	}

	user.Password = utils.MD5Encode(req.NewPassword)
	if s.ctx != nil {
		user.UpdateBy = getCurrentNickName(s.ctx)
	}
	return dao.SysUserDao.Updates(user)
}

// New 管理员创建新用户
func (s *sysUserService) New(user *sys.SysUser) (*sys.SysUser, error) {
	_, exists := s.ctx.Get("claims")
	if !exists {
		return nil, errors.New("未登录")
	}
	return s.Register(request.RegisterReq{
		LoginName: user.LoginName,
		Password:  user.Password,
		NickName:  user.NickName,
		Email:     user.Email,
		Mobile:    user.Mobile,
		Avatar:    user.Avatar,
		Sex:       user.Sex,
	})
}

// Delete 软删除用户
func (s *sysUserService) Delete(id int64) error {
	user, err := s.GetSysUser(request.GetSysUserReq{Id: uint64(id)})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("用户不存在")
		}
		return err
	}
	if user == nil {
		return errors.New("用户不存在")
	}

	// 软删除，将状态设置为停用
	user.DelFlag = 1
	if s.ctx != nil {
		user.UpdateBy = getCurrentNickName(s.ctx)
	}
	err = dao.SysUserDao.Updates(user)
	if err != nil {
		return err
	}

	// 更新成功后删除缓存
	redis, err := db.Redis.GetConnection()
	if err != nil {
		logs.Error("获取 Redis 连接失败: {}", err.Error())
		return err
	}
	redis.Del(fmt.Sprintf("sys:user:id:%d", user.Id))
	return nil
}

// ChangeStatus 更改用户状态
func (s *sysUserService) ChangeStatus(id int64, status uint8) error {
	user, err := s.GetSysUser(request.GetSysUserReq{Id: uint64(id)})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("用户不存在")
		}
		return err
	}
	if user == nil {
		return errors.New("用户不存在")
	}

	user.Status = status
	if s.ctx != nil {
		user.UpdateBy = getCurrentNickName(s.ctx)
	}
	err = dao.SysUserDao.Save(user)
	if err != nil {
		return err
	}

	// 更新成功后删除缓存
	redis, err := db.Redis.GetConnection()
	if err != nil {
		logs.Error("获取 Redis 连接失败: {}", err.Error())
		return err
	}
	redis.Del(fmt.Sprintf("sys:user:id:%d", user.Id))

	return nil
}
