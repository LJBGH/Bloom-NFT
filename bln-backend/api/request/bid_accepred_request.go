package request

type BidAcceptedRequest struct {
	BidID  uint   `json:"bidId" binding:"required"`
	Seller string `json:"seller" binding:"required"` // 必须与挂单卖家一致，用于服务端校验
}
