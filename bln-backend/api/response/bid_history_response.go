package response

import (
	"bloom-nft/enums"
	"time"
)

// BidHistoryResponse 个人中心「我的出价」列表项
type BidHistoryResponse struct {
	ID          uint         `json:"id"`
	OrdersID    uint         `json:"ordersId"`
	Buyer       string       `json:"buyer"`
	Price       float64      `json:"price"`
	Deadline    time.Time    `json:"deadline"`
	Nonce       int          `json:"nonce"`
	Status      enums.Status `json:"status"`
	StatusDesc  string       `json:"statusDesc"`
	Signature   string       `json:"signature"`
	TxHash      string       `json:"txHash"`
	CreateTime  time.Time    `json:"createTime"`
	UpdateTime  time.Time    `json:"updateTime"`
	NftListID   uint         `json:"nftListId"`
	TokenId     uint         `json:"tokenId"`
	EntrySeller string       `json:"entrySeller"`
	ImageUrl    string       `json:"imageUrl"`
}
