package listener

import (
	"bloom-nft/config"
	"bloom-nft/database"
	"bloom-nft/enums"
	"bloom-nft/model"
	"bloom-nft/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type MarketplaceListener struct {
	db           *gorm.DB
	client       *ethclient.Client
	abiObj       abi.ABI
	contractAddr common.Address
	chainID      *big.Int
	batchSize    uint64
}

// StartMarketplaceListener 初始化 RPC/ABI 上下文并启动市场合约事件轮询。
func StartMarketplaceListener(ctx context.Context) error {
	// 如果监听器未启用，则返回。
	if !config.AppConfig.Listener.Enabled {
		log.Info("listener disabled by config")
		return nil
	}

	// 获取 RPC URL。
	rpcURL := config.AppConfig.NetWork.RpcUrl
	if strings.TrimSpace(rpcURL) == "" {
		return errors.New("empty rpc url")
	}

	// 获取合约地址。
	addrHex := utils.GetContractAddress("BloomMarketplace")
	if !common.IsHexAddress(addrHex) {
		return fmt.Errorf("invalid BloomMarketplace address: %s", addrHex)
	}

	// 获取合约 ABI。
	abiText := utils.GetContractABI("BloomMarketplace")
	if strings.TrimSpace(abiText) == "" {
		return errors.New("empty BloomMarketplace ABI")
	}

	// 解析合约 ABI。
	abiObj, err := abi.JSON(strings.NewReader(abiText))
	if err != nil {
		return fmt.Errorf("parse BloomMarketplace ABI failed: %w", err)
	}

	// 连接 RPC。
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return fmt.Errorf("dial rpc failed: %w", err)
	}

	// 获取链 ID。
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("get chain id failed: %w", err)
	}

	// 获取批量大小。
	batchSize := config.AppConfig.Listener.BatchSize
	if batchSize == 0 {
		batchSize = 500
	}

	// 创建监听器。
	l := &MarketplaceListener{
		db:           database.DB,
		client:       client,
		abiObj:       abiObj,
		contractAddr: common.HexToAddress(addrHex),
		chainID:      chainID,
		batchSize:    batchSize,
	}

	// 打印监听器启动信息。
	log.WithFields(log.Fields{
		"chainId":    chainID.String(),
		"contract":   l.contractAddr.Hex(),
		"batchSize":  batchSize,
		"pollSecond": config.AppConfig.Listener.PollIntervalSec,
	}).Info("marketplace listener started")

	return l.run(ctx)
}

// run 运行监听器。
func (l *MarketplaceListener) run(ctx context.Context) error {
	// 获取轮询间隔。
	pollSec := config.AppConfig.Listener.PollIntervalSec
	if pollSec <= 0 {
		pollSec = 5
	}
	// 创建轮询器。
	ticker := time.NewTicker(time.Duration(pollSec) * time.Second)
	defer ticker.Stop()

	// 初始化同步。
	if err := l.syncOnce(ctx); err != nil {
		log.WithError(err).Error("listener initial sync failed")
	}

	// 轮询处理。
	for {
		select {
		// 上下文取消。
		case <-ctx.Done():
			return nil
		// 轮询处理。
		case <-ticker.C:
			if err := l.syncOnce(ctx); err != nil {
				log.WithError(err).Error("listener sync failed")
			}
		}
	}
}

// syncOnce 同步一次区块范围 [from, to]，处理完成后推进游标。
func (l *MarketplaceListener) syncOnce(ctx context.Context) error {
	// 获取最新区块号。
	latest, err := l.client.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("get latest block failed: %w", err)
	}

	// 获取游标。
	cursor, err := l.getOrInitCursor()
	if err != nil {
		return err
	}

	// 获取起始区块号。
	from := cursor.LastBlock + 1
	if cursor.LastBlock == 0 && config.AppConfig.Listener.StartBlock > 0 {
		from = config.AppConfig.Listener.StartBlock
	}
	if from > latest {
		return nil
	}

	// 获取结束区块号。
	to := from + l.batchSize - 1
	if to > latest {
		to = latest
	}

	// 仅查询当前批次窗口内、目标市场合约地址的日志。
	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(from),
		ToBlock:   new(big.Int).SetUint64(to),
		Addresses: []common.Address{l.contractAddr},
	}

	// 过滤日志。
	logs, err := l.client.FilterLogs(ctx, query)
	if err != nil {
		return fmt.Errorf("filter logs failed [from=%d,to=%d]: %w", from, to, err)
	}

	// 遍历日志并处理。
	for _, vLog := range logs {
		if err := l.handleLog(vLog); err != nil {
			log.WithError(err).WithFields(log.Fields{
				"txHash":      vLog.TxHash.Hex(),
				"logIndex":    vLog.Index,
				"block":       vLog.BlockNumber,
				"contract":    l.contractAddr.Hex(),
				"topic0":      topicHex(vLog, 0),
				"topicsCount": len(vLog.Topics),
			}).Error("handle event log failed")
		}
	}

	// 更新游标，推进区块范围。
	return l.updateCursor(to)
}

// handleLog 处理市场合约事件日志。
func (l *MarketplaceListener) handleLog(vLog types.Log) error {
	if len(vLog.Topics) == 0 {
		return nil
	}

	// topic0 是事件签名哈希，这里将其解析为 ABI 事件元信息。
	ev, err := l.abiObj.EventByID(vLog.Topics[0])
	if err != nil {
		return nil
	}

	// 幂等保护：已写入 chain_event_logs 的日志不重复处理。
	exist, err := l.isEventProcessed(vLog)
	if err != nil {
		return err
	}
	if exist {
		return nil
	}

	// 按事件类型分发到对应业务处理函数。
	switch ev.Name {
	case "OrderCancelled":
		err = l.onOrderCancelled(vLog, ev) // 取消上架/撤回出价事件（取决于 isListing）
	case "Buy":
		err = l.onBuy(vLog) // 购买事件
	case "BidAccepted":
		err = l.onBidAccepted(vLog) // 接受出价事件
	default:
		return nil
	}
	if err != nil {
		return err
	}

	// 仅在业务更新成功后落库处理标记，避免“标记成功但业务失败”。
	return l.insertEventLog(vLog, ev.Name)
}

// weiToBtFloat 将 18 位 wei 转为 BT 浮点（与前端 parseUnits 精度对齐）。
func weiToBtFloat(wei *big.Int) float64 {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	r := new(big.Rat).SetFrac(wei, scale)
	f, _ := r.Float64()
	return f
}

// onOrderCancelled 处理合约的取消事件（取消上架/撤回出价）。
// BloomMarketplace.sol: event OrderCancelled(bytes32 indexed orderHash, address indexed maker, bool isListing);
// - isListing=true  => cancel 上架：topics[1]=listingHash
// - isListing=false => cancel 出价：topics[1]=bidHash
func (l *MarketplaceListener) onOrderCancelled(vLog types.Log, ev *abi.Event) error {
	var data struct {
		IsListing bool
	}
	if err := l.abiObj.UnpackIntoInterface(&data, ev.Name, vLog.Data); err != nil {
		return fmt.Errorf("unpack OrderCancelled failed: %w", err)
	}
	if len(vLog.Topics) < 2 {
		return errors.New("invalid OrderCancelled topics")
	}

	orderHash := strings.ToLower(vLog.Topics[1].Hex())
	now := time.Now()

	return l.db.Transaction(func(tx *gorm.DB) error {
		if data.IsListing {
			// 取消上架订单：更新 entry_orders 状态为 ListingCancelled，并更新其 pending 的 bids 状态为 BidDelisted
			var entry model.EntryOrders
			if err := tx.Where("listing_hash = ?", orderHash).First(&entry).Error; err != nil {
				return fmt.Errorf("find entry order failed: %w", err)
			}
			if err := tx.Model(&model.EntryOrders{}).Where("id = ?", entry.ID).Updates(map[string]any{
				"status":      enums.ListingCancelled,
				"update_time": now,
			}).Error; err != nil {
				return err
			}
			return tx.Model(&model.BidPlaced{}).
				Where("orders_id = ? AND status = ?", entry.ID, enums.BidPending).
				Updates(map[string]any{
					"status":      enums.BidDelisted,
					"update_time": now,
				}).Error
		}

		// cancelBidOrder: update orders_bid
		return tx.Model(&model.BidPlaced{}).Where("bid_hash = ?", orderHash).Updates(map[string]any{
			"status":      enums.BidCancelled,
			"update_time": now,
		}).Error
	})
}

// onBuy 处理购买事件。
func (l *MarketplaceListener) onBuy(vLog types.Log) error {
	// 新合约 Buy 事件：topics[1]=listingHash, topics[2]=seller, topics[3]=buyer
	if len(vLog.Topics) < 4 {
		return errors.New("invalid Buy topics")
	}

	// 获取 listingHash。
	listingHash := strings.ToLower(vLog.Topics[1].Hex())
	buyer := common.HexToAddress(vLog.Topics[3].Hex()).Hex()
	now := time.Now()

	// 更新订单状态。
	return l.db.Transaction(func(tx *gorm.DB) error {
		var entry model.EntryOrders
		if err := tx.Where("listing_hash = ?", listingHash).First(&entry).Error; err != nil {
			return fmt.Errorf("find entry order failed: %w", err)
		}
		buyerLower := strings.ToLower(buyer)
		if err := tx.Model(&model.EntryOrders{}).Where("id = ?", entry.ID).Updates(map[string]any{
			"status":      enums.ListingCompleted,
			"buyer":       buyerLower,
			"tx_hash":     vLog.TxHash.Hex(),
			"update_time": now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.NftList{}).Where("id = ?", entry.NftListID).Updates(map[string]any{
			"owner":       buyerLower,
			"update_time": now,
		}).Error; err != nil {
			return err
		}
		// 直购成交时，该单下仍在进行中的出价均视为未中标，买家可链上 refundLosingBid
		return tx.Model(&model.BidPlaced{}).
			Where("orders_id = ? AND status = ?", entry.ID, enums.BidPending).
			Updates(map[string]any{
				"status":      enums.BidOutbid,
				"update_time": now,
			}).Error
	})
}

// onBidAccepted 处理接受出价事件。
func (l *MarketplaceListener) onBidAccepted(vLog types.Log) error {
	// 新合约 BidAccepted 事件：topics[1]=listingHash, topics[2]=bidHash
	if len(vLog.Topics) < 3 {
		return errors.New("invalid BidAccepted topics")
	}
	var data struct {
		// BidAccepted 事件 data（不含 indexed 字段）顺序为：buyer,nft,tokenId,price,fee
		Buyer   common.Address
		Nft     common.Address
		TokenId *big.Int
		Price   *big.Int
		Fee     *big.Int
	}

	// 解析事件数据。
	if err := l.abiObj.UnpackIntoInterface(&data, "BidAccepted", vLog.Data); err != nil {
		return fmt.Errorf("unpack BidAccepted failed: %w", err)
	}

	// 获取 listingHash。
	listingHash := strings.ToLower(vLog.Topics[1].Hex())
	// 获取 bidHash。
	bidHash := strings.ToLower(vLog.Topics[2].Hex())
	// 获取买家地址。
	buyer := strings.ToLower(data.Buyer.Hex())
	// 获取当前时间。
	now := time.Now()

	// 更新订单状态。
	return l.db.Transaction(func(tx *gorm.DB) error {
		var entry model.EntryOrders
		if err := tx.Where("listing_hash = ?", listingHash).First(&entry).Error; err != nil {
			return fmt.Errorf("find entry order failed: %w", err)
		}
		// 更新订单状态。
		if err := tx.Model(&model.EntryOrders{}).Where("id = ?", entry.ID).Updates(map[string]any{
			"status":      enums.ListingCompleted,
			"buyer":       buyer,
			"tx_hash":     vLog.TxHash.Hex(),
			"update_time": now,
		}).Error; err != nil {
			return err
		}

		var bid model.BidPlaced
		if err := tx.Where("bid_hash = ?", bidHash).First(&bid).Error; err != nil {
			return fmt.Errorf("find bid failed: %w", err)
		}
		// 更新中标出价状态。
		if err := tx.Model(&model.BidPlaced{}).Where("id = ?", bid.ID).Updates(map[string]any{
			"status":      enums.BidCompleted,
			"tx_hash":     vLog.TxHash.Hex(),
			"update_time": now,
		}).Error; err != nil {
			return err
		}

		// 同步 NFT 持有者在库中的记录（成交后链上 owner 已为买家）
		if err := tx.Model(&model.NftList{}).Where("id = ?", entry.NftListID).Updates(map[string]any{
			"owner":       buyer,
			"update_time": now,
		}).Error; err != nil {
			return err
		}

		// 其余仍处于「进行中」的出价标记为未中标，买家需调用合约 refundLosingBid 取回托管 BT
		return tx.Model(&model.BidPlaced{}).
			Where("orders_id = ? AND id <> ? AND status = ?", entry.ID, bid.ID, enums.BidPending).
			Updates(map[string]any{
				"status":      enums.BidOutbid,
				"update_time": now,
			}).Error
	})
}

// getOrInitCursor 获取或初始化游标。
func (l *MarketplaceListener) getOrInitCursor() (*model.ChainEventCursor, error) {
	chainID := l.chainID.String()
	contract := strings.ToLower(l.contractAddr.Hex())
	var cursor model.ChainEventCursor
	err := l.db.Where("chain_id = ? AND contract = ?", chainID, contract).First(&cursor).Error
	if err == nil {
		return &cursor, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	// 首次启动：游标初始化为 0，实际起始区块由 syncOnce 中的 StartBlock 控制。
	now := time.Now()
	cursor = model.ChainEventCursor{
		ChainID:    chainID,
		Contract:   contract,
		LastBlock:  0,
		CreateTime: now,
		UpdateTime: now,
	}
	if err := l.db.Create(&cursor).Error; err != nil {
		return nil, err
	}
	return &cursor, nil
}

// updateCursor 更新游标。
func (l *MarketplaceListener) updateCursor(block uint64) error {
	return l.db.Model(&model.ChainEventCursor{}).
		Where("chain_id = ? AND contract = ?", l.chainID.String(), strings.ToLower(l.contractAddr.Hex())).
		Updates(map[string]any{
			"last_block":  block,
			"update_time": time.Now(),
		}).Error
}

// isEventProcessed 判断事件是否已处理。
func (l *MarketplaceListener) isEventProcessed(vLog types.Log) (bool, error) {
	var count int64
	err := l.db.Model(&model.ChainEventLog{}).Where(
		"chain_id = ? AND contract = ? AND tx_hash = ? AND log_index = ?",
		l.chainID.String(),
		strings.ToLower(l.contractAddr.Hex()),
		vLog.TxHash.Hex(),
		vLog.Index,
	).Count(&count).Error
	return count > 0, err
}

// insertEventLog 记录最小化原始日志，用于去重标记与审计追踪。
func (l *MarketplaceListener) insertEventLog(vLog types.Log, eventName string) error {
	payloadObj := map[string]any{
		"address":     vLog.Address.Hex(),
		"blockNumber": vLog.BlockNumber,
		"txHash":      vLog.TxHash.Hex(),
		"logIndex":    vLog.Index,
		"topics":      topicsToHex(vLog.Topics),
	}
	// 记录链上事件。
	payload, _ := json.Marshal(payloadObj)
	// 创建链上事件记录。
	record := model.ChainEventLog{
		ChainID:     l.chainID.String(),
		Contract:    strings.ToLower(l.contractAddr.Hex()),
		EventName:   eventName,
		TxHash:      vLog.TxHash.Hex(),
		LogIndex:    uint(vLog.Index),
		BlockNumber: vLog.BlockNumber,
		Payload:     string(payload),
		CreateTime:  time.Now(),
	}
	return l.db.Create(&record).Error
}

// topicHex 将 topic 转换为十六进制字符串。
func topicHex(vLog types.Log, idx int) string {
	if len(vLog.Topics) <= idx {
		return ""
	}
	return vLog.Topics[idx].Hex()
}

// topicsToHex 将 topics 转换为十六进制字符串。
func topicsToHex(topics []common.Hash) []string {
	res := make([]string, 0, len(topics))
	for _, t := range topics {
		res = append(res, t.Hex())
	}
	return res
}
