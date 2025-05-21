package vo

import "github.com/maczh/mgin/models/sys"

type ListSysApiByGroupResp struct {
	Group string       `json:"group"`
	Apis  []sys.SysApi `json:"apis"`
}
