package model

import "time"

// ChainEventLog 记录已处理链上事件，便于排错与幂等
type ChainEventLog struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	ChainID     string    `json:"chainId" gorm:"type:varchar(32);not null;index:idx_chain_contract_txlog,unique"`
	Contract    string    `json:"contract" gorm:"type:varchar(64);not null;index:idx_chain_contract_txlog,unique"`
	EventName   string    `json:"eventName" gorm:"type:varchar(64);not null"`
	TxHash      string    `json:"txHash" gorm:"type:varchar(66);not null;index:idx_chain_contract_txlog,unique"`
	LogIndex    uint      `json:"logIndex" gorm:"type:int unsigned;not null;index:idx_chain_contract_txlog,unique"`
	BlockNumber uint64    `json:"blockNumber" gorm:"type:bigint unsigned;not null"`
	Payload     string    `json:"payload" gorm:"type:text"`
	CreateTime  time.Time `json:"createTime" gorm:"type:datetime;not null"`
}

func (ChainEventLog) TableName() string {
	return "chain_event_log"
}
