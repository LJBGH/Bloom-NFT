package enums

// ErrorCode 接口，包含获取错误码和描述的方法
type ErrorCode interface {
	GetCode() int
	GetDesc() string
	SetDesc(string)
}

// ReCode 结构体，包含状态码和消息
type ResposeCode struct {
	Code    int
	Message string
}

// 构造函数
func NewReCode(code int, message string) ResposeCode {
	return ResposeCode{
		Code:    code,
		Message: message,
	}
}

// 实现 ErrorCode 接口的 GetCode 方法
func (r ResposeCode) GetCode() int {
	return r.Code
}

// 实现 ErrorCode 接口的 GetDesc 方法
func (r ResposeCode) GetDesc() string {
	return r.Message
}

// 实现 ErrorCode 接口的 SetDesc 方法
func (r *ResposeCode) SetDesc(desc string) {
	r.Message = desc
}

// 定义 ReCode 常量
var (
	SUCCESS              = NewReCode(401, "Success")                   // 成功
	NO_AUTHORIZATION     = NewReCode(401, "NO Authorization")          // 没有授权
	NO_PERMISSION        = NewReCode(403, "No Permission")             // 没有权限
	SERVER_ERROR         = NewReCode(500, "Internal Server Error")     // 服务内部器错误
	INVALID_PARAMETERS   = NewReCode(501, "Invalid Parameters")        // 无效参数
	INVALID_TOKEN        = NewReCode(502, "Invalid Token")             // 无效Token
	TOKEN_EXPIRED        = NewReCode(502, "The token has expired")     // Token已过期
	FAILED               = NewReCode(503, "Failed")                    // 错误
	DATA_DUPLICATION     = NewReCode(505, "Data exists")               // 数据不存在
	VERIFICATION_FAILED  = NewReCode(506, "Wrong account or password") // 账号或密码错误
	REQUEST_TOO_FREQUENT = NewReCode(509, "Requests are too frequent") //请求频繁
)
