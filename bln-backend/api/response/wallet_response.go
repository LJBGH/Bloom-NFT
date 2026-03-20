package response

import "bloom-nft/model"

type WalletResult struct {
	model.WalletInfo
}

type WalletAccountResult struct {
	model.WalletAccount
}

type WalletAccountsResult struct {
	Accounts []model.WalletAccount `json:"accounts"`
}
