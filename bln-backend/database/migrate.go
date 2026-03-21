package database

import "bloom-nft/model"

func MigrateModels() []interface{} {
	return []interface{}{
		&model.Nft{},
		&model.NftList{},
		&model.EntryOrders{},
		&model.BidPlaced{},
		&model.ChainEventCursor{},
		&model.ChainEventLog{},
		&model.ChainRefEntryOrder{},
		&model.ChainRefBid{},
	}
}
