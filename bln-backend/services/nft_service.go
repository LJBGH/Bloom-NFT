package services

import (
	"bloom-nft/api/request"
	"bloom-nft/api/response"
	"bloom-nft/config"
	"bloom-nft/model"
	"bloom-nft/repository"
	"bloom-nft/utils"
	"fmt"
	"io"
	"time"
)

type NftService struct {
	NftRepository     *repository.NftRepository
	NftListRepository *repository.NftListRepository
}

func NewNftService(nftRepository *repository.NftRepository, nftListRepository *repository.NftListRepository) *NftService {
	return &NftService{
		NftRepository:     nftRepository,
		NftListRepository: nftListRepository,
	}
}

// 铸造NFT
func (n *NftService) Mint(request *request.MintRequest, fileData io.Reader) (*response.MintResult, error) {

	// 上传到 Pinata
	imageCID, err := utils.UploadToPinata(request.Name, fileData)
	if err != nil {
		return nil, err
	}

	metadata := map[string]interface{}{
		"name":        request.Name,
		"description": request.Description,
		"image":       "ipfs://" + imageCID, // ⚠️ 关键：使用 ipfs:// 协议
		// "external_url": config.AppConfig.IpfsPinana.ViewUrl + imageCID,
		"attributes": []map[string]string{
			{"trait_type": "Background", "value": "Red"},
			{"trait_type": "Eyes", "value": "Blue"},
		},
	}

	metadataCID, err := utils.UploadJSONToPinata(metadata)
	if err != nil {
		return nil, err
	}

	mingResult := response.MintResult{
		ImageCid:    imageCID,
		MetadataCid: metadataCID,
		TokenUrl:    "ipfs://" + metadataCID,
		MetadataUrl: fmt.Sprintf("%s/ipfs/%s", config.AppConfig.IpfsPinana.ViewUrl, metadataCID),
		ImageUrl:    fmt.Sprintf("%s/ipfs/%s", config.AppConfig.IpfsPinana.ViewUrl, imageCID),
	}

	// 插入数据库
	n.NftListRepository.Insert(model.NftList{
		NftID:       1,
		Name:        request.Name,
		Description: request.Description,
		ImageUrl:    mingResult.ImageUrl,
		MetadataUrl: mingResult.MetadataUrl,
		TokenUrl:    mingResult.TokenUrl,
		CreateTime:  time.Now(),
		UpdateTime:  time.Now(),
	})

	return &mingResult, nil
}

// 更新NFT信息
func (n *NftService) UpdateNftList(tokenUrl string, tokenId uint, owner string) error {
	var nftList model.NftList
	// 以传入的 tokenUrl 查询记录（之前误用了 nftList.TokenUrl，导致一直是空字符串）
	err := n.NftListRepository.DB.Where("token_url = ?", tokenUrl).First(&nftList).Error
	if err != nil {
		return err
	}

	// 已存在：更新可变字段
	nftList.TokenId = tokenId
	nftList.Owner = owner
	nftList.UpdateTime = time.Now().UTC()

	return n.NftListRepository.DB.Save(&nftList).Error
}

// 获取所有系列NFT
func (n *NftService) AllNft() ([]model.Nft, error) {
	allNft, err := n.NftRepository.GetAll()
	if err != nil {
		return nil, err
	}

	return allNft, nil
}

// 根据NFT系列Id 获取所有NFT
func (n *NftService) AllNftList(nftId uint) ([]model.NftList, error) {
	allNftList, err := n.NftListRepository.GetListByNftId(nftId)
	if err != nil {
		return nil, err
	}

	return allNftList, nil
}

// 根据用户地址获取其拥有的 NFT 类目列表（返回 Nft）
func (n *NftService) CategoriesByOwner(owner string) ([]model.Nft, error) {
	ids, err := n.NftListRepository.GetNftIDsByOwner(owner)
	if err != nil {
		return nil, err
	}

	result := make([]model.Nft, 0, len(ids))
	for _, id := range ids {
		nft, err := n.NftRepository.GetById(id)
		if err != nil {
			return nil, err
		}
		result = append(result, *nft)
	}
	return result, nil
}

// 根据用户地址和 NFT 系列 Id 获取该用户在该类目下的所有 NFT
func (n *NftService) NftListByOwnerAndNftId(owner string, nftId uint) ([]model.NftList, error) {
	list, err := n.NftListRepository.GetListByOwnerAndNftId(owner, nftId)
	if err != nil {
		return nil, err
	}
	return list, nil
}
