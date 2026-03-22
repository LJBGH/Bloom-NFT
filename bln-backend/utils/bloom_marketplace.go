package utils

import (
	"bloom-nft/config"
	"bloom-nft/model"
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	log "github.com/sirupsen/logrus"
)

var rpcUrl string = config.AppConfig.NetWork.RpcUrl
var contractName string = "BloomMarketplace"

// 验证签名
func VerifySignature(signature string) bool {
	sigBytes, err := hexutil.Decode(signature)
	if err != nil {
		return false
	}
	return len(sigBytes) == 65
}

// ListWithSigOnChain 执行 BloomMarketplace.listWithSig 上链调用。
// 需要卖家已提前 approve NFT 给 marketplace 合约。
func ListWithSigOnChain(entry model.EntryOrders, nftContract common.Address) (common.Hash, error) {
	// 打印入口关键参数，便于按请求维度排查
	log.WithFields(log.Fields{
		"seller":      entry.Seller,
		"tokenId":     entry.TokenId,
		"nftListId":   entry.NftListID,
		"price":       entry.Price,
		"deadline":    entry.Deadline,
		"nonce":       entry.Nonce,
		"nftContract": nftContract.Hex(),
	}).Info("开始执行 listWithSig 上链流程")

	// 第 1 步：校验签名格式
	if !VerifySignature(entry.Signature) {
		log.Error("签名校验失败：signature 不是 65 字节十六进制")
		return common.Hash{}, errors.New("invalid signature")
	}

	// 第 2 步：准备 RPC 地址
	if strings.TrimSpace(rpcUrl) == "" {
		rpcUrl = strings.TrimSpace(config.AppConfig.NetWork.RpcUrl)
	}
	if strings.TrimSpace(rpcUrl) == "" {
		log.Error("RPC 地址为空：请检查配置文件 NetWork.RpcUrl")
		return common.Hash{}, errors.New("rpc url is empty")
	}
	log.WithField("rpcUrl", rpcUrl).Info("RPC 地址已就绪")

	// 第 3 步：连接链节点
	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		log.WithError(err).Error("连接 RPC 节点失败")
		return common.Hash{}, fmt.Errorf("dial rpc failed (rpc=%s): %w", rpcUrl, err)
	}
	defer client.Close()
	log.Info("RPC 节点连接成功")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 第 4 步：读取链 ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		log.WithError(err).Error("读取链 ID 失败")
		return common.Hash{}, fmt.Errorf("get chain id failed: %w", err)
	}
	log.WithField("chainID", chainID.String()).Info("链 ID 读取成功")

	// 第 5 步：读取并校验 marketplace 合约地址
	marketplaceAddrHex := GetContractAddress(contractName)
	if marketplaceAddrHex == "" {
		log.Error("未找到 BloomMarketplace 合约地址配置")
		return common.Hash{}, errors.New("BloomMarketplace address not found")
	}
	marketplaceAddr := common.HexToAddress(marketplaceAddrHex)
	if marketplaceAddr == (common.Address{}) {
		log.WithField("marketplaceAddr", marketplaceAddrHex).Error("BloomMarketplace 合约地址非法")
		return common.Hash{}, errors.New("invalid BloomMarketplace address")
	}
	log.WithField("marketplaceAddr", marketplaceAddr.Hex()).Info("BloomMarketplace 地址校验通过")

	// 第 6 步：读取并解析 ABI
	abiJSON := GetContractABI(contractName)
	if strings.TrimSpace(abiJSON) == "" {
		log.Error("未找到 BloomMarketplace ABI 配置")
		return common.Hash{}, errors.New("BloomMarketplace ABI not found")
	}
	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		log.WithError(err).Error("解析 BloomMarketplace ABI 失败")
		return common.Hash{}, fmt.Errorf("parse BloomMarketplace ABI failed: %w", err)
	}
	log.Info("BloomMarketplace ABI 解析成功")

	// 第 7 步：加载私钥并创建交易签名器
	privateKeyHex := strings.TrimPrefix(strings.TrimSpace(config.AppConfig.NetWork.AccountPrivateKey), "0x")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		log.WithError(err).Error("解析账户私钥失败")
		return common.Hash{}, fmt.Errorf("parse private key failed: %w", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		log.WithError(err).Error("创建交易签名器失败")
		return common.Hash{}, fmt.Errorf("create transactor failed: %w", err)
	}
	auth.Context = ctx
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(6_000_000)
	log.WithField("gasLimit", auth.GasLimit).Info("交易签名器初始化完成")

	// 第 8 步：解码签名
	sigBytes, err := hexutil.Decode(entry.Signature)
	if err != nil {
		log.WithError(err).Error("签名解码失败")
		return common.Hash{}, fmt.Errorf("decode signature failed: %w", err)
	}
	log.WithField("signatureBytesLen", len(sigBytes)).Info("签名解码成功")

	// 第 9 步：价格转 Wei
	priceWei, err := btToWei(entry.Price)
	if err != nil {
		log.WithError(err).Error("价格转换 Wei 失败")
		return common.Hash{}, fmt.Errorf("convert price to wei failed: %w", err)
	}
	log.WithField("priceWei", priceWei.String()).Info("价格转换 Wei 成功")

	listing := struct {
		Nft      common.Address
		Seller   common.Address
		TokenId  *big.Int
		Price    *big.Int
		Deadline *big.Int
		Nonce    *big.Int
	}{
		Nft:      nftContract,
		Seller:   common.HexToAddress(entry.Seller),
		TokenId:  new(big.Int).SetUint64(uint64(entry.TokenId)),
		Price:    priceWei,
		Deadline: big.NewInt(entry.Deadline.Unix()),
		Nonce:    big.NewInt(int64(entry.Nonce)),
	}
	log.WithFields(log.Fields{
		"listing.nft":      listing.Nft.Hex(),
		"listing.seller":   listing.Seller.Hex(),
		"listing.tokenId":  listing.TokenId.String(),
		"listing.priceWei": listing.Price.String(),
		"listing.deadline": listing.Deadline.String(),
		"listing.nonce":    listing.Nonce.String(),
	}).Info("listWithSig 参数组装完成")

	// 第 10 步：发送 listWithSig 交易
	contract := bind.NewBoundContract(marketplaceAddr, parsedABI, client, client, client)

	// 第 10.1 步：先做只读调用 token()，确认合约可正常响应
	var tokenOut []interface{}
	if err := contract.Call(&bind.CallOpts{Context: ctx}, &tokenOut, "token"); err != nil {
		log.WithError(err).Error("调用 BloomMarketplace.token() 失败")
		return common.Hash{}, fmt.Errorf("preflight call token() failed (marketplace=%s): %w", marketplaceAddr.Hex(), err)
	}
	if len(tokenOut) == 0 {
		log.Error("调用 BloomMarketplace.token() 返回为空")
		return common.Hash{}, fmt.Errorf("preflight call token() returned empty (marketplace=%s)", marketplaceAddr.Hex())
	}
	tokenAddr, ok := tokenOut[0].(common.Address)
	if !ok {
		log.WithField("actualType", fmt.Sprintf("%T", tokenOut[0])).Error("token() 返回类型异常")
		return common.Hash{}, fmt.Errorf("preflight call token() unexpected return type: %T", tokenOut[0])
	}
	log.WithField("tokenAddress", tokenAddr.Hex()).Info("调用 BloomMarketplace.token() 成功")

	// 第 10.2 步：读取 marketplace.nonces(seller)，检查 nonce 是否匹配
	var nonceOut []interface{}
	if err := contract.Call(&bind.CallOpts{Context: ctx}, &nonceOut, "nonces", listing.Seller); err != nil {
		log.WithError(err).Error("读取 marketplace.nonces(seller) 失败")
	} else if len(nonceOut) > 0 {
		if onChainNonce, ok := nonceOut[0].(*big.Int); ok {
			log.WithFields(log.Fields{
				"seller":       listing.Seller.Hex(),
				"requestNonce": listing.Nonce.String(),
				"onChainNonce": onChainNonce.String(),
				"nonceMatched": onChainNonce.Cmp(listing.Nonce) == 0,
			}).Info("读取 marketplace.nonces(seller) 成功")
		} else {
			log.WithField("actualType", fmt.Sprintf("%T", nonceOut[0])).Warn("marketplace.nonces(seller) 返回类型异常")
		}
	}

	// 第 10.3 步：读取 NFT owner/approve 信息，检查是否具备转移权限
	nftABIJSON := GetContractABI("BloomNFT")
	if strings.TrimSpace(nftABIJSON) == "" {
		log.Warn("未找到 BloomNFT ABI，跳过 owner/approve 预检查")
	} else {
		nftABI, err := abi.JSON(strings.NewReader(nftABIJSON))
		if err != nil {
			log.WithError(err).Warn("解析 BloomNFT ABI 失败，跳过 owner/approve 预检查")
		} else {
			nftContractReader := bind.NewBoundContract(listing.Nft, nftABI, client, nil, nil)

			var ownerOut []interface{}
			if err := nftContractReader.Call(&bind.CallOpts{Context: ctx}, &ownerOut, "ownerOf", listing.TokenId); err != nil {
				log.WithError(err).Error("调用 NFT.ownerOf(tokenId) 失败")
			} else if len(ownerOut) > 0 {
				if owner, ok := ownerOut[0].(common.Address); ok {
					log.WithFields(log.Fields{
						"tokenId":       listing.TokenId.String(),
						"ownerOfToken":  owner.Hex(),
						"listingSeller": listing.Seller.Hex(),
						"ownerMatched":  strings.EqualFold(owner.Hex(), listing.Seller.Hex()),
					}).Info("调用 NFT.ownerOf(tokenId) 成功")
				} else {
					log.WithField("actualType", fmt.Sprintf("%T", ownerOut[0])).Warn("NFT.ownerOf(tokenId) 返回类型异常")
				}
			}

			var approvedOut []interface{}
			if err := nftContractReader.Call(&bind.CallOpts{Context: ctx}, &approvedOut, "getApproved", listing.TokenId); err != nil {
				log.WithError(err).Error("调用 NFT.getApproved(tokenId) 失败")
			} else if len(approvedOut) > 0 {
				if approved, ok := approvedOut[0].(common.Address); ok {
					log.WithFields(log.Fields{
						"tokenId":         listing.TokenId.String(),
						"approvedAddress": approved.Hex(),
						"marketplaceAddr": marketplaceAddr.Hex(),
						"approvedMatched": strings.EqualFold(approved.Hex(), marketplaceAddr.Hex()),
					}).Info("调用 NFT.getApproved(tokenId) 成功")
				} else {
					log.WithField("actualType", fmt.Sprintf("%T", approvedOut[0])).Warn("NFT.getApproved(tokenId) 返回类型异常")
				}
			}

			var approveAllOut []interface{}
			if err := nftContractReader.Call(&bind.CallOpts{Context: ctx}, &approveAllOut, "isApprovedForAll", listing.Seller, marketplaceAddr); err != nil {
				log.WithError(err).Error("调用 NFT.isApprovedForAll(seller, marketplace) 失败")
			} else if len(approveAllOut) > 0 {
				if approvedForAll, ok := approveAllOut[0].(bool); ok {
					log.WithFields(log.Fields{
						"seller":         listing.Seller.Hex(),
						"marketplace":    marketplaceAddr.Hex(),
						"approvedForAll": approvedForAll,
					}).Info("调用 NFT.isApprovedForAll(seller, marketplace) 成功")
				} else {
					log.WithField("actualType", fmt.Sprintf("%T", approveAllOut[0])).Warn("NFT.isApprovedForAll 返回类型异常")
				}
			}
		}
	}

	// 第 10.4 步：先做 listWithSig 的只读预执行，尽量拿到更明确的失败原因
	var preflightOut []interface{}
	if err := contract.Call(&bind.CallOpts{Context: ctx}, &preflightOut, "listWithSig", listing, sigBytes); err != nil {
		log.WithError(err).Error("listWithSig 只读预执行失败（通常可反映 require 校验失败）")
	} else {
		log.WithField("outputLen", len(preflightOut)).Info("listWithSig 只读预执行成功")
	}

	tx, err := contract.Transact(auth, "listWithSig", listing, sigBytes)
	if err != nil {
		log.WithError(err).Error("发送 listWithSig 交易失败")
		return common.Hash{}, fmt.Errorf("call listWithSig transact failed (marketplace=%s, seller=%s, tokenId=%d): %w", marketplaceAddr.Hex(), entry.Seller, entry.TokenId, err)
	}
	log.WithField("txHash", tx.Hash().Hex()).Info("listWithSig 交易已发出")

	// 第 11 步：等待交易上链
	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		log.WithError(err).Error("等待交易上链失败")
		return tx.Hash(), fmt.Errorf("wait tx mined failed (txHash=%s): %w", tx.Hash().Hex(), err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		log.WithFields(log.Fields{
			"txHash": tx.Hash().Hex(),
			"status": receipt.Status,
		}).Error("交易已上链但执行失败（reverted）")
		return tx.Hash(), errors.New("listWithSig transaction reverted")
	}
	log.WithFields(log.Fields{
		"txHash": tx.Hash().Hex(),
		"status": receipt.Status,
	}).Info("listWithSig 上链成功")
	return tx.Hash(), nil
}

// 出价上链（注意：合约要求 msg.sender == bid.buyer）
func BidWithSigOnChain(bid model.BidPlaced, listingHash common.Hash) (common.Hash, error) {
	if !VerifySignature(bid.Signature) {
		return common.Hash{}, errors.New("invalid bid signature")
	}
	rpc := strings.TrimSpace(config.AppConfig.NetWork.RpcUrl)
	if rpc == "" {
		return common.Hash{}, errors.New("rpc url is empty")
	}
	client, err := ethclient.Dial(rpc)
	if err != nil {
		return common.Hash{}, fmt.Errorf("dial rpc failed: %w", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("get chain id failed: %w", err)
	}
	privateKeyHex := strings.TrimPrefix(strings.TrimSpace(config.AppConfig.NetWork.AccountPrivateKey), "0x")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return common.Hash{}, fmt.Errorf("parse private key failed: %w", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return common.Hash{}, fmt.Errorf("create transactor failed: %w", err)
	}
	auth.Context = ctx
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(6_000_000)

	marketplaceAddrHex := GetContractAddress(contractName)
	if marketplaceAddrHex == "" {
		return common.Hash{}, errors.New("BloomMarketplace address not found")
	}
	marketplaceAddr := common.HexToAddress(marketplaceAddrHex)
	abiJSON := GetContractABI(contractName)
	if strings.TrimSpace(abiJSON) == "" {
		return common.Hash{}, errors.New("BloomMarketplace ABI not found")
	}
	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return common.Hash{}, fmt.Errorf("parse BloomMarketplace ABI failed: %w", err)
	}

	priceWei, err := btToWei(bid.Price)
	if err != nil {
		return common.Hash{}, fmt.Errorf("convert bid price to wei failed: %w", err)
	}
	sigBytes, err := hexutil.Decode(bid.Signature)
	if err != nil {
		return common.Hash{}, fmt.Errorf("decode bid signature failed: %w", err)
	}

	bidParam := struct {
		ListingHash common.Hash
		Buyer       common.Address
		Price       *big.Int
		Deadline    *big.Int
		Nonce       *big.Int
	}{
		ListingHash: listingHash,
		Buyer:       common.HexToAddress(bid.Buyer),
		Price:       priceWei,
		Deadline:    big.NewInt(bid.Deadline.Unix()),
		Nonce:       big.NewInt(int64(bid.Nonce)),
	}

	contract := bind.NewBoundContract(marketplaceAddr, parsedABI, client, client, client)
	tx, err := contract.Transact(auth, "bidWithSig", bidParam, sigBytes)
	if err != nil {
		return common.Hash{}, fmt.Errorf("call bidWithSig transact failed: %w", err)
	}
	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		return tx.Hash(), fmt.Errorf("wait bidWithSig mined failed: %w", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return tx.Hash(), errors.New("bidWithSig transaction reverted")
	}
	return tx.Hash(), nil
}

// 接受出价上链（注意：合约要求 msg.sender == listing.seller）
func AcceptBidOnChain(entry model.EntryOrders, bid model.BidPlaced, nftContract common.Address, listingHash common.Hash) (common.Hash, error) {
	if !VerifySignature(entry.Signature) {
		return common.Hash{}, errors.New("invalid listing signature")
	}
	if !VerifySignature(bid.Signature) {
		return common.Hash{}, errors.New("invalid bid signature")
	}

	rpc := strings.TrimSpace(config.AppConfig.NetWork.RpcUrl)
	if rpc == "" {
		return common.Hash{}, errors.New("rpc url is empty")
	}
	client, err := ethclient.Dial(rpc)
	if err != nil {
		return common.Hash{}, fmt.Errorf("dial rpc failed: %w", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("get chain id failed: %w", err)
	}
	privateKeyHex := strings.TrimPrefix(strings.TrimSpace(config.AppConfig.NetWork.AccountPrivateKey), "0x")
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return common.Hash{}, fmt.Errorf("parse private key failed: %w", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return common.Hash{}, fmt.Errorf("create transactor failed: %w", err)
	}
	auth.Context = ctx
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(6_000_000)

	marketplaceAddrHex := GetContractAddress(contractName)
	if marketplaceAddrHex == "" {
		return common.Hash{}, errors.New("BloomMarketplace address not found")
	}
	marketplaceAddr := common.HexToAddress(marketplaceAddrHex)
	abiJSON := GetContractABI(contractName)
	if strings.TrimSpace(abiJSON) == "" {
		return common.Hash{}, errors.New("BloomMarketplace ABI not found")
	}
	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return common.Hash{}, fmt.Errorf("parse BloomMarketplace ABI failed: %w", err)
	}

	listingPriceWei, err := btToWei(entry.Price)
	if err != nil {
		return common.Hash{}, fmt.Errorf("convert listing price to wei failed: %w", err)
	}
	bidPriceWei, err := btToWei(bid.Price)
	if err != nil {
		return common.Hash{}, fmt.Errorf("convert bid price to wei failed: %w", err)
	}
	listingSigBytes, err := hexutil.Decode(entry.Signature)
	if err != nil {
		return common.Hash{}, fmt.Errorf("decode listing signature failed: %w", err)
	}
	bidSigBytes, err := hexutil.Decode(bid.Signature)
	if err != nil {
		return common.Hash{}, fmt.Errorf("decode bid signature failed: %w", err)
	}

	listingParam := struct {
		Nft      common.Address
		Seller   common.Address
		TokenId  *big.Int
		Price    *big.Int
		Deadline *big.Int
		Nonce    *big.Int
	}{
		Nft:      nftContract,
		Seller:   common.HexToAddress(entry.Seller),
		TokenId:  new(big.Int).SetUint64(uint64(entry.TokenId)),
		Price:    listingPriceWei,
		Deadline: big.NewInt(entry.Deadline.Unix()),
		Nonce:    big.NewInt(int64(entry.Nonce)),
	}
	bidParam := struct {
		ListingHash common.Hash
		Buyer       common.Address
		Price       *big.Int
		Deadline    *big.Int
		Nonce       *big.Int
	}{
		ListingHash: listingHash,
		Buyer:       common.HexToAddress(bid.Buyer),
		Price:       bidPriceWei,
		Deadline:    big.NewInt(bid.Deadline.Unix()),
		Nonce:       big.NewInt(int64(bid.Nonce)),
	}

	contract := bind.NewBoundContract(marketplaceAddr, parsedABI, client, client, client)
	tx, err := contract.Transact(auth, "acceptBid", listingParam, listingSigBytes, bidParam, bidSigBytes)
	if err != nil {
		return common.Hash{}, fmt.Errorf("call acceptBid transact failed: %w", err)
	}
	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		return tx.Hash(), fmt.Errorf("wait acceptBid mined failed: %w", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return tx.Hash(), errors.New("acceptBid transaction reverted")
	}
	return tx.Hash(), nil
}

// 把浮点价格字符串转成 18 位最小单位
func btToWei(bt float64) (*big.Int, error) {
	// 与前端 parseUnits(String(priceNum), 18) 对齐：把浮点价格字符串转成 18 位最小单位
	btStr := strconv.FormatFloat(bt, 'f', -1, 64)
	rat, ok := new(big.Rat).SetString(btStr)
	if !ok {
		return nil, errors.New("invalid price format")
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	rat.Mul(rat, new(big.Rat).SetInt(scale))
	out := new(big.Int).Quo(rat.Num(), rat.Denom()) // floor
	if out.Sign() <= 0 {
		return nil, errors.New("price must be > 0")
	}
	return out, nil
}
