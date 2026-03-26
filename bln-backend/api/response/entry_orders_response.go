package response

import (
	"bloom-nft/enums"
	"time"
)

// 挂单信息
type EntryOrdersResponse struct {
	ID         uint         `json:"id" gorm:"primarykey"`
	NftListID  uint         `json:"nftListId" gorm:"type:int;not null"`      // NFTID
	NftAddress string       `json:"nftAddress"`                              // NFT 合约地址（来自 nft.address）
	Seller     string       `json:"seller" gorm:"type:varchar(64);not null"` // 卖家地址
	Buyer      string       `json:"buyer" gorm:"type:varchar(64)"`           // 买家地址
	TokenId    uint         `json:"tokenId" gorm:"type:int;not null"`        // tokenId
	Price      float64      `json:"price" gorm:"type:float;not null"`        // 价格
	PriceWei   string       `json:"priceWei"`                               // price 转成 wei（uint256，十进制字符串）
	Deadline   time.Time    `json:"deadline" gorm:"type:datetime; not null"` // 截止时间
	Salt       string       `json:"salt" gorm:"type:varchar(80);not null"`   // Listing salt（十进制）
	Status     enums.ListingStatus `json:"status" gorm:"type:int;not null"` // 挂单状态
	StatusDesc string       `json:"statusDesc"`                              // 状态说明
	TxHash     string       `json:"txHash" gorm:"type:varchar(256)"`         // 交易哈希
	Signature  string       `json:"signature" gorm:"type:varchar(256)"`      // listing 签名（用于前端直连 buy）
	IsMerkle   bool         `json:"isMerkle"`                                // Merkle 批量上架：购买时无需 listing 单笔签名

	// Merkle 信息：仅当 IsMerkle=true 时参与合约购买（buy / buyBatch 的 merkle 路径）。
	MerkleRoot      string   `json:"merkleRoot" gorm:"type:varchar(66)"`           // bytes32 hex
	RootDeadlineSec uint64   `json:"rootDeadlineSec" gorm:"type:bigint unsigned"` // uint256 unix seconds
	MerkleProof     []string `json:"merkleProof" gorm:"-"`                       // []bytes32 hex

	CreateTime time.Time    `json:"createTime" gorm:"type:datetime; not null"`
	UpdateTime time.Time    `json:"updateTime" gorm:"type:datetime; not null"`
	ImageUrl   string       `json:"imageUrl" gorm:"type:varchar(512);not null"` // NFT图片URL
	// ListingHash 是合约的 EIP-712 digest（由 entry_orders 的 listing 参数确定；在创建时写入）
	ListingHash string `json:"listingHash"`
}
