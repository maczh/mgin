package service

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

var (
	Captcha     = &captchaService{}
	SysUser     = &sysUserService{}
	SysApi      = &sysApiService{}
	SysUserExt  = &sysUserExtService{}
	SysDict     = &sysDictService{}
	SysDept     = &sysDeptService{}
	SysRole     = &sysRoleService{}
	SysPost     = &sysPostService{}
	SysMenu     = &sysMenuService{}
	SysRoleApi  = &sysRoleApiService{}
	SysRoleMenu = &sysRoleMenuService{}
	SysConfig   = &sysConfigService{}
)

// 从*gin.Context 根据jwt提取用户昵称
func getCurrentNickName(c *gin.Context) string {
	claims := c.MustGet("claims").(jwt.MapClaims)
	return claims["nickName"].(string)
}

// 从*gin.Context 获取用户id
func getCurrentUserId(c *gin.Context) uint {
	claims := c.MustGet("claims").(jwt.MapClaims)
	return uint(claims["userId"].(float64))
}

// 从*gin.Context 获取用户角色id
func getCurrentRoleId(c *gin.Context) uint {
	claims := c.MustGet("claims").(jwt.MapClaims)
	return uint(claims["roleId"].(float64))
}
