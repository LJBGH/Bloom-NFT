package model

import "time"

// ChainRefEntryOrder 维护 entry_orders 与 listingHash 的映射
type ChainRefEntryOrder struct {
	ID           uint      `json:"id" gorm:"primarykey"`
	EntryOrderID uint      `json:"entryOrderId" gorm:"type:int unsigned;not null;index"`
	ListingHash  string    `json:"listingHash" gorm:"type:varchar(66);not null;uniqueIndex"`
	CreateTime   time.Time `json:"createTime" gorm:"type:datetime;not null"`
	UpdateTime   time.Time `json:"updateTime" gorm:"type:datetime;not null"`
}

func (ChainRefEntryOrder) TableName() string {
	return "chain_ref_entry_order"
}
