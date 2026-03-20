package model

// WalletAccount 单个账户信息（派生路径下的一个 address）
type WalletAccount struct {
	Index      int    `json:"index"`
	Path       string `json:"path"`
	Name       string `json:"name"`
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
	Address    string `json:"address"`

	// 仅用于接口返回（wallet.json 的 accounts 节点不会写入）
	Mnemonic string `json:"mnemonic,omitempty"`
}

// WalletStore 本地 wallet.json 持久化结构
type WalletStore struct {
	Mnemonic            string          `json:"mnemonic"`
	CurrentAccountIndex int             `json:"currentAccountIndex"`
	Accounts            []WalletAccount `json:"accounts"`
}

