package request

// WalletImportRequest 钱包导入 JSON 文件内容
// 约定字段名与 WalletResult 保持一致，方便直接透传/映射
type WalletImportRequest struct {
	Mnemonic   string `json:"mnemonic"`   // 助记词
	PrivateKey string `json:"privateKey"` // 私钥
}
