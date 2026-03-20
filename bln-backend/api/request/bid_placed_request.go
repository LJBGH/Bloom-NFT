package request

import "time"

type BidPlacedRequest struct {
	OrdersID uint      `json:"ordersId" binding:"required"`
	Buyer    string    `json:"buyer" binding:"required"`
	Price    uint      `json:"price" binding:"required"`
	Deadline time.Time `json:"deadline" binding:"required"`
	Nonce    int       `json:"nonce" binding:"required"`
}
