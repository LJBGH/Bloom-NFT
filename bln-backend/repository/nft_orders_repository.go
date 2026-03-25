package repository

import (
	"bloom-nft/enums"
	"bloom-nft/model"
	"errors"
	"time"

	"gorm.io/gorm"
)

type NftOrdersRepository struct {
	DB *gorm.DB
}

// EntryOrdersWithImageUrl 用于关联查询：entry_orders + nft_list.image_url
// 该结构只用于查询结果，不参与自动建表。
type EntryOrdersWithImageUrl struct {
	model.EntryOrders
	ImageUrl    string `gorm:"column:image_url"`
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
func (n *NftOrdersRepository) GetEntryOrdersList(nftId *uint, status *enums.ListingStatus) ([]EntryOrdersWithImageUrl, error) {
	var entryOrders []EntryOrdersWithImageUrl
	q := n.DB.
		Table("orders_entry").
		Select("orders_entry.*, nft_list.image_url AS image_url").
		Joins("LEFT JOIN nft_list ON nft_list.id = orders_entry.nft_list_id")

	if nftId != nil {
		// nftId 对应的是 orders_entry.nft_list_id
		q = q.Where("orders_entry.nft_list_id = ?", *nftId)
	}
	if status != nil {
		q = q.Where("orders_entry.status = ?", *status)
	}

	if err := q.
		Order("orders_entry.create_time DESC").
		Find(&entryOrders).Error; err != nil {
		return nil, err
	}

	return entryOrders, nil
}

// GetEntryOrdersBySeller 当前用户作为卖家的全部挂单（按创建时间倒序，含历史）
func (n *NftOrdersRepository) GetEntryOrdersBySeller(seller string) ([]EntryOrdersWithImageUrl, error) {
	var entryOrders []EntryOrdersWithImageUrl
	err := n.DB.
		Table("orders_entry").
		Select("orders_entry.*, nft_list.image_url AS image_url").
		Joins("LEFT JOIN nft_list ON nft_list.id = orders_entry.nft_list_id").
		Where("LOWER(orders_entry.seller) = LOWER(?)", seller).
		Order("orders_entry.create_time DESC").
		Find(&entryOrders).Error
	if err != nil {
		return nil, err
	}
	return entryOrders, nil
}

// BidHistoryRow 买家出价历史联表查询结果
type BidHistoryRow struct {
	ID         uint         `gorm:"column:id"`
	OrdersID   uint         `gorm:"column:orders_id"`
	Buyer      string       `gorm:"column:buyer"`
	Price      float64      `gorm:"column:price"`
	Deadline   time.Time    `gorm:"column:deadline"`
	Salt       string       `gorm:"column:salt"`
	Status     enums.BidStatus `gorm:"column:status"`
	Signature  string       `gorm:"column:signature"`
	TxHash     string       `gorm:"column:tx_hash"`
	CreateTime time.Time    `gorm:"column:create_time"`
	UpdateTime time.Time    `gorm:"column:update_time"`
	NftListID  uint         `gorm:"column:nft_list_id"`
	TokenId    uint         `gorm:"column:token_id"`
	Seller           string       `gorm:"column:entry_seller"`
	ImageUrl         string       `gorm:"column:image_url"`
	ListingHash      string       `gorm:"column:listing_hash"`
	EntryOrderStatus enums.ListingStatus `gorm:"column:entry_order_status"`
}

// GetBidHistoryByBuyer 当前用户作为买家的全部出价记录
func (n *NftOrdersRepository) GetBidHistoryByBuyer(buyer string) ([]BidHistoryRow, error) {
	var rows []BidHistoryRow
	err := n.DB.Table("orders_bid").
		Select(`orders_bid.id, orders_bid.orders_id, orders_bid.buyer, orders_bid.price, orders_bid.deadline, orders_bid.salt, orders_bid.status, orders_bid.signature, orders_bid.tx_hash, orders_bid.create_time, orders_bid.update_time,
			orders_entry.nft_list_id AS nft_list_id, orders_entry.token_id AS token_id, orders_entry.seller AS entry_seller, orders_entry.status AS entry_order_status,
			COALESCE(nft_list.image_url, '') AS image_url,
			COALESCE(orders_entry.listing_hash, '') AS listing_hash`).
		Joins("INNER JOIN orders_entry ON orders_entry.id = orders_bid.orders_id").
		Joins("LEFT JOIN nft_list ON nft_list.id = orders_entry.nft_list_id").
		Where("LOWER(orders_bid.buyer) = LOWER(?)", buyer).
		Order("orders_bid.create_time DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// GetEntryOrderByNftListAndSeller 按 nft_list_id + 卖家地址取最新一条挂单（用于卖家查询自己的出价列表）
func (n *NftOrdersRepository) GetEntryOrderByNftListAndSeller(nftListId uint, seller string) (*model.EntryOrders, error) {
	var entry model.EntryOrders
	err := n.DB.Where("nft_list_id = ? AND LOWER(seller) = LOWER(?)", nftListId, seller).
		Order("id DESC").
		First(&entry).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &entry, nil
}

// 获取出价列表
func (n *NftOrdersRepository) GetBidPlacedList(ordersId uint) ([]model.BidPlaced, error) {
	var bidPlaced []model.BidPlaced
	if err := n.DB.Where("orders_id = ?", ordersId).Order("create_time DESC").Find(&bidPlaced).Error; err != nil {
		return nil, err
	}
	return bidPlaced, nil
}
