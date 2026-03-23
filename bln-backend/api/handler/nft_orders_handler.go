package handler

import (
	"bloom-nft/api/request"
	"bloom-nft/enums"
	middleware "bloom-nft/middleware/exception"
	"bloom-nft/model"
	"bloom-nft/services"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type NftOrdersHandler struct {
	NftOrdersService *services.NftOrdersService
}

func NewNftOrdersHandler(nftOrdersService *services.NftOrdersService) *NftOrdersHandler {
	return &NftOrdersHandler{
		NftOrdersService: nftOrdersService,
	}
}

// 挂单
// @Summary      创建挂单
// @Description  创建挂单（entry order）
// @Tags         order
// @Accept       json
// @Produce      json
// @Param        request  body      request.EntryOrdersRequest  true  "挂单参数"
// @Success      200      {object} map[string]interface{}
// @Failure      400      {object} map[string]interface{}
// @Router       /order/entryorders [post]
func (n *NftOrdersHandler) EntryOrders(c *gin.Context) {
	var request request.EntryOrdersRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
	}

	txHash, err := n.NftOrdersService.EntryOrders(&request)
	if err != nil {
		panic(middleware.NewBusinessError(enums.FAILED, err))
	}

	c.JSON(http.StatusOK, model.OkWithData(txHash))
}

// 出价
// @Summary      创建出价
// @Description  对某个挂单订单进行出价（bid placed）
// @Tags         order
// @Accept       json
// @Produce      json
// @Param        request  body      request.BidPlacedRequest  true  "出价参数"
// @Success      200      {object} map[string]interface{}
// @Failure      400      {object} map[string]interface{}
// @Router       /order/bidplaced [post]
func (n *NftOrdersHandler) BidPlaced(c *gin.Context) {
	var request request.BidPlacedRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
	}

	txHash, err := n.NftOrdersService.BidPlaced(&request)
	if err != nil {
		panic(middleware.NewBusinessError(enums.FAILED, err))
	}

	c.JSON(http.StatusOK, model.OkWithData(txHash))
}

// 接受出价
// @Summary      接受出价
// @Description  接受出价
// @Tags         order
// @Accept       json
// @Produce      json
// @Param        request  body      request.BidPlacedRequest  true  "出价参数"
// @Success      200      {object} map[string]interface{}
// @Failure      400      {object} map[string]interface{}
// @Router       /order/bidaccepted [post]
func (n *NftOrdersHandler) BidAccepted(c *gin.Context) {
	var request request.BidAcceptedRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
	}

	txHash, err := n.NftOrdersService.BidAccepted(&request)
	if err != nil {
		panic(middleware.NewBusinessError(enums.FAILED, err))
	}

	c.JSON(http.StatusOK, model.OkWithData(txHash))
}

// 获取挂单列表
// @Summary      获取挂单列表
// @Description  获取挂单列表
// @Tags         order
// @Accept       json
// @Produce      json
// @Param        nftId  query int  false  "NFT 系列Id（可选；不传则返回全部）"
// @Success      200    {object} map[string]interface{}
// @Failure      400    {object} map[string]interface{}
// @Router       /order/orderlist [get]
func (n *NftOrdersHandler) GetEntryOrdersList(c *gin.Context) {
	nftId := c.Param("nftId")
	if nftId == "" {
		// 支持无 path 参数时，用 query 过滤
		nftId = c.Query("nftId")
	}

	var nftIdPtr *uint
	if nftId != "" {
		nftIdParsed, err := strconv.ParseUint(nftId, 10, 0)
		if err != nil {
			panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
		}
		tmp := uint(nftIdParsed)
		nftIdPtr = &tmp
	}

	statusStr := c.Query("status")
	var statusPtr *enums.Status
	if statusStr != "" {
		statusParsed, err := strconv.ParseUint(statusStr, 10, 0)
		if err != nil {
			panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
		}
		tmp := enums.Status(statusParsed)
		if tmp > enums.Refunded {
			panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
		}
		statusPtr = &tmp
	}

	respList, err := n.NftOrdersService.GetEntryOrdersList(nftIdPtr, statusPtr)
	if err != nil {
		panic(middleware.NewBusinessError(enums.FAILED, err))
	}

	c.JSON(http.StatusOK, model.OkWithData(respList))
}

// 获取出价列表
// @Summary      获取出价列表
// @Description  获取某个订单 `ordersId` 下的出价列表
// @Tags         order
// @Accept       json
// @Produce      json
// @Param        ordersId  path  int  true  "订单ID"
// @Success      200       {object} map[string]interface{}
// @Failure      400       {object} map[string]interface{}
// @Router       /order/bidlist/{ordersId} [get]
func (n *NftOrdersHandler) GetBidPlacedList(c *gin.Context) {
	ordersId := c.Param("ordersId")
	ordersIdParsed, err := strconv.ParseUint(ordersId, 10, 0)
	if err != nil {
		panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
	}
	list, err := n.NftOrdersService.GetBidPlacedList(uint(ordersIdParsed))
	if err != nil {
		panic(middleware.NewBusinessError(enums.FAILED, err))
	}

	c.JSON(http.StatusOK, model.OkWithData(list))
}

// GetBidPlacedListForSellerNftList 卖家查询某 NFT 列表项（nft_list_id）对应挂单上的出价列表
// Query: nftListId, seller（必填；后端校验该卖家拥有该 nft_list 下的挂单）
func (n *NftOrdersHandler) GetBidPlacedListForSellerNftList(c *gin.Context) {
	nftListIdStr := c.Query("nftListId")
	seller := c.Query("seller")
	if nftListIdStr == "" || seller == "" {
		panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
	}
	nftListIdParsed, err := strconv.ParseUint(nftListIdStr, 10, 0)
	if err != nil {
		panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
	}

	list, err := n.NftOrdersService.GetBidPlacedListForSellerNftList(uint(nftListIdParsed), seller)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
		}
		panic(middleware.NewBusinessError(enums.FAILED, err))
	}

	c.JSON(http.StatusOK, model.OkWithData(list))
}

// GetMyEntryOrders 当前用户作为卖家的挂单历史（query: seller 必填）
func (n *NftOrdersHandler) GetMyEntryOrders(c *gin.Context) {
	seller := c.Query("seller")
	if seller == "" {
		panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
	}
	list, err := n.NftOrdersService.GetMyEntryOrdersBySeller(seller)
	if err != nil {
		panic(middleware.NewBusinessError(enums.FAILED, err))
	}
	c.JSON(http.StatusOK, model.OkWithData(list))
}

// GetMyBidHistory 当前用户作为买家的出价历史（query: buyer 必填）
func (n *NftOrdersHandler) GetMyBidHistory(c *gin.Context) {
	buyer := c.Query("buyer")
	if buyer == "" {
		panic(&middleware.BusinessError{ResposeCode: enums.INVALID_PARAMETERS})
	}
	list, err := n.NftOrdersService.GetMyBidHistoryByBuyer(buyer)
	if err != nil {
		panic(middleware.NewBusinessError(enums.FAILED, err))
	}
	c.JSON(http.StatusOK, model.OkWithData(list))
}
