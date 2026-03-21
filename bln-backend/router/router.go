package router

import (
	"bloom-nft/api/handler"
	docs "bloom-nft/docs"

	"bloom-nft/middleware"
	exception "bloom-nft/middleware/exception"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// SetupRouter 初始化所有路由
func SetupRouter(
	testHandler *handler.TestHandler,
	loginHandler *handler.LoginHandler,
	walletHandler *handler.WalletHandler,
	nftHander *handler.NftHandler,
	nftOrdersHandler *handler.NftOrdersHandler) *gin.Engine {
	r := gin.New()

	// 使用自定义的 Recovery 中间件
	r.Use(exception.CustomRecovery())

	// 记录接口请求日志
	r.Use(gin.Logger())

	// 全局使用跨域中间件
	r.Use(middleware.CORSMiddleware())

	// 配置 Swagger 基础路径
	docs.SwaggerInfo.BasePath = "/api"

	// Swagger 文档路由
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 业务路由
	// 业务路由加统一前缀
	api := r.Group("/api")
	{
		api.GET("/test/ping", testHandler.Ping)
		api.GET("/test/time", testHandler.GetTime)

		api.POST("/auth/login", loginHandler.Login)

		// 钱包
		api.GET("/wallet/create/mnemonic", walletHandler.CreateMnemonic)
		api.GET("/wallet/create/wallet", walletHandler.CreateWallet)
		api.POST("/wallet/import", walletHandler.Import)
		api.GET("/wallet/backup", walletHandler.Backup)
		api.GET("/wallet/restore", walletHandler.Restore)
		api.GET("/wallet/restore/privateKey", walletHandler.RestoreWithPrivateKey)
		api.GET("/wallet/restore/mnemonic", walletHandler.RestoreWithMnemonic)
		api.GET("/wallet/account/current", walletHandler.CurrentAccount)
		api.GET("/wallet/account/all", walletHandler.AllAccounts)
		api.GET("/wallet/account/switch", walletHandler.SwitchCurrentAccount)

		api.POST("/nft/mint", nftHander.Mint)
		api.POST("/nft/update", nftHander.UpdateNftList)
		api.GET("/nft/all", nftHander.AllNft)
		api.GET("/nft/listall/:nftId", nftHander.AllNftList)
		api.GET("/nft/user/categories", nftHander.UserCategories)
		api.GET("/nft/user/list/:nftId", nftHander.UserNftList)

		api.GET("/order/orderlist/:nftId", nftOrdersHandler.GetEntryOrdersList) //挂单列表
		api.GET("/order/orderlist", nftOrdersHandler.GetEntryOrdersList)        //挂单列表（nftId 可选）
		api.GET("/order/bidlist/:ordersId", nftOrdersHandler.GetBidPlacedList)  //出价列表
		api.POST("/order/entryorders", nftOrdersHandler.EntryOrders)            // 挂单
		api.POST("/order/bidplaced", nftOrdersHandler.BidPlaced)                // 出价
		api.POST("/order/bidaccepted", nftOrdersHandler.BidAccepted)            // 接受出价
	}
	return r
}
