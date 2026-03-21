package model

import (
	"time"
)

type EntryOrdersEvent struct {
	ID         uint      `json:"id" gorm:"primarykey"`
	EventName  string    `json:"eventName" gorm:"type:varchar(50);not null"`  // 事件名称
	Signature  string    `json:"signature" gorm:"type:varchar(256);not null"` // EIP-712 signature
	Content    string    `json:"content" gorm:"type:varchar(256);not null"`   // 事件内容
	CreateTime time.Time `json:"createTime" gorm:"type:datetime; not null"`
	UpdateTime time.Time `json:"updateTime" gorm:"type:datetime; not null"`
}

func (EntryOrdersEvent) TableName() string {
	return "entry_orders_event"
}
