package repository

import (
	"bloom-nft/model"

	"gorm.io/gorm"
)

type NftOrdersRepository struct {
	DB *gorm.DB
}

// EntryOrdersWithImageUrl 用于关联查询：entry_orders + nft_list.image_url
// 该结构只用于查询结果，不参与自动建表。
type EntryOrdersWithImageUrl struct {
	model.EntryOrders
	ImageUrl string `gorm:"column:image_url"`
}

func NewNftOrdersRepository(db *gorm.DB) *NftOrdersRepository {
	return &NftOrdersRepository{DB: db}
}

// 插入挂单
func (n *NftOrdersRepository) InsertEntryOrders(entryOrders model.EntryOrders) error {
	return n.DB.Create(&entryOrders).Error
}

// 插入出价
func (n *NftOrdersRepository) InsertBidPlaced(bidPlaced model.BidPlaced) error {
	return n.DB.Create(&bidPlaced).Error
}

// 获取挂单列表
func (n *NftOrdersRepository) GetEntryOrdersList(nftId *uint) ([]EntryOrdersWithImageUrl, error) {
	var entryOrders []EntryOrdersWithImageUrl
	if nftId != nil {
		// nftId 对应的是 entry_orders.nft_list_id
		if err := n.DB.
			Table("entry_orders").
			Select("entry_orders.*, nft_list.image_url AS image_url").
			Joins("LEFT JOIN nft_list ON nft_list.id = entry_orders.nft_list_id").
			Where("entry_orders.nft_list_id = ?", nftId).
			Order("entry_orders.create_time DESC").
			Find(&entryOrders).Error; err != nil {
			return nil, err
		}

	} else {
		if err := n.DB.
			Table("entry_orders").
			Select("entry_orders.*, nft_list.image_url AS image_url").
			Joins("LEFT JOIN nft_list ON nft_list.id = entry_orders.nft_list_id").
			Order("entry_orders.create_time DESC").
			Find(&entryOrders).Error; err != nil {
			return nil, err
		}
	}

	return entryOrders, nil
}

// 获取出价列表
func (n *NftOrdersRepository) GetBidPlacedList(ordersId uint) ([]model.BidPlaced, error) {
	var bidPlaced []model.BidPlaced
	if err := n.DB.Where("orders_id = ?", ordersId).Find(&bidPlaced).Order("create_time DESC").Error; err != nil {
		return nil, err
	}
	return bidPlaced, nil
}
