package repository

import (
	"bloom-nft/model"
	"errors"

	"time"

	"gorm.io/gorm"
)

type NftListRepository struct {
	DB *gorm.DB
}

func NewNftListRepository(db *gorm.DB) *NftListRepository {
	return &NftListRepository{DB: db}
}

type NftListWithEntryOrders struct {
	model.NftList
	EntryOrders model.EntryOrders `gorm:"foreignKey:NftListID"`
}

// 根据nftID获取NFTList
func (n *NftListRepository) GetListByNftId(nftId uint) ([]NftListWithEntryOrders, error) {
	// 使用结构体作为条件，让 GORM 根据字段映射到正确的列名（避免 nftId/nft_id 不一致）。
	// 关联查询 entry_orders表，获取订单状态

	var nftListWithEntryOrders []NftListWithEntryOrders
	query := n.DB.Where(&model.NftList{NftID: nftId}).Joins("LEFT JOIN orders_entry ON orders_entry.nft_list_id = nft_list.id")
	if err := query.Find(&nftListWithEntryOrders).Error; err != nil {
		return nil, err
	}

	return nftListWithEntryOrders, nil
}

// 根据 owner 获取其拥有的 NFT 类目 ID 列表（去重按 nft_id）
func (n *NftListRepository) GetNftIDsByOwner(owner string) ([]uint, error) {
	var ids []uint
	if err := n.DB.
		Model(&model.NftList{}).
		Where("owner = ?", owner).
		Distinct().
		Pluck("nft_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// 根据 owner 和 nftID 获取该用户在某个 NFT 类目下的所有 NFT
func (n *NftListRepository) GetListByOwnerAndNftId(owner string, nftId uint) ([]model.NftList, error) {
	var nftList []model.NftList
	if err := n.DB.
		Where("owner = ? AND nft_id = ?", owner, nftId).
		Find(&nftList).Error; err != nil {
		return nil, err
	}
	return nftList, nil
}

// 插入
func (n *NftListRepository) Insert(nft model.NftList) error {
	// 幂等插入：存在则更新，不存在则创建。
	// 由于模型字段 `CreateTime/UpdateTime` 在 gorm tag 中是 not null，这里需要确保零值被填充。
	if n == nil || n.DB == nil {
		return errors.New("nft list repository db is nil")
	}

	now := time.Now().UTC()

	// CreateTime 仅在创建时保证；更新时会保留数据库已有的 CreateTime。
	if nft.CreateTime.IsZero() {
		nft.CreateTime = now
	}
	// UpdateTime 每次调用都刷新。
	nft.UpdateTime = now

	// 如果主键 id 不是 0，按主键做存在性判断。
	if nft.ID != 0 {
		var existing model.NftList
		err := n.DB.Where("id = ?", nft.ID).First(&existing).Error
		if err == nil {
			// 已存在：更新可变字段。
			existing.NftID = nft.NftID
			existing.Name = nft.Name
			existing.Description = nft.Description
			existing.ImageUrl = nft.ImageUrl
			existing.TokenUrl = nft.TokenUrl
			existing.TokenId = nft.TokenId
			existing.Owner = nft.Owner
			existing.MetadataUrl = nft.MetadataUrl
			existing.UpdateTime = nft.UpdateTime
			return n.DB.Save(&existing).Error
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 不存在：创建新记录。
			return n.DB.Create(&nft).Error
		}
		return err
	}

	// 如果没有提供 id，就退化为普通创建（由数据库/ORM 处理主键）。
	return n.DB.Create(&nft).Error
}
