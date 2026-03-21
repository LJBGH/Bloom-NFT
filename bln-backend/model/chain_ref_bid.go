package model

import "time"

// ChainRefBid 维护 bid_placed 与 bidHash 的映射
type ChainRefBid struct {
	ID         uint      `json:"id" gorm:"primarykey"`
	BidID      uint      `json:"bidId" gorm:"type:int unsigned;not null;index"`
	BidHash    string    `json:"bidHash" gorm:"type:varchar(66);not null;uniqueIndex"`
	CreateTime time.Time `json:"createTime" gorm:"type:datetime;not null"`
	UpdateTime time.Time `json:"updateTime" gorm:"type:datetime;not null"`
}

func (ChainRefBid) TableName() string {
	return "chain_ref_bid"
}
