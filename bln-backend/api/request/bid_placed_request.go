package request

import "time"

type BidPlacedRequest struct {
	OrdersID  uint      `json:"ordersId" binding:"required"`
	Buyer     string    `json:"buyer" binding:"required"`
	Price     float64   `json:"price" binding:"required"`
	Deadline  time.Time `json:"deadline" binding:"required"`
	Salt      string    `json:"salt" binding:"required"` // Bid salt（十进制或 0x 十六进制，uint256）
	Signature string    `json:"signature" binding:"required"` // EIP-712 signature
}
