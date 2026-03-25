package listener

import (
	"bloom-nft/config"
	"bloom-nft/database"
	"bloom-nft/enums"
	"bloom-nft/model"
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// StartMarketplaceExpirationWorker 轮询数据库，把过期挂单/出价从 Pending 更新为 Expired。
// 说明：该 worker 不与合约交互，只依赖数据库里已经落库的 deadline 字段。
func StartMarketplaceExpirationWorker(ctx context.Context) error {
	// 沿用 Listener.Enabled 开关：你可以后续再加独立配置项再拆开。
	if !config.AppConfig.Listener.Enabled {
		log.Info("expiration worker disabled by config")
		return nil
	}

	// 获取轮询间隔。
	pollSec := config.AppConfig.Listener.PollIntervalSec
	if pollSec <= 0 {
		pollSec = 5
	}

	// 创建轮询器。
	ticker := time.NewTicker(time.Duration(pollSec) * time.Second)
	defer ticker.Stop()

	// 初始化同步。
	log.WithFields(log.Fields{
		"pollSecond": pollSec,
	}).Info("marketplace expiration worker started")

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := expireOnce(); err != nil {
				log.WithError(err).Error("expireOnce failed")
			}
		}
	}
}

// expireOnce 处理一次过期。
func expireOnce() error {
	now := time.Now()

	return database.DB.Transaction(func(tx *gorm.DB) error {
		// 1) 挂单过期：ListingPending -> ListingExpired
		listRes := tx.Model(&model.EntryOrders{}).
			Where("status = ? AND deadline <= ?", enums.ListingPending, now).
			Updates(map[string]any{
				"status":      enums.ListingExpired,
				"update_time": now,
			})
		if listRes.Error != nil {
			return listRes.Error
		}

		// 2) 出价过期：BidPending -> BidExpired
		// 只处理挂单处于 Pending/Expired 的出价，避免污染 completed/cancelled 数据。
		bidRes := tx.Exec(`
			UPDATE orders_bid b
			JOIN orders_entry o ON o.id = b.orders_id
			SET b.status = ?, b.update_time = ?
			WHERE b.status = ?
				AND o.status IN (?, ?)
				AND (b.deadline <= ? OR o.deadline <= ?)
		`, enums.BidExpired, now, enums.BidPending, enums.ListingPending, enums.ListingExpired, now, now)
		if bidRes.Error != nil {
			return bidRes.Error
		}

		if listRes.RowsAffected > 0 || bidRes.RowsAffected > 0 {
			log.WithFields(log.Fields{
				"listingExpired": listRes.RowsAffected,
				"bidExpired":     bidRes.RowsAffected,
				"now":            now.Format(time.RFC3339),
			}).Info("marketplace expiration updated")
		}

		return nil
	})
}
