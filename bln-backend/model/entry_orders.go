package model

import (
	"bloom-nft/enums"
	"time"
)

type EntryOrders struct {
	ID         uint                `json:"id" gorm:"primarykey"`
	NftListID  uint                `json:"nftListId" gorm:"type:int;not null"`                // NFTID
	Seller     string              `json:"seller" gorm:"type:varchar(64);not null"`           // 卖家地址
	Buyer      string              `json:"buyer" gorm:"type:varchar(64)"`                     // 买家地址
	TokenId    uint                `json:"tokenId" gorm:"type:int;not null"`                  // tokenId
	Price      float64             `json:"price" gorm:"type:float;not null"`                  // 价格
	Deadline   time.Time           `json:"deadline" gorm:"type:datetime; not null"`           // 截止时间
	Salt       string              `json:"salt" gorm:"type:varchar(80);not null;default:'0'"` // Listing EIP-712 salt（十进制字符串）
	Status     enums.ListingStatus `json:"status" gorm:"type:int;not null"`                   // 挂单状态
	Signature  string              `json:"signature" gorm:"type:varchar(256);not null"`       // EIP-712 signature（单笔挂单；批量 Merkle 时为批次根签名）
	IsMerkle   bool                `json:"isMerkle" gorm:"column:is_merkle;default:false"`    // 是否通过 Merkle 批量授权上架（购买时无需 listing 单笔签名）
	TxHash     string              `json:"txHash" gorm:"type:varchar(256)"`                   // 交易哈希
	CreateTime time.Time           `json:"createTime" gorm:"type:datetime; not null"`
	UpdateTime time.Time           `json:"updateTime" gorm:"type:datetime; not null"`
}

func (EntryOrders) TableName() string {
	return "entry_orders"
}
