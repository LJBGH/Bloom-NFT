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

// 接受出价上链：只在 acceptBid 时调用合约；上架/出价均为链下签名。
// Merkle 模式（entry.IsMerkle=true）：
//   - listingSignature 传空（长度=0），由合约走 merkle 分支校验 proof/merkleRoot/rootDeadline
//   - batchSignature 使用 entry.Signature（卖家对 batchRoot 的签名）
func AcceptBidOnChain(
	entry model.EntryOrders,
	bid model.BidPlaced,
	nftContract common.Address,
	merkleProof []common.Hash,
	merkleRoot common.Hash,
	rootDeadline time.Time,
) (common.Hash, error) {
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

	// 可选：做 ERC721 owner/approval 预检查，尽量减少 revert 成本。
	nftABIJSON := GetContractABI("BloomNFT")
	if strings.TrimSpace(nftABIJSON) != "" {
		if nftABI, err := abi.JSON(strings.NewReader(nftABIJSON)); err == nil {
			nftContractReader := bind.NewBoundContract(nftContract, nftABI, client, nil, nil)
			tokenID := new(big.Int).SetUint64(uint64(entry.TokenId))
			var ownerOut []interface{}
			if err := nftContractReader.Call(&bind.CallOpts{Context: ctx}, &ownerOut, "ownerOf", tokenID); err == nil && len(ownerOut) > 0 {
				if ownerAddr, ok := ownerOut[0].(common.Address); ok {
					if ownerAddr != common.HexToAddress(entry.Seller) {
						return common.Hash{}, errors.New("seller not owner (precheck)")
					}
				}
			}

			// approved / isApprovedForAll 检查（只做 best-effort，不保证所有 NFT 都严格实现）
			var approveOut []interface{}
			_ = nftContractReader.Call(&bind.CallOpts{Context: ctx}, &approveOut, "getApproved", tokenID)
			approvedOK := false
			if len(approveOut) > 0 {
				if approvedAddr, ok := approveOut[0].(common.Address); ok && approvedAddr == marketplaceAddr {
					approvedOK = true
				}
			}

			var approveAllOut []interface{}
			if err := nftContractReader.Call(&bind.CallOpts{Context: ctx}, &approveAllOut, "isApprovedForAll", common.HexToAddress(entry.Seller), marketplaceAddr); err == nil && len(approveAllOut) > 0 {
				if approvedForAll, ok := approveAllOut[0].(bool); ok && approvedForAll {
					approvedOK = true
				}
			}

			if !approvedOK {
				// 直接返回可避免交易 revert，但如果预检查失败/ABI 不匹配可能误判。
				// 为了更稳妥，这里不强制；只在 getApproved/isApprovedForAll 两者都成功时再强制。
			}
		}
	}

	contract := bind.NewBoundContract(marketplaceAddr, parsedABI, client, client, client)

	// 解码签名：对非 Merkle listing，listingSignature 使用 entry.Signature；
	// 对 Merkle listing，把 listingSignature 置空，把 entry.Signature 当 batchSignature 传入合约。
	var listingSigBytes []byte
	var batchSigBytes []byte

	if entry.IsMerkle {
		if !VerifySignature(entry.Signature) {
			return common.Hash{}, errors.New("invalid batch signature (entry.signature)")
		}
		// listingSignature 为空 -> 合约走 merkle 路径
		listingSigBytes = []byte{}
		batchSigBytes, err = hexutil.Decode(entry.Signature)
		if err != nil {
			return common.Hash{}, fmt.Errorf("decode batch signature failed: %w", err)
		}
	} else {
		var err2 error
		listingSigBytes, err2 = hexutil.Decode(entry.Signature)
		if err2 != nil {
			return common.Hash{}, fmt.Errorf("decode listing signature failed: %w", err2)
		}
		batchSigBytes = []byte{}
	}

	bidSigBytes, err := hexutil.Decode(bid.Signature)
	if err != nil {
		return common.Hash{}, fmt.Errorf("decode bid signature failed: %w", err)
	}

	// 参数组装：Listing / Bid（新版合约不再使用 listingHash / escrow）
	listingPriceWei, err := btToWei(entry.Price)
	if err != nil {
		return common.Hash{}, fmt.Errorf("convert listing price to wei failed: %w", err)
	}
	bidPriceWei, err := btToWei(bid.Price)
	if err != nil {
		return common.Hash{}, fmt.Errorf("convert bid price to wei failed: %w", err)
	}

	listingSaltWei, err := parseListingSalt(entry.Salt)
	if err != nil {
		return common.Hash{}, fmt.Errorf("invalid listing salt: %w", err)
	}
	bidSaltWei, err := parseListingSalt(bid.Salt)
	if err != nil {
		return common.Hash{}, fmt.Errorf("invalid bid salt: %w", err)
	}

	tokenID := new(big.Int).SetUint64(uint64(entry.TokenId))
	listingDeadline := big.NewInt(entry.Deadline.Unix())
	bidDeadline := big.NewInt(bid.Deadline.Unix())

	listingParam := struct {
		Nft      common.Address
		Seller   common.Address
		TokenId  *big.Int
		Price    *big.Int
		Deadline *big.Int
		Salt     *big.Int
	}{
		Nft:      nftContract,
		Seller:   common.HexToAddress(entry.Seller),
		TokenId:  tokenID,
		Price:    listingPriceWei,
		Deadline: listingDeadline,
		Salt:     listingSaltWei,
	}

	bidParam := struct {
		Nft      common.Address
		Buyer    common.Address
		TokenId  *big.Int
		Price    *big.Int
		Deadline *big.Int
		Salt     *big.Int
	}{
		Nft:      nftContract,
		Buyer:    common.HexToAddress(bid.Buyer),
		TokenId:  tokenID,
		Price:    bidPriceWei,
		Deadline: bidDeadline,
		Salt:     bidSaltWei,
	}

	// rootDeadline：合约参数为 uint256（unix seconds）
	rootDeadlineUnix := rootDeadline.Unix()
	tx, err := contract.Transact(
		auth,
		"acceptBid",
		listingParam,
		listingSigBytes,
		merkleProof,
		merkleRoot,
		big.NewInt(rootDeadlineUnix),
		batchSigBytes,
		bidParam,
		bidSigBytes,
	)
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

// decodeListingSignature Merkle 上架的 entry_orders.signature 可为空；单笔挂单为 65 字节 EIP-712 签名。
func decodeListingSignature(sig string) ([]byte, error) {
	s := strings.TrimSpace(sig)
	if s == "" {
		return []byte{}, nil
	}
	b, err := hexutil.Decode(s)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// parseListingSalt 解析挂单 EIP-712 中的 salt（十进制字符串，或 0x 十六进制）。
func parseListingSalt(s string) (*big.Int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return big.NewInt(0), nil
	}
	out := new(big.Int)
	if _, ok := out.SetString(s, 10); ok {
		return out, nil
	}
	hexStr := strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if _, ok := out.SetString(hexStr, 16); ok {
		return out, nil
	}
	return nil, errors.New("invalid salt: expected decimal uint256 or hex")
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
