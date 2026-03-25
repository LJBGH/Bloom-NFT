package services

import (
	"bloom-nft/api/request"
	"bloom-nft/api/response"
	"bloom-nft/enums"
	"bloom-nft/model"
	"bloom-nft/repository"
	"bloom-nft/utils"
	"encoding/json"
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
func (n *NftOrdersService) OrdersEntry(request *request.EntryOrdersRequest) (string, error) {
	salt := strings.TrimSpace(request.Salt)

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

	// 计算 listingHash（用于 listener 反查 entry_orders 并更新状态）。
	chainID, err := utils.GetChainID()
	if err != nil {
		return "", fmt.Errorf("get chain id failed: %w", err)
	}
	marketplaceAddrStr := utils.GetContractAddress("BloomMarketplace")
	if marketplaceAddrStr == "" {
		return "", errors.New("BloomMarketplace address not found")
	}
	marketplaceAddr := common.HexToAddress(marketplaceAddrStr)
	if marketplaceAddr == (common.Address{}) {
		return "", errors.New("invalid BloomMarketplace address")
	}

	priceWei, err := utils.BtToWei(request.Price)
	if err != nil {
		return "", fmt.Errorf("convert listing price to wei failed: %w", err)
	}
	saltWei, err := utils.ParseListingSalt(salt)
	if err != nil {
		return "", fmt.Errorf("invalid listing salt failed: %w", err)
	}

	listingHash := utils.CalcListingHash(
		chainID,
		marketplaceAddr,
		nftContractAddr,
		common.HexToAddress(request.Seller),
		uint64(request.TokenId),
		priceWei,
		uint64(request.Deadline.Unix()),
		saltWei,
	)

	now := time.Now()
	entry := model.EntryOrders{
		NftListID: request.NftListID,
		Seller:    request.Seller,
		TokenId:   request.TokenId,
		Price:     request.Price,
		Deadline:  request.Deadline,
		Salt:      salt,
		// 新版本：不再链上上架；直接进入“进行中”，供 expiration worker 与 acceptBid 校验使用。
		Status:      enums.ListingPending,
		Signature:   request.Signature,
		ListingHash: strings.ToLower(listingHash.Hex()),
		// 非 Merkle 单笔挂单默认不参与 merkle 路径校验
		MerkleRoot:      "",
		RootDeadlineSec: 0,
		MerkleProof:     "[]",
		CreateTime:      now,
		UpdateTime:      now,
	}

	// 不再上链：返回空 txHash；链上 buy/acceptBid 后由 listener 更新状态。
	if err := n.NftOrdersRepository.InsertEntryOrders(entry); err != nil {
		return "", fmt.Errorf("insert entry order failed: %w", err)
	}
	return "", nil
}

// EntryOrdersBatch Merkle 批量上架：入库多笔后逐笔调用 listWithMerkleProof。
func (n *NftOrdersService) OrdersEntryBatch(req *request.BatchEntryOrdersRequest) ([]string, error) {
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

	// 新版本合约不再支持链上 Merkle 上架；因此批量接口只负责入库。
	// 为了后续 listener 能通过 Buy/BidAccepted 里的 listingHash 反查到 entry_orders，
	// 这里直接把 listingHash 写入 entry_orders.listing_hash。
	chainID, err := utils.GetChainID()
	if err != nil {
		return nil, fmt.Errorf("get chain id failed: %w", err)
	}
	marketplaceAddrStr := utils.GetContractAddress("BloomMarketplace")
	if marketplaceAddrStr == "" {
		return nil, errors.New("BloomMarketplace address not found")
	}
	marketplaceAddr := common.HexToAddress(marketplaceAddrStr)
	if marketplaceAddr == (common.Address{}) {
		return nil, errors.New("invalid BloomMarketplace address")
	}

	merkleRoot := strings.ToLower(strings.TrimSpace(req.MerkleRoot))
	if merkleRoot == "" {
		return nil, errors.New("merkleRoot is empty")
	}
	rootDeadlineSec := uint64(req.RootDeadline.Unix())
	batchSig := strings.TrimSpace(req.BatchSignature)
	if batchSig == "" {
		return nil, errors.New("batchSignature is empty")
	}

	txHashes := make([]string, len(req.Items)) // 为兼容历史 API：txHash 为空（不再上链上架）

	if err := n.NftOrdersRepository.DB.Transaction(func(tx *gorm.DB) error {
		for _, it := range req.Items {
			var nftContractAddrStr string
			if err := tx.Table("nft_list").
				Select("nft.address").
				Joins("JOIN nft ON nft.id = nft_list.nft_id").
				Where("nft_list.id = ?", it.NftListID).
				Scan(&nftContractAddrStr).Error; err != nil {
				return fmt.Errorf("query nft contract address failed: %w", err)
			}
			nftAddr := common.HexToAddress(nftContractAddrStr)
			if nftAddr == (common.Address{}) {
				return errors.New("invalid nft contract address")
			}

			salt := strings.TrimSpace(it.Salt)
			priceWei, err := utils.BtToWei(it.Price)
			if err != nil {
				return fmt.Errorf("convert listing price to wei failed (nftListId=%d): %w", it.NftListID, err)
			}
			saltWei, err := utils.ParseListingSalt(salt)
			if err != nil {
				return fmt.Errorf("invalid listing salt failed (nftListId=%d): %w", it.NftListID, err)
			}

			listingHash := utils.CalcListingHash(
				chainID,
				marketplaceAddr,
				nftAddr,
				common.HexToAddress(it.Seller),
				uint64(it.TokenId),
				priceWei,
				uint64(it.Deadline.Unix()),
				saltWei,
			)

			entry := model.EntryOrders{
				NftListID:       it.NftListID,
				Seller:          it.Seller,
				TokenId:         it.TokenId,
				Price:           it.Price,
				Deadline:        it.Deadline,
				Salt:            salt,
				Status:          enums.ListingPending,
				Signature:       batchSig,
				IsMerkle:        true,
				MerkleRoot:      merkleRoot,
				RootDeadlineSec: rootDeadlineSec,
				MerkleProof: func() string {
					proof := it.Proof
					if proof == nil {
						proof = []string{}
					}
					b, _ := json.Marshal(proof)
					return string(b)
				}(),
				ListingHash: strings.ToLower(listingHash.Hex()),
				CreateTime:  now,
				UpdateTime:  now,
			}

			if err := tx.Create(&entry).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("insert batch entry failed: %w", err)
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
func (n *NftOrdersService) OrdersBid(request *request.BidPlacedRequest) (string, error) {
	var entry model.EntryOrders
	if err := n.NftOrdersRepository.DB.Where("id = ?", request.OrdersID).First(&entry).Error; err != nil {
		return "", fmt.Errorf("query entry order failed: %w", err)
	}
	if entry.Status != enums.ListingPending {
		return "", fmt.Errorf("entry order status is not pending (orderId=%d, status=%d)", entry.ID, entry.Status)
	}

	now := time.Now()
	bid := model.BidPlaced{
		OrdersID:   request.OrdersID,
		Buyer:      request.Buyer,
		Price:      request.Price,
		Deadline:   request.Deadline,
		Salt:       strings.TrimSpace(request.Salt),
		Status:     enums.BidPending,
		Signature:  request.Signature,
		CreateTime: now,
		UpdateTime: now,
	}

	// 计算 bidHash，直接写入 bid_placed.bid_hash，供 listener 通过 bidHash 反查更新状态。
	chainID, err := utils.GetChainID()
	if err != nil {
		return "", fmt.Errorf("get chain id failed: %w", err)
	}
	marketplaceAddrStr := utils.GetContractAddress("BloomMarketplace")
	if marketplaceAddrStr == "" {
		return "", errors.New("BloomMarketplace address not found")
	}
	marketplaceAddr := common.HexToAddress(marketplaceAddrStr)
	if marketplaceAddr == (common.Address{}) {
		return "", errors.New("invalid BloomMarketplace address")
	}

	var nftContractAddrStr string
	if err := n.NftOrdersRepository.DB.
		Table("nft_list").
		Select("nft.address").
		Joins("JOIN nft ON nft.id = nft_list.nft_id").
		Where("nft_list.id = ?", entry.NftListID).
		Scan(&nftContractAddrStr).Error; err != nil {
		return "", fmt.Errorf("query nft contract address failed: %w", err)
	}
	nftContractAddr := common.HexToAddress(nftContractAddrStr)
	if nftContractAddr == (common.Address{}) {
		return "", errors.New("invalid nft contract address")
	}

	priceWei, err := utils.BtToWei(bid.Price)
	if err != nil {
		return "", fmt.Errorf("convert bid price to wei failed: %w", err)
	}
	saltWei, err := utils.ParseListingSalt(bid.Salt)
	if err != nil {
		return "", fmt.Errorf("invalid bid salt failed: %w", err)
	}

	bidHash := utils.CalcBidHash(
		chainID,
		marketplaceAddr,
		nftContractAddr,
		common.HexToAddress(bid.Buyer),
		uint64(entry.TokenId),
		priceWei,
		uint64(bid.Deadline.Unix()),
		saltWei,
	)

	bid.BidHash = strings.ToLower(bidHash.Hex())

	if err := n.NftOrdersRepository.InsertBidPlaced(bid); err != nil {
		return "", fmt.Errorf("insert bid failed: %w", err)
	}

	// 不再上链：返回空 txHash。
	return "", nil
}

// 接受出价
func (n *NftOrdersService) OrdersBidAccepted(request *request.BidAcceptedRequest) (string, error) {
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

	// Merkle 参数：从 orders_entry 存储读取（不依赖前端传入）
	var merkleProof []common.Hash
	merkleRoot := common.Hash{}
	rootDeadline := time.Unix(0, 0)
	if entry.IsMerkle {
		if strings.TrimSpace(entry.MerkleRoot) == "" {
			return "", errors.New("orders_entry.merkleRoot is empty for merkle listing")
		}
		merkleRoot = common.HexToHash(strings.TrimSpace(entry.MerkleRoot))
		rootDeadline = time.Unix(int64(entry.RootDeadlineSec), 0)

		// MerkleProof 存储为 JSON 数组字符串：["0x..","0x.."]
		if strings.TrimSpace(entry.MerkleProof) != "" && entry.MerkleProof != "[]" {
			var proofStrs []string
			if err := json.Unmarshal([]byte(entry.MerkleProof), &proofStrs); err != nil {
				return "", fmt.Errorf("parse orders_entry.merkleProof json failed: %w", err)
			}
			merkleProof = make([]common.Hash, 0, len(proofStrs))
			for _, p := range proofStrs {
				if strings.TrimSpace(p) == "" {
					continue
				}
				merkleProof = append(merkleProof, common.HexToHash(strings.TrimSpace(p)))
			}
		}
	}

	txHash, err := utils.AcceptBidOnChain(entry, bid, nftContractAddr, merkleProof, merkleRoot, rootDeadline)
	if err != nil {
		return "", fmt.Errorf("acceptBid on-chain failed: %w", err)
	}
	return txHash.Hex(), nil
}

// 获取挂单列表
func entryOrdersWithImageToResponse(entryOrders []repository.EntryOrdersWithImageUrl) []response.EntryOrdersResponse {
	resp := make([]response.EntryOrdersResponse, 0, len(entryOrders))
	for _, m := range entryOrders {
		var merkleProof []string
		if strings.TrimSpace(m.MerkleProof) != "" {
			// MerkleProof 存为 JSON 数组字符串：["0x..","0x.."]
			_ = json.Unmarshal([]byte(m.MerkleProof), &merkleProof)
		}
		resp = append(resp, response.EntryOrdersResponse{
			ID:         m.ID,
			NftListID:  m.NftListID,
			Seller:     m.Seller,
			Buyer:      m.Buyer,
			TokenId:    m.TokenId,
			Price:      m.Price,
			Deadline:   m.Deadline,
			Salt:       m.Salt,
			Status:     m.Status,
			StatusDesc: m.EntryOrders.Status.Desc(),
			TxHash:     m.TxHash,
			Signature:  m.Signature,
			IsMerkle:   m.IsMerkle,
			MerkleRoot: m.MerkleRoot,
			// rootDeadlineSec 是 uint256(unix seconds)
			RootDeadlineSec: m.RootDeadlineSec,
			MerkleProof:     merkleProof,
			CreateTime:      m.CreateTime,
			UpdateTime:      m.UpdateTime,
			ImageUrl:        m.ImageUrl,
			ListingHash:     m.ListingHash,
		})
	}
	return resp
}

// 获取挂单列表
func (n *NftOrdersService) GetOrdersEntryList(nftId *uint, status *enums.ListingStatus) ([]response.EntryOrdersResponse, error) {
	entryOrders, err := n.NftOrdersRepository.GetEntryOrdersList(nftId, status)
	if err != nil {
		return nil, err
	}
	return entryOrdersWithImageToResponse(entryOrders), nil
}

// GetMyEntryOrdersBySeller 当前地址作为卖家的全部挂单（含历史）
func (n *NftOrdersService) GetMyOrdersEntryBySeller(seller string) ([]response.EntryOrdersResponse, error) {
	rows, err := n.NftOrdersRepository.GetEntryOrdersBySeller(seller)
	if err != nil {
		return nil, err
	}
	return entryOrdersWithImageToResponse(rows), nil
}

// GetMyBidHistoryByBuyer 当前地址作为买家的全部出价
func (n *NftOrdersService) GetMyOrdersBidHistoryByBuyer(buyer string) ([]response.BidHistoryResponse, error) {
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
func (n *NftOrdersService) GetOrdersBidList(ordersId uint) ([]model.BidPlaced, error) {
	return n.NftOrdersRepository.GetBidPlacedList(ordersId)
}

// GetBidPlacedListForSellerNftList 卖家按自己的 nft_list_id 查询该挂单下的出价（校验 nft_list_id + seller）
func (n *NftOrdersService) GetOrdersBidListForSellerNftList(nftListId uint, seller string) ([]model.BidPlaced, error) {
	entry, err := n.NftOrdersRepository.GetEntryOrderByNftListAndSeller(nftListId, seller)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("no entry order for this nft list or seller mismatch: %w", err)
		}
		return nil, err
	}
	return n.NftOrdersRepository.GetBidPlacedList(entry.ID)
}
