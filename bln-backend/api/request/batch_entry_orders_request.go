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
	Proof     []string  `json:"proof" binding:"required"` // 32 字节 hex 数组，与 Merkle 树一致
}

// BatchEntryOrdersRequest Merkle 根 + 根截止时间 + 批次签名 + 各叶子证明。
type BatchEntryOrdersRequest struct {
	RootDeadline   time.Time `json:"rootDeadline" binding:"required"`
	MerkleRoot     string    `json:"merkleRoot" binding:"required"`
	BatchSignature string    `json:"batchSignature" binding:"required"`
	Items          []BatchEntryItem `json:"items" binding:"required,min=1"`
}
