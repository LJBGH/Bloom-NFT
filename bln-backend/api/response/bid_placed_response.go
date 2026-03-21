package response

import (
	"bloom-nft/enums"
	"time"
)

// 出价者信息
type BidPlacedResponse struct {
	ID         uint         `json:"id" gorm:"primarykey"`
	OrdersID   uint         `json:"ordersId" gorm:"type:int;not null"`       // 订单ID
	Buyer      string       `json:"buyer" gorm:"type:varchar(64);not null"`  // 买家地址
	Price      float64      `json:"price" gorm:"type:float;not null"`        // 价格
	Deadline   time.Time    `json:"deadline" gorm:"type:datetime; not null"` // 截止时间
	Nonce      int          `json:"nonce" gorm:"type:int;not null"`          // 非重复 nonce
	Status     enums.Status `json:"status" gorm:"type:int;not null"`         // 状态
	TxHash     string       `json:"txHash" gorm:"type:varchar(256)"`         // 交易哈希
	CreateTime time.Time    `json:"createTime" gorm:"type:datetime; not null"`
	UpdateTime time.Time    `json:"updateTime" gorm:"type:datetime; not null"`
}
