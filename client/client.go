package client

import (
	"github.com/levigross/grequests"
	"github.com/maczh/mgin/config"
	"github.com/maczh/mgin/middleware/trace"
	"github.com/maczh/mgin/models"
)

type Options struct {
	Method   string                 `json:"method"`   //接口方法 GET|POST|PUT|DELETE
	Protocol string                 `json:"protocol"` //协议 x-form|json|restful
	Group    string                 `json:"group"`    //应用分组，用于nacos中分组，不传为当前nacos分组及默认分组
	Header   map[string]string      `json:"header"`   //额外的头部参数
	Query    map[string]string      `json:"query"`    //URL Query参数
	Data     map[string]string      `json:"data"`     //x-form Postform参数
	Json     any                    `json:"json"`     //json或restful模式的body参数
	Path     map[string]string      `json:"path"`     //restful模式的路径参数
	Files    []grequests.FileUpload //文件上传数据
	Retry    bool                   `json:"retry"` //是否重试
}

func Call[T any](service, uri string, op Options) (models.Result[T], error) {
	if op.Protocol == "" {
		op.Protocol = config.Config.Discovery.CallType
	}
	headers := trace.GetHeaders()
	if op.Header != nil && len(op.Header) > 0 {
		for k, v := range op.Header {
			headers[k] = v
		}
	}
}
