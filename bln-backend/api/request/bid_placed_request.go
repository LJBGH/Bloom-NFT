package request

import "time"

type BidPlacedRequest struct {
	OrdersID  uint      `json:"ordersId" binding:"required"`
	Buyer     string    `json:"buyer" binding:"required"`
	Price     float64   `json:"price" binding:"required"`
	Deadline  time.Time `json:"deadline" binding:"required"`
	Nonce     *int      `json:"nonce" binding:"required,gte=0"`
	Signature string    `json:"signature" binding:"required"` // EIP-712 signature
}
