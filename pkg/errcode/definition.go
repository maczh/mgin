package errcode

// Module 业务模块常量：用于错误码分组与文档归类。
// v2 新增。v1 时代的错误码常量（URI_NOT_FOUND=1000 等）不重新分配模块，保持兼容。
const (
	ModuleCommon   = "common"
	ModuleAuth     = "auth"
	ModuleUser     = "user"
	ModuleDatabase = "database"
	ModuleJob      = "job"
	ModuleStorage  = "storage"
	ModuleClient   = "client"
	ModulePlugin   = "plugin"
)

// Definition v2 新增：一个标准化的"业务错误定义"。
//
// 字段含义：
//   - Code:       业务错误码（int），与 v1 常量同空间（1000+，与 HTTP status 错开）
//   - HTTPStatus: HTTP 状态码（int）。v1 时代所有错误都 200，v2 起按定义走正确状态码
//   - MessageKey: i18n 翻译键（如 "errcode.uri_not_found"），用于 ErrorDef 自动取文案
//   - Module:     业务模块，便于按模块聚合
//   - Args:       可选，预留默认占位参数
type Definition struct {
	Code       int
	HTTPStatus int
	MessageKey string
	Module     string
}

// New 构造一个 Definition。HTTPStatus 必须为 0 或一个合法 HTTP 状态码，
// 非法值会被自动修正为 500（服务端错误），避免误返 2xx 给客户端。
func New(module string, code, httpStatus int, messageKey string) Definition {
	if httpStatus == 0 {
		httpStatus = 500
	}
	if httpStatus < 100 || httpStatus >= 600 {
		httpStatus = 500
	}
	return Definition{
		Code:       code,
		HTTPStatus: httpStatus,
		MessageKey: messageKey,
		Module:     module,
	}
}

// 预置 v1 错误码的 v2 Definition 映射。
// 业务侧仍可用 URI_NOT_FOUND=1000 这样的旧常量；通过这些 Definition，
// v2 框架（NoRoute、Recovery、i18n.ErrorDef）可以自动用上 HTTPStatus。
var definitions = map[int]Definition{
	URI_NOT_FOUND:          New(ModuleCommon, URI_NOT_FOUND, 404, "errcode.uri_not_found"),
	SYSTEM_ERROR:           New(ModuleCommon, SYSTEM_ERROR, 500, "errcode.system_error"),
	DB_CONNECT_ERROR:       New(ModuleDatabase, DB_CONNECT_ERROR, 503, "errcode.db_connect_error"),
	REQUEST_PARAMETER_LOST: New(ModuleCommon, REQUEST_PARAMETER_LOST, 400, "errcode.request_parameter_lost"),
	DATA_NOT_FOUND:         New(ModuleCommon, DATA_NOT_FOUND, 404, "errcode.data_not_found"),
	USER_NOT_FOUND:         New(ModuleAuth, USER_NOT_FOUND, 401, "errcode.user_not_found"),
	PASSWORD_ERROR:         New(ModuleAuth, PASSWORD_ERROR, 401, "errcode.password_error"),
	VERIFY_CODE_ERROR:      New(ModuleAuth, VERIFY_CODE_ERROR, 400, "errcode.verify_code_error"),
	TOKEN_ERROR:            New(ModuleAuth, TOKEN_ERROR, 401, "errcode.token_error"),
	AUTHENTICATION_FAILURE: New(ModuleAuth, AUTHENTICATION_FAILURE, 401, "errcode.authentication_failure"),
	SERVICE_UNAVAILABLE:    New(ModuleCommon, SERVICE_UNAVAILABLE, 503, "errcode.service_unavailable"),
	PARAM_ERROR:            New(ModuleCommon, PARAM_ERROR, 400, "errcode.param_error"),
	TOO_MANY_REQUESTS:      New(ModuleCommon, TOO_MANY_REQUESTS, 429, "errcode.too_many_requests"),
	JOB_ERROR:              New(ModuleJob, JOB_ERROR, 500, "errcode.job_error"),
	STORAGE_ERROR:          New(ModuleStorage, STORAGE_ERROR, 500, "errcode.storage_error"),
}

// LookupDef 按 v1 错误码常量查 v2 Definition。未注册则返 500 + SYSTEM_ERROR。
func LookupDef(code int) Definition {
	if d, ok := definitions[code]; ok {
		return d
	}
	return New(ModuleCommon, code, 500, "errcode.unknown")
}

// RegisterDef 把自定义 Definition 注册到全局映射（业务侧可在 init() 中调用）。
// 同 code 后注册会覆盖前者。
func RegisterDef(d Definition) {
	definitions[d.Code] = d
}

// AllDefs 导出全部已注册 Definition，主要给 docs / OpenAPI 生成器使用。
func AllDefs() map[int]Definition {
	out := make(map[int]Definition, len(definitions))
	for k, v := range definitions {
		out[k] = v
	}
	return out
}
