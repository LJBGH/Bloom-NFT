package request

type BidAcceptedRequest struct {
	BidID uint `json:"bidId" binding:"required"`
}
