package model

import "time"

//
type NftList struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	NftID       uint      `json:"nftId" gorm:"type:int;not null"`
	Name        string    `json:"name" gorm:"type:varchar(50);not null"`
	Description string    `json:"description" gorm:"type:varchar(256);not null"`
	ImageUrl    string    `json:"imageUrl" gorm:"type:varchar(512);not null"`
	MetadataUrl string    `json:"metadataUrl" gorm:"type:varchar(512);not null"`
	TokenUrl    string    `json:"tokenUrl" gorm:"type:varchar(64);not null"`
	TokenId     uint      `json:"tokenId" gorm:"type:int;not null"`
	Owner       string    `json:"owner" gorm:"type:varchar(64);not null"`
	CreateTime  time.Time `json:"createTime" gorm:"type:datetime; not null"`
	UpdateTime  time.Time `json:"updateTime" gorm:"type:datetime; not null"`
}

func (NftList) TableName() string {
	return "nft_list"
}
