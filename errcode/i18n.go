package errcode

const (
	UrlNotFound        = "404"
	SystemError        = "系统异常"
	DbConnectErr       = "数据库连接失败"
	DbInsertErr        = "数据库插入失败"
	DbUpdateErr        = "数据库更新失败"
	DbDeleteErr        = "数据库删除失败"
	DataNotFound       = "数据库查无数据"
	ParamLost          = "参数不可为空"
	ParamError         = "参数错误"
	ConnectFail        = "网络连接失败"
	ServiceUnavailable = "服务不存在"
	Success            = "success"
	DbQueryErr         = "数据库查询失败"
)
