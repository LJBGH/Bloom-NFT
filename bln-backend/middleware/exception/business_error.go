package middleware

import (
	"bloom-nft/enums"
)

// BusinessError 自定义业务异常
type BusinessError struct {
	ResposeCode enums.ResposeCode
}

func (e *BusinessError) Error() string {
	return e.ResposeCode.GetDesc()
}
