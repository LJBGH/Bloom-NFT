package response

import (
	"bloom-nft/enums"
	"time"
)

// 挂单信息
type EntryOrdersResponse struct {
	ID         uint         `json:"id" gorm:"primarykey"`
	NftListID  uint         `json:"nftListId" gorm:"type:int;not null"`      // NFTID
	Seller     string       `json:"seller" gorm:"type:varchar(64);not null"` // 卖家地址
	Buyer      string       `json:"buyer" gorm:"type:varchar(64)"`           // 买家地址
	TokenId    uint         `json:"tokenId" gorm:"type:int;not null"`        // tokenId
	Price      float64      `json:"price" gorm:"type:float;not null"`        // 价格
	Deadline   time.Time    `json:"deadline" gorm:"type:datetime; not null"` // 截止时间
	Nonce      int          `json:"nonce" gorm:"type:int;not null"`          // 非重复 nonce
	Status     enums.Status `json:"status" gorm:"type:int;not null"`         // 状态
	StatusDesc string       `json:"statusDesc"`                              // 状态说明
	TxHash     string       `json:"txHash" gorm:"type:varchar(256)"`         // 交易哈希
	Signature  string       `json:"signature" gorm:"type:varchar(256)"`      // listing 签名（用于前端直连 buy）
	CreateTime time.Time    `json:"createTime" gorm:"type:datetime; not null"`
	UpdateTime time.Time    `json:"updateTime" gorm:"type:datetime; not null"`
	ImageUrl   string       `json:"imageUrl" gorm:"type:varchar(512);not null"` // NFT图片URL
}
