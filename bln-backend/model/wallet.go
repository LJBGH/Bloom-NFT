package model

type WalletInfo struct {
	Mnemonic   string `json:"mnemonic"`   // 助记词
	PrivateKey string `json:"privateKey"` // 私钥
	PublicKey  string `json:"publicKey"`  // 公钥
	Address    string `json:"address"`    // 地址
}
