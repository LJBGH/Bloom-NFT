package logger

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// 全局变量，用于存储日志文件和日期
var (
	logFile *os.File
	logDate string
	logMu   sync.Mutex
)

// getLogWriter 返回当天的日志文件，跨天时自动切换到新文件
func getLogWriter() io.Writer {
	// 加锁，防止多个goroutine同时写入日志文件
	logMu.Lock()
	defer logMu.Unlock()
	today := time.Now().Format("2006-01-02")
	if logFile != nil && logDate == today {
		return logFile
	}
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
	// 创建日志目录
	if err := os.MkdirAll("logs", 0755); err != nil {
		return os.Stdout
	}
	// 创建日志文件路径
	path := filepath.Join("logs", today+".log")
	// 打开日志文件
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return os.Stdout
	}
	logFile = f
	logDate = today
	return f
}

// dayWriter 实现 io.Writer，每次写入时检查日期，跨天则切文件
type dayWriter struct{}

func (d dayWriter) Write(p []byte) (n int, err error) {
	return getLogWriter().Write(p)
}

// GetOutputWriter 返回当前应使用的日志 Writer（与 logrus 一致：生产仅文件，开发为控制台+文件）
// 用于将 Gin 等框架的默认输出也写入按天切分的 logs 文件
func GetOutputWriter() io.Writer {
	env := os.Getenv("GO_ENV")
	fileWriter := dayWriter{}
	if env == "production" {
		return fileWriter
	}
	return io.MultiWriter(os.Stdout, fileWriter)
}

// InitLogger 初始化日志记录器，按天写入 logs 目录
func InitLogger() {
	env := os.Getenv("GO_ENV")

	fileWriter := dayWriter{}
	if env == "production" {
		log.SetLevel(log.InfoLevel)
		log.SetOutput(fileWriter)
	} else {
		log.SetLevel(log.DebugLevel)
		log.SetOutput(io.MultiWriter(os.Stdout, fileWriter))
	}
}
