package request

import "time"

type EntryOrdersRequest struct {
	NftListID uint      `json:"nftListId" binding:"required"`
	Seller    string    `json:"seller" binding:"required"`
	Buyer     string    `json:"buyer" binding:"required"`
	TokenId   uint      `json:"tokenId" binding:"required"`
	Price     uint      `json:"price" binding:"required"`
	Deadline  time.Time `json:"deadline" binding:"required"`
	Nonce     int       `json:"nonce" binding:"required"`
}
