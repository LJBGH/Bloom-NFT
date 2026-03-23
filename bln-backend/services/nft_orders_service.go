package services

import (
	"bloom-nft/api/request"
	"bloom-nft/api/response"
	"bloom-nft/enums"
	"bloom-nft/model"
	"bloom-nft/repository"
	"bloom-nft/utils"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"gorm.io/gorm"
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
func (n *NftOrdersService) EntryOrders(request *request.EntryOrdersRequest) (string, error) {
	entry := model.EntryOrders{
		NftListID:  request.NftListID,
		Seller:     request.Seller,
		TokenId:    request.TokenId,
		Price:      request.Price,
		Deadline:   request.Deadline,
		Salt:       strings.TrimSpace(request.Salt),
		Status:     enums.ListingReady,
		Signature:  request.Signature,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}

	if err := n.NftOrdersRepository.InsertEntryOrders(entry); err != nil {
		return "", fmt.Errorf("insert entry order failed: %w", err)
	}

	// 订单入库成功后，执行链上 listWithSig。
	// 注意：listing.nft 必须与前端签名时使用的 NFT 合约地址一致。
	var nftContractAddrStr string
	if err := n.NftOrdersRepository.DB.
		Table("nft_list").
		Select("nft.address").
		Joins("JOIN nft ON nft.id = nft_list.nft_id").
		Where("nft_list.id = ?", request.NftListID).
		Scan(&nftContractAddrStr).Error; err != nil {
		return "", fmt.Errorf("query nft contract address failed (nftListId=%d): %w", request.NftListID, err)
	}

	nftContractAddr := common.HexToAddress(nftContractAddrStr)
	if nftContractAddr == (common.Address{}) {
		return "", errors.New("invalid nft contract address")
	}

	fmt.Println("开始上链挂单：", nftContractAddr.Hex())

	txHash, err := utils.ListWithSigOnChain(entry, nftContractAddr)
	if err != nil {
		// best-effort: 上链失败记为已取消
		_ = n.NftOrdersRepository.DB.Model(&model.EntryOrders{}).
			Where("signature = ? AND seller = ? AND token_id = ? AND nft_list_id = ?",
				request.Signature, request.Seller, request.TokenId, request.NftListID).
			Updates(map[string]any{
				"status":      enums.ListingCancelled,
				"update_time": time.Now(),
			}).Error
		return "", fmt.Errorf("listWithSig on-chain failed (nftListId=%d, seller=%s, tokenId=%d): %w", request.NftListID, request.Seller, request.TokenId, err)
	}

	return txHash.Hex(), nil
}

// EntryOrdersBatch Merkle 批量上架：入库多笔后逐笔调用 listWithMerkleProof。
func (n *NftOrdersService) EntryOrdersBatch(req *request.BatchEntryOrdersRequest) ([]string, error) {
	if len(req.Items) == 0 {
		return nil, errors.New("empty items")
	}
	seller0 := strings.ToLower(strings.TrimSpace(req.Items[0].Seller))
	for _, it := range req.Items {
		if strings.ToLower(strings.TrimSpace(it.Seller)) != seller0 {
			return nil, errors.New("batch items must share the same seller")
		}
	}

	now := time.Now()
	var ids []uint
	if err := n.NftOrdersRepository.DB.Transaction(func(tx *gorm.DB) error {
		ids = make([]uint, 0, len(req.Items))
		for _, it := range req.Items {
			entry := model.EntryOrders{
				NftListID:  it.NftListID,
				Seller:     it.Seller,
				TokenId:    it.TokenId,
				Price:      it.Price,
				Deadline:   it.Deadline,
				Salt:       strings.TrimSpace(it.Salt),
				Status:     enums.ListingReady,
				Signature:  req.BatchSignature,
				IsMerkle:   true,
				CreateTime: now,
				UpdateTime: now,
			}
			if err := tx.Create(&entry).Error; err != nil {
				return err
			}
			ids = append(ids, entry.ID)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("insert batch entry failed: %w", err)
	}

	txHashes := make([]string, 0, len(req.Items))
	root := common.HexToHash(req.MerkleRoot)
	rootDeadline := req.RootDeadline.Unix()

	for i, it := range req.Items {
		var entry model.EntryOrders
		if err := n.NftOrdersRepository.DB.Where("id = ?", ids[i]).First(&entry).Error; err != nil {
			return nil, fmt.Errorf("load entry id=%d: %w", ids[i], err)
		}

		var nftContractAddrStr string
		if err := n.NftOrdersRepository.DB.
			Table("nft_list").
			Select("nft.address").
			Joins("JOIN nft ON nft.id = nft_list.nft_id").
			Where("nft_list.id = ?", it.NftListID).
			Scan(&nftContractAddrStr).Error; err != nil {
			_ = n.markBatchEntriesCancelled(ids)
			return nil, fmt.Errorf("query nft contract address failed: %w", err)
		}
		nftAddr := common.HexToAddress(nftContractAddrStr)
		if nftAddr == (common.Address{}) {
			_ = n.markBatchEntriesCancelled(ids)
			return nil, errors.New("invalid nft contract address")
		}

		proof := make([]common.Hash, len(it.Proof))
		for j, p := range it.Proof {
			proof[j] = common.HexToHash(p)
		}

		// 批量上架
		h, err := utils.ListWithMerkleProofOnChain(entry, nftAddr, proof, root, rootDeadline, req.BatchSignature)
		if err != nil {
			_ = n.markBatchEntriesCancelled(ids)
			return nil, fmt.Errorf("listWithMerkleProof failed (nftListId=%d): %w", it.NftListID, err)
		}
		txHashes = append(txHashes, h.Hex())
	}

	return txHashes, nil
}

// 批量取消上架
func (n *NftOrdersService) markBatchEntriesCancelled(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return n.NftOrdersRepository.DB.Model(&model.EntryOrders{}).
		Where("id IN ?", ids).
		Updates(map[string]any{
			"status":      enums.ListingCancelled,
			"update_time": time.Now(),
		}).Error
}

// 出价
func (n *NftOrdersService) BidPlaced(request *request.BidPlacedRequest) (string, error) {
	var entry model.EntryOrders
	if err := n.NftOrdersRepository.DB.Where("id = ?", request.OrdersID).First(&entry).Error; err != nil {
		return "", fmt.Errorf("query entry order failed: %w", err)
	}
	if entry.Status != enums.ListingPending {
		return "", fmt.Errorf("entry order status is not pending (orderId=%d, status=%d)", entry.ID, entry.Status)
	}

	var entryRef model.ChainRefEntryOrder
	if err := n.NftOrdersRepository.DB.Where("entry_order_id = ?", entry.ID).First(&entryRef).Error; err != nil {
		return "", fmt.Errorf("query listing hash mapping failed: %w", err)
	}

	bid := model.BidPlaced{
		OrdersID:   request.OrdersID,
		Buyer:      request.Buyer,
		Price:      request.Price,
		Deadline:   request.Deadline,
		Salt:       strings.TrimSpace(request.Salt),
		Status:     enums.BidReady,
		Signature:  request.Signature,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}
	if err := n.NftOrdersRepository.InsertBidPlaced(bid); err != nil {
		return "", fmt.Errorf("insert bid failed: %w", err)
	}
	if err := n.NftOrdersRepository.DB.Where("orders_id = ? AND buyer = ? AND salt = ?",
		bid.OrdersID, bid.Buyer, bid.Salt).Order("id DESC").First(&bid).Error; err != nil {
		return "", fmt.Errorf("query inserted bid failed: %w", err)
	}

	txHash, err := utils.BidWithSigOnChain(bid, common.HexToHash(entryRef.ListingHash))
	if err != nil {
		_ = n.NftOrdersRepository.DB.Model(&model.BidPlaced{}).Where("id = ?", bid.ID).Updates(map[string]any{
			"status":      enums.BidCancelled,
			"update_time": time.Now(),
		}).Error
		return "", fmt.Errorf("placeBid on-chain failed: %w", err)
	}

	return txHash.Hex(), nil
}

// 接受出价
func (n *NftOrdersService) BidAccepted(request *request.BidAcceptedRequest) (string, error) {
	var bid model.BidPlaced
	if err := n.NftOrdersRepository.DB.Where("id = ?", request.BidID).First(&bid).Error; err != nil {
		return "", fmt.Errorf("query bid failed: %w", err)
	}

	var entry model.EntryOrders
	if err := n.NftOrdersRepository.DB.Where("id = ?", bid.OrdersID).First(&entry).Error; err != nil {
		return "", fmt.Errorf("query entry order failed: %w", err)
	}

	if !strings.EqualFold(entry.Seller, request.Seller) {
		return "", errors.New("seller mismatch: only listing seller can accept this bid")
	}
	if entry.Status != enums.ListingPending {
		return "", fmt.Errorf("listing is not active (orderId=%d, status=%d)", entry.ID, entry.Status)
	}
	if bid.Status != enums.BidPending {
		return "", fmt.Errorf("bid is not active (bidId=%d, status=%d)", bid.ID, bid.Status)
	}

	var entryRef model.ChainRefEntryOrder
	if err := n.NftOrdersRepository.DB.Where("entry_order_id = ?", entry.ID).First(&entryRef).Error; err != nil {
		return "", fmt.Errorf("query listing hash mapping failed: %w", err)
	}

	var nftContractAddrStr string
	if err := n.NftOrdersRepository.DB.
		Table("nft_list").
		Select("nft.address").
		Joins("JOIN nft ON nft.id = nft_list.nft_id").
		Where("nft_list.id = ?", entry.NftListID).
		Scan(&nftContractAddrStr).Error; err != nil {
		return "", fmt.Errorf("query nft contract address failed (nftListId=%d): %w", entry.NftListID, err)
	}
	nftContractAddr := common.HexToAddress(nftContractAddrStr)
	if nftContractAddr == (common.Address{}) {
		return "", errors.New("invalid nft contract address")
	}

	txHash, err := utils.AcceptBidOnChain(entry, bid, nftContractAddr, common.HexToHash(entryRef.ListingHash))
	if err != nil {
		return "", fmt.Errorf("acceptBid on-chain failed: %w", err)
	}
	return txHash.Hex(), nil
}

// 获取挂单列表
func entryOrdersWithImageToResponse(entryOrders []repository.EntryOrdersWithImageUrl) []response.EntryOrdersResponse {
	resp := make([]response.EntryOrdersResponse, 0, len(entryOrders))
	for _, m := range entryOrders {
		resp = append(resp, response.EntryOrdersResponse{
			ID:          m.ID,
			NftListID:   m.NftListID,
			Seller:      m.Seller,
			Buyer:       m.Buyer,
			TokenId:     m.TokenId,
			Price:       m.Price,
			Deadline:    m.Deadline,
			Salt:        m.Salt,
			Status:      m.Status,
			StatusDesc:  m.EntryOrders.Status.Desc(),
			TxHash:      m.TxHash,
			Signature:   m.Signature,
			IsMerkle:    m.IsMerkle,
			CreateTime:  m.CreateTime,
			UpdateTime:  m.UpdateTime,
			ImageUrl:    m.ImageUrl,
			ListingHash: m.ListingHash,
		})
	}
	return resp
}

// 获取挂单列表
func (n *NftOrdersService) GetEntryOrdersList(nftId *uint, status *enums.ListingStatus) ([]response.EntryOrdersResponse, error) {
	entryOrders, err := n.NftOrdersRepository.GetEntryOrdersList(nftId, status)
	if err != nil {
		return nil, err
	}
	return entryOrdersWithImageToResponse(entryOrders), nil
}

// GetMyEntryOrdersBySeller 当前地址作为卖家的全部挂单（含历史）
func (n *NftOrdersService) GetMyEntryOrdersBySeller(seller string) ([]response.EntryOrdersResponse, error) {
	rows, err := n.NftOrdersRepository.GetEntryOrdersBySeller(seller)
	if err != nil {
		return nil, err
	}
	return entryOrdersWithImageToResponse(rows), nil
}

// GetMyBidHistoryByBuyer 当前地址作为买家的全部出价
func (n *NftOrdersService) GetMyBidHistoryByBuyer(buyer string) ([]response.BidHistoryResponse, error) {
	rows, err := n.NftOrdersRepository.GetBidHistoryByBuyer(buyer)
	if err != nil {
		return nil, err
	}
	out := make([]response.BidHistoryResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, response.BidHistoryResponse{
			ID:               r.ID,
			OrdersID:         r.OrdersID,
			Buyer:            r.Buyer,
			Price:            r.Price,
			Deadline:         r.Deadline,
			Salt:             r.Salt,
			Status:           r.Status,
			StatusDesc:       r.Status.Desc(),
			Signature:        r.Signature,
			TxHash:           r.TxHash,
			CreateTime:       r.CreateTime,
			UpdateTime:       r.UpdateTime,
			NftListID:        r.NftListID,
			TokenId:          r.TokenId,
			EntrySeller:      r.Seller,
			ImageUrl:         r.ImageUrl,
			ListingHash:      r.ListingHash,
			EntryOrderStatus: r.EntryOrderStatus,
		})
	}
	return out, nil
}

// 获取出价列表
func (n *NftOrdersService) GetBidPlacedList(ordersId uint) ([]model.BidPlaced, error) {
	return n.NftOrdersRepository.GetBidPlacedList(ordersId)
}

// GetBidPlacedListForSellerNftList 卖家按自己的 nft_list_id 查询该挂单下的出价（校验 nft_list_id + seller）
func (n *NftOrdersService) GetBidPlacedListForSellerNftList(nftListId uint, seller string) ([]model.BidPlaced, error) {
	entry, err := n.NftOrdersRepository.GetEntryOrderByNftListAndSeller(nftListId, seller)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("no entry order for this nft list or seller mismatch: %w", err)
		}
		return nil, err
	}
	return n.NftOrdersRepository.GetBidPlacedList(entry.ID)
}
