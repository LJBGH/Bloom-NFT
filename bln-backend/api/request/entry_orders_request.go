package request

import "time"

type EntryOrdersRequest struct {
	NftListID uint      `json:"nftListId" binding:"required"` // NFTId
	Seller    string    `json:"seller" binding:"required"`    // 卖家地址
	TokenId   uint      `json:"tokenId" binding:"required"`   // tokenId
	Price     float64   `json:"price" binding:"required"`     // 价格（wei 字符串）
	Deadline  time.Time `json:"deadline" binding:"required"`  // 截止时间
	Salt      string    `json:"salt" binding:"required"`      // Listing EIP-712 salt（十进制字符串，uint256）
	Signature string    `json:"signature" binding:"required"` // EIP-712 signature
}
