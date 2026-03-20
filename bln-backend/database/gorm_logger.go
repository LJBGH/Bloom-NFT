package database

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	"gorm.io/gorm/logger"
)

// GormLogger 一个简单的 GORM 日志实现
type GormLogger struct {
	logger *log.Logger
}

func NewGormLogger(l *log.Logger) logger.Interface {
	return &GormLogger{logger: l}
}

// LogMode 实现接口，简单返回自身
func (g *GormLogger) LogMode(level logger.LogLevel) logger.Interface {
	return g
}

func (g *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	g.logger.Printf("[INFO] "+msg, data...)
}

func (g *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	g.logger.Printf("[WARN] "+msg, data...)
}

func (g *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	g.logger.Printf("[ERROR] "+msg, data...)
}

func (g *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	sql, rows := fc()
	elapsed := time.Since(begin)
	if err != nil {
		g.logger.Printf("[TRACE][ERR] %s | rows=%d | err=%v | took=%s", sql, rows, err, elapsed)
		return
	}
	g.logger.Printf("[TRACE] %s | rows=%d | took=%s", sql, rows, elapsed)
}
