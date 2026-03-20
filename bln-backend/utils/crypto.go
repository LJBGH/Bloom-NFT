package utils

import (
	"crypto/ecdsa"
	"fmt"
	"strings"

	"bloom-nft/model"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
	"github.com/tyler-smith/go-bip39"
)

// HD 钱包（分层确定性钱包） 的“派生路径”
// 同一份助记词怎么“派生”出某一个具体的账户私钥/地址
// 不同路径会得到不同的私钥/地址（但都来自同一助记词）
const defaultDerivationPath = "m/44'/60'/0'/0/0"

// CreateMnemonic 生成 BIP39 助记词（默认 12 个词）
func CreateMnemonic() (string, error) {
	entropy, err := bip39.NewEntropy(128) // 12 words
	if err != nil {
		return "", err
	}
	return bip39.NewMnemonic(entropy)
}

// CreateWallet 创建钱包：
// - 传入 mnemonic 为空字符串：自动生成助记词并派生第一个地址
// - 传入 mnemonic 非空：用该助记词派生第一个地址
func CreateWallet(mnemonic string) (*model.WalletInfo, error) {
	if mnemonic == "" {
		var err error
		mnemonic, err = CreateMnemonic()
		if err != nil {
			return nil, err
		}
	}
	return WalletFromMnemonic(mnemonic, defaultDerivationPath)
}

// WalletFromMnemonic 从助记词派生钱包信息（ETH: m/44'/60'/0'/0/0）
func WalletFromMnemonic(mnemonic string, derivationPath string) (*model.WalletInfo, error) {
	// 第 1 步：校验助记词是否合法（单词拼写、顺序、校验和）
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic")
	}
	// 第 2 步：如果没有传自定义路径，使用默认 ETH 路径 m/44'/60'/0'/0/0
	if derivationPath == "" {
		derivationPath = defaultDerivationPath
	}

	// 第 3 步：根据助记词创建 HD 钱包“根节点”（内部会先从助记词算出 seed）
	wallet, err := hdwallet.NewFromMnemonic(mnemonic)
	if err != nil {
		return nil, err
	}
	// 第 4 步：解析派生路径字符串，例如 m/44'/60'/0'/0/0
	path := hdwallet.MustParseDerivationPath(derivationPath)

	// 第 5 步：按照路径从根节点派生出具体 account（得到该路径对应的“子私钥”）
	account, err := wallet.Derive(path, false)
	if err != nil {
		return nil, err
	}
	// 第 6 步：从 account 中取出 ECDSA 私钥对象
	privateKey, err := wallet.PrivateKey(account)
	if err != nil {
		return nil, err
	}

	// 第 7 步：由私钥推导出公钥，并转换为以太坊地址
	publicKeyECDSA, ok := privateKey.Public().(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not *ecdsa.PublicKey")
	}

	// 第 8 步：把私钥、公钥、地址都编码为常见的 Hex/字符串形式，方便前端或持久化使用
	privateKeyHex := hexutil.Encode(crypto.FromECDSA(privateKey))
	publicKeyHex := hexutil.Encode(crypto.FromECDSAPub(publicKeyECDSA))
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()

	return &model.WalletInfo{
		Mnemonic:   mnemonic,
		Address:    address,
		PrivateKey: privateKeyHex,
		PublicKey:  publicKeyHex,
	}, nil
}

// CreateRandomWallet 直接生成随机私钥的钱包（不使用助记词）
// 适用于只想要“单个随机地址 + 私钥”，不需要 HD 钱包/助记词恢复能力的场景
func CreateRandomWallet() (*model.WalletInfo, error) {
	// 生成一个新的 ECDSA 私钥（secp256k1 曲线，与以太坊一致）
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}

	publicKeyECDSA, ok := privateKey.Public().(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not *ecdsa.PublicKey")
	}

	privateKeyHex := hexutil.Encode(crypto.FromECDSA(privateKey))
	publicKeyHex := hexutil.Encode(crypto.FromECDSAPub(publicKeyECDSA))
	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()

	return &model.WalletInfo{
		// 不使用助记词，这里留空即可
		Mnemonic:   "",
		Address:    address,
		PrivateKey: privateKeyHex,
		PublicKey:  publicKeyHex,
	}, nil
}

// WalletFromPrivateKeyHex 从私钥(hex)构造钱包信息（不包含助记词）
func WalletFromPrivateKeyHex(privateKeyHex string) (*model.WalletInfo, error) {
	privateKeyHex = strings.TrimSpace(privateKeyHex)
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0X")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, err
	}
	publicKeyECDSA, ok := privateKey.Public().(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not *ecdsa.PublicKey")
	}

	return &model.WalletInfo{
		Address:    crypto.PubkeyToAddress(*publicKeyECDSA).Hex(),
		PrivateKey: hexutil.Encode(crypto.FromECDSA(privateKey)),
		PublicKey:  hexutil.Encode(crypto.FromECDSAPub(publicKeyECDSA)),
	}, nil
}
