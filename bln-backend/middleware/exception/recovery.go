package middleware

import (
	"net/http"
	"runtime/debug"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
)

// CustomRecovery 中间件，用于捕获 panic 并返回自定义的错误信息
func CustomRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				var response ErrorResponse
				// 检查错误类型
				switch e := err.(type) {
				case *BusinessError:
					stack := trimStack(string(debug.Stack()), 30)
					// 业务异常仍返回业务码，但日志附带底层错误与堆栈方便定位
					if e.Cause != nil {
						log.WithFields(log.Fields{
							"code":    e.ResposeCode.GetCode(),
							"message": e.ResposeCode.GetDesc(),
							"cause":   e.Cause.Error(),
						}).Error("catch business exception")
					} else {
						log.WithFields(log.Fields{
							"code":    e.ResposeCode.GetCode(),
							"message": e.ResposeCode.GetDesc(),
						}).Error("catch business exception")
					}
					log.Errorf("stack trace:\n%s", stack)
					// 处理业务异常
					response = NewErrorResponse(e.ResposeCode.GetCode(), e.ResposeCode.GetDesc())
					c.JSON(e.ResposeCode.GetCode(), response)
				default:
					stack := trimStack(string(debug.Stack()), 30)
					log.WithField("error", err).Error("catch panic exception")
					log.Errorf("stack trace:\n%s", stack)
					// 处理系统异常
					response = NewErrorResponse(http.StatusInternalServerError, "Internal Server Error")
					c.JSON(http.StatusInternalServerError, response)
				}
			}
		}()

		// 继续处理请求
		c.Next()
	}
}

func trimStack(stack string, maxLines int) string {
	if maxLines <= 0 {
		return stack
	}
	lines := strings.Split(stack, "\n")
	if len(lines) <= maxLines {
		return stack
	}
	return strings.Join(lines[:maxLines], "\n") + "\n... (stack truncated)"
}
