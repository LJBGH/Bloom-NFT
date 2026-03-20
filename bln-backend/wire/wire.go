//go:build wireinject
// +build wireinject

package wire

import (
	"bloom-nft/api/handler"
	"bloom-nft/database"
	"bloom-nft/repository"
	"bloom-nft/router"
	"bloom-nft/services"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
)

// InitializeApp 由 Wire 生成实现，返回组装好的 *gin.Engine（具体实现在 wire_gen.go）
func InitializeApp() (*gin.Engine, error) {
	wire.Build(
		// Database 层
		database.ProvideDB,

		// Repository 层 依赖 Database
		repository.NewNftRepository,
		repository.NewNftListRepository,
		repository.NewNftOrdersRepository,

		// Service 层 依赖 Repository 层
		services.NewTestService,
		services.NewLoginService,
		services.NewWalletService,
		services.NewNftService,
		services.NewNftOrdersService,

		// Handler 层（依赖上面的 Service，Wire 会自动注入）
		handler.NewTestHandler,
		handler.NewLoginHandler,
		handler.NewWalletHandler,
		handler.NewNftHandler,
		handler.NewNftOrdersHandler,

		// 组装路由
		router.SetupRouter,
	)
	return nil, nil
}
