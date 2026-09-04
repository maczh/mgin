package errcode

const (
	URI_NOT_FOUND          = 1000
	SYSTEM_ERROR           = 1001
	DB_CONNECT_ERROR       = 1002
	REQUEST_PARAMETER_LOST = 1003
	DATA_NOT_FOUND         = 1004
	USER_NOT_FOUND         = 1005
	PASSWORD_ERROR         = 1006
	VERIFY_CODE_ERROR      = 1007
	TOKEN_ERROR            = 1008
	AUTHENTICATION_FAILURE = 1009
	SERVICE_UNAVAILABLE    = 1010
	PARAM_ERROR            = 1014 //参数校验错误
	TOO_MANY_REQUESTS      = 1011 //触发限流
	JOB_ERROR              = 1012 //定时任务执行异常
	STORAGE_ERROR          = 1013 //对象存储操作异常
)
