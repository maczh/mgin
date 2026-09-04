package vo

type ListRoleApiResp struct {
	RoleId   int64             `json:"roleId"`
	RoleName string            `json:"roleName"`
	Apis     []ListRoleApiItem `json:"apis"`
}

type ListRoleApiItem struct {
	ApiId   int64  `json:"apiId"`
	ApiPath string `json:"apiPath"`
	Name    string `json:"name"`
	Method  string `json:"method"`
	Group   string `json:"group"`
}
