package model

import (
	"bloom-nft/enums"
	"time"
)

type BidPlaced struct {
	ID         uint            `json:"id" gorm:"primarykey"`
	OrdersID   uint            `json:"ordersId" gorm:"type:int;not null"`                    // 订单ID
	Buyer      string          `json:"buyer" gorm:"type:varchar(64);not null"`               // 买家地址
	Price      float64         `json:"price" gorm:"type:float;not null"`                     // 价格
	Deadline   time.Time       `json:"deadline" gorm:"type:datetime; not null"`              // 截止时间
	Salt       string          `json:"salt" gorm:"type:varchar(80);not null"`                // Bid salt（十进制或 0x 十六进制）
	Status     enums.BidStatus `json:"status" gorm:"type:int;not null"`                      // 出价状态
	Signature  string          `json:"signature" gorm:"type:varchar(256);not null"`          // EIP-712 signature
	BidHash    string          `json:"bidHash" gorm:"type:varchar(66);not null;uniqueIndex"` // 链上 bidHash
	TxHash     string          `json:"txHash" gorm:"type:varchar(256)"`                      // 交易哈希
	CreateTime time.Time       `json:"createTime" gorm:"type:datetime; not null"`
	UpdateTime time.Time       `json:"updateTime" gorm:"type:datetime; not null"`
}

func (BidPlaced) TableName() string {
	return "orders_bid"
}
