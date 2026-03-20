package model

import "time"

type Nft struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	Name        string    `json:"name" gorm:"type:varchar(50);not null"`
	Description string    `json:"description" gorm:"type:varchar(256);not null"`
	Address     string    `json:"address" gorm:"type:varchar(80);not null"`
	CreateTime  time.Time `json:"createTime" gorm:"type:datetime; not null"`
	UpdateTime  time.Time `json:"updateTime" gorm:"type:datetime; not null"`
}

func (Nft) TableName() string {
	return "nft"
}
