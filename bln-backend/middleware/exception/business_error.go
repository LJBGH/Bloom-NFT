package middleware

import (
	"bloom-nft/enums"
)

// BusinessError 自定义业务异常
type BusinessError struct {
	ResposeCode enums.ResposeCode
	Cause       error
}

func (e *BusinessError) Error() string {
	return e.ResposeCode.GetDesc()
}

// NewBusinessError 创建业务异常，并可附带底层错误原因用于日志排查
func NewBusinessError(code enums.ResposeCode, cause error) *BusinessError {
	return &BusinessError{
		ResposeCode: code,
		Cause:       cause,
	}
}
