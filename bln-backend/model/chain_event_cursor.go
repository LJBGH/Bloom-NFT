package model

import "time"

// ChainEventCursor 记录监听处理进度
type ChainEventCursor struct {
	ID         uint      `json:"id" gorm:"primarykey"`
	ChainID    string    `json:"chainId" gorm:"type:varchar(32);not null;index:idx_chain_contract,unique"`
	Contract   string    `json:"contract" gorm:"type:varchar(64);not null;index:idx_chain_contract,unique"`
	LastBlock  uint64    `json:"lastBlock" gorm:"type:bigint unsigned;not null;default:0"`
	CreateTime time.Time `json:"createTime" gorm:"type:datetime;not null"`
	UpdateTime time.Time `json:"updateTime" gorm:"type:datetime;not null"`
}

func (ChainEventCursor) TableName() string {
	return "chain_event_cursor"
}
