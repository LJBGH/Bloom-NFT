package response

import (
	"bloom-nft/enums"
	"time"
)

type MintResult struct {
	ImageCid    string `json:"imageCid"`
	MetadataCid string `json:"metadataCid"`
	TokenUrl    string `json:"tokenUrl"`    // "ipfs://" + MetadataCid,
	MetadataUrl string `json:"metadataUrl"` //"https://" + *** + "/ipfs/" + imageCID,
	ImageUrl    string `json:"imageUrl"`    //"https://" + *** + "/ipfs/" + metadataCID,
}

type NftListResult struct {
	ID          uint         `json:"id"`
	NftID       uint         `json:"nftId"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	ImageUrl    string       `json:"imageUrl"`
	MetadataUrl string       `json:"metadataUrl"`
	TokenUrl    string       `json:"tokenUrl"`
	TokenId     uint         `json:"tokenId"`
	Owner       string       `json:"owner"`
	CreateTime  time.Time    `json:"createTime"`
	UpdateTime  time.Time    `json:"updateTime"`
	Status      enums.ListingStatus `json:"status"`
	StatusDesc  string              `json:"statusDesc"`
	Price       float64             `json:"price"`
}

func (n *NftListResult) GetStatusDesc() string {
	return n.Status.Desc()
}
