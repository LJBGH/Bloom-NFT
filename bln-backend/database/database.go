package database

import (
	"bloom-nft/logger"
	"fmt"
	"time"

	"bloom-nft/config"

	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ProvideDB() *gorm.DB {
	return InitializeDB("")
}

// Init 初始化数据库连接（MySQL + GORM）
// dsn 示例: "user:password@tcp(127.0.0.1:3306)/bloom-nft?charset=utf8mb4&parseTime=True&loc=Local"
func InitializeDB(dsn string) *gorm.DB {
	if dsn == "" {
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			config.AppConfig.Database.User,
			config.AppConfig.Database.Password,
			config.AppConfig.Database.Host,
			config.AppConfig.Database.Port,
			config.AppConfig.Database.Name)
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})

	if err != nil {
		logger.ErrorWithStackf("failed to connect database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.ErrorWithStackf("failed to get generic DB: %v", err)
	}

	// 简单连接池设置，可根据需要调整
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	DB = db
	log.Infof("database initialized successfully")
	return db
}

// AutoMigrate 对传入的模型执行自动迁移
func AutoMigrate() {
	if DB == nil {
		logger.ErrorWithStack("database is not initialized")
		return
	}
	if !config.AppConfig.Database.IsMigrate {
		log.Infof("skip auto migrate: ISMIGRATE is false")
		return
	}
	models := MigrateModels()
	err := DB.AutoMigrate(models...)
	if err != nil {
		logger.ErrorWithStackf("failed to auto migrate: %v", err)
		return
	}

	log.Infof("auto migrate success")
}
