package repository

import (
	"errors"
	"bloom-nft/model"

	"gorm.io/gorm"
	"time"
)

type NftRepository struct {
	DB *gorm.DB
}

func NewNftRepository(db *gorm.DB) *NftRepository {
	return &NftRepository{DB: db}
}

// 根据ID获取NFT
func (n *NftRepository) GetById(id uint) (*model.Nft, error) {
	var nft model.Nft
	if err := n.DB.Where("id = ?", id).First(&nft).Error; err != nil {
		return nil, err
	}
	return &nft, nil
}

// 获取所有NFT
func (r *NftRepository) GetAll() ([]model.Nft, error) {
	var nft []model.Nft
	if err := r.DB.Find(&nft).Error; err != nil {
		return nil, err
	}
	return nft, nil
}

// 插入NFT
func (n *NftRepository) Insert(nft model.Nft) error {
	// 幂等插入：存在则更新，不存在则创建。
	// 由于模型字段 `CreateTime/UpdateTime` 在 gorm tag 中是 not null，这里需要确保零值被填充。
	if n == nil || n.DB == nil {
		return errors.New("nft repository db is nil")
	}

	now := time.Now().UTC()

	// CreateTime 仅在创建时保证；更新时会保留数据库已有的 CreateTime。
	if nft.CreateTime.IsZero() {
		nft.CreateTime = now
	}
	// UpdateTime 每次调用都刷新。
	nft.UpdateTime = now

	// 如果主键 id 不是 0，按主键做存在性判断（便于区块链 token id 作为业务主键）。
	if nft.ID != 0 {
		var existing model.Nft
		err := n.DB.Where("id = ?", nft.ID).First(&existing).Error
		if err == nil {
			// 已存在：更新可变字段。
			existing.Name = nft.Name
			existing.Description = nft.Description
			existing.Address = nft.Address
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
