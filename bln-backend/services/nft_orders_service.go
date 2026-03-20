package services

import (
	"bloom-nft/api/request"
	"bloom-nft/api/response"
	"bloom-nft/enums"
	"bloom-nft/model"
	"bloom-nft/repository"
	"time"
)

type NftOrdersService struct {
	NftOrdersRepository *repository.NftOrdersRepository
}

func NewNftOrdersService(nftOrdersRepository *repository.NftOrdersRepository) *NftOrdersService {
	return &NftOrdersService{
		NftOrdersRepository: nftOrdersRepository,
	}
}

// 挂单
func (n *NftOrdersService) EntryOrders(request *request.EntryOrdersRequest) error {
	err := n.NftOrdersRepository.InsertEntryOrders(model.EntryOrders{
		NftListID:  request.NftListID,
		Seller:     request.Seller,
		Buyer:      request.Buyer,
		TokenId:    request.TokenId,
		Price:      request.Price,
		Deadline:   request.Deadline,
		Nonce:      request.Nonce,
		Status:     enums.Pending,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	})
	if err != nil {
		return err
	}
	return nil
}

// 出价
func (n *NftOrdersService) BidPlaced(request *request.BidPlacedRequest) error {
	err := n.NftOrdersRepository.InsertBidPlaced(model.BidPlaced{
		OrdersID:   request.OrdersID,
		Buyer:      request.Buyer,
		Price:      request.Price,
		Deadline:   request.Deadline,
		Nonce:      request.Nonce,
		Status:     enums.Pending,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	})
	if err != nil {
		return err
	}
	return nil
}

// 获取挂单列表
func (n *NftOrdersService) GetEntryOrdersList(nftId *uint) ([]response.EntryOrdersResponse, error) {
	entryOrders, err := n.NftOrdersRepository.GetEntryOrdersList(nftId)
	if err != nil {
		return nil, err
	}

	resp := make([]response.EntryOrdersResponse, 0, len(entryOrders))
	for _, m := range entryOrders {
		resp = append(resp, response.EntryOrdersResponse{
			ID:         m.ID,
			NftListID:  m.NftListID,
			Seller:     m.Seller,
			Buyer:      m.Buyer,
			TokenId:    m.TokenId,
			Price:      m.Price,
			Deadline:   m.Deadline,
			Nonce:      m.Nonce,
			Status:     m.Status,
			CreateTime: m.CreateTime,
			UpdateTime: m.UpdateTime,
			ImageUrl:   m.ImageUrl,
		})
	}
	return resp, nil
}

// 获取出价列表
func (n *NftOrdersService) GetBidPlacedList(ordersId uint) ([]model.BidPlaced, error) {
	return n.NftOrdersRepository.GetBidPlacedList(ordersId)
}
