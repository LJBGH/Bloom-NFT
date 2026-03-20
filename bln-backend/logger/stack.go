package logger

import (
	"runtime/debug"

	log "github.com/sirupsen/logrus"
)

// ErrorWithStack 记录 error 并附带当前堆栈
func ErrorWithStack(args ...interface{}) {
	log.WithField("stack", string(debug.Stack())).Error(args...)
}

// ErrorWithStackf 带格式的 Error + 堆栈
func ErrorWithStackf(format string, args ...interface{}) {
	log.WithField("stack", string(debug.Stack())).Errorf(format, args...)
}
