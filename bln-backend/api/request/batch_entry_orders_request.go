package request

import "time"

// BatchEntryItem 批量 Merkle 上架中的单笔（nonce 固定为 0，由链下 salt 保证唯一）。
type BatchEntryItem struct {
	NftListID uint      `json:"nftListId" binding:"required"`
	Seller    string    `json:"seller" binding:"required"`
	TokenId   uint      `json:"tokenId" binding:"required"`
	Price     float64   `json:"price" binding:"required"`
	Deadline  time.Time `json:"deadline" binding:"required"`
	Salt      string    `json:"salt" binding:"required"`
	Signature string    `json:"signature" binding:"omitempty"` // 单笔挂单签名
	Proof     []string  `json:"proof" binding:"omitempty"`     // 32 字节 hex 数组，与 Merkle 树一致
}

// BatchEntryOrdersRequest Merkle 根 + 根截止时间 + 批次签名 + 各叶子证明。
type BatchEntryOrdersRequest struct {
	RootDeadline   time.Time        `json:"rootDeadline" binding:"required"`   // 根截止时间
	MerkleRoot     string           `json:"merkleRoot" binding:"required"`     // 根哈希
	BatchSignature string           `json:"batchSignature" binding:"required"` // 批次根签名
	Items          []BatchEntryItem `json:"items" binding:"required,min=1"`    // 批量上架订单
}
