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

		api.POST("/nft/mint", nftHander.Mint)                     // 铸造 NFT
		api.POST("/nft/update", nftHander.UpdateNftList)          // 更新 NFT
		api.GET("/nft/all", nftHander.AllNft)                     // 获取所有 NFT 系列
		api.GET("/nft/listall/:nftId", nftHander.AllNftList)      // 获取指定 NFT 系列下的所有 NFT
		api.GET("/nft/user/categories", nftHander.UserCategories) // 获取用户拥有的 NFT 类目列表
		api.GET("/nft/user/list/:nftId", nftHander.UserNftList)   // 获取用户在该 NFT 类目下的所有 NFT 列表

		api.GET("/order/orderlist/:nftId", nftOrdersHandler.GetOrdersEntryList)                 //挂单列表
		api.GET("/order/orderlist", nftOrdersHandler.GetOrdersEntryList)                        //挂单列表（nftId 可选）
		api.GET("/order/bidlist-for-seller", nftOrdersHandler.GetOrdersBidListForSellerNftList) // 卖家按 nft_list_id 查出价
		api.GET("/order/my-entryorders", nftOrdersHandler.GetMyOrdersEntry)                     // 我的挂单历史
		api.GET("/order/my-bids", nftOrdersHandler.GetMyOrdersBidHistory)                             // 我的出价历史
		api.GET("/order/bidlist/:ordersId", nftOrdersHandler.GetOrdersBidList)                  //出价列表
		api.POST("/order/entryorders", nftOrdersHandler.OrdersEntry)                            // 挂单
		api.POST("/order/entryorders/batch", nftOrdersHandler.OrdersEntryBatch)                 // Merkle 批量挂单
		api.POST("/order/bidplaced", nftOrdersHandler.OrdersBid)                                // 出价
		api.POST("/order/bidaccepted", nftOrdersHandler.OrdersBidAccepted)                            // 接受出价
	}
	return r
}
