package main

import (
	"context"
	"fmt"
	"log"

	"bloom-nft/config"
	"bloom-nft/database"
	"bloom-nft/listener"
	"bloom-nft/logger"

	wire "bloom-nft/wire"

	"github.com/gin-gonic/gin"
)

// @title           bloom-nft
// @version         1.0
// @description     bloom-nft 接口文档 认证方式：在 Authorize 中填写 "Bearer {token}"
// @host            localhost:8081
// @BasePath        /
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
func main() {
	// 加载配置
	config.LoadConfig()
	// 初始化日志
	logger.InitLogger()
	// 将 Gin 的默认输出（[GIN-debug] 路由注册、请求日志等）也写入 logs 文件
	gin.DefaultWriter = logger.GetOutputWriter()

	// 先通过 Wire 初始化应用（这里面会调用 ProvideDB -> InitializeDB，给 DB 赋值）
	r, err := wire.InitializeApp()
	if err != nil {
		log.Fatalf("failed to initialize app: %v", err)
		panic(err)
	}

	// 然后再做自动迁移，这时 DB 已经不是 nil 了
	database.AutoMigrate()

	// 启动链上事件监听（后台运行，不阻塞 API）
	go func() {
		if err := listener.StartMarketplaceListener(context.Background()); err != nil {
			log.Printf("start marketplace listener failed: %v", err)
		}
	}()

	addr := fmt.Sprintf(":%s", config.AppConfig.Port)
	log.Printf("swagger is running on http://localhost%s/swagger/index.html", addr)
	r.Run(addr)
}
