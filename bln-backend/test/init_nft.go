package main

import (
	"bloom-nft/api/request"
	"bloom-nft/config"
	"bloom-nft/database"
	"bloom-nft/model"
	"bloom-nft/repository"
	"bloom-nft/services"
	"bloom-nft/utils"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type FileInfo struct {
	request  request.MintRequest
	fileData io.Reader
}

var rpcUrl string = "http://127.0.0.1:8545"
var accountProviteKey string = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
var abiPath string = "../abi/BloomNFT.json"

func InitNft() {
	config.LoadConfig()
	database.InitializeDB("")
	database.AutoMigrate()
	db := database.DB

	nftReposity := repository.NewNftRepository(db)
	nftListRepository := repository.NewNftListRepository(db)

	nftService := services.NewNftService(nftReposity, nftListRepository)
	nftContractAddress := utils.GetContractAddress("BloomNFT")
	if nftContractAddress == "" {
		panic("nft contract address is empty")
	}

	// 先插入NFT类目
	nftReposity.Insert(model.Nft{
		ID:          1,
		Name:        "Bloom-NFT",
		Description: "Bloom-NFT",
		Address:     nftContractAddress,
		CreateTime:  time.Now(),
		UpdateTime:  time.Now(),
	})

	// 读取文件
	files, err := getFiles()
	if err != nil {
		panic(err)
	}

	// 准备链上 mint 合约调用（只做一次）
	rpcUrl = strings.TrimSpace(rpcUrl)
	if config.AppConfig.NetWork.RpcUrl != "" {
		rpcUrl = config.AppConfig.NetWork.RpcUrl
	}
	if config.AppConfig.NetWork.AccountPrivateKey != "" {
		accountProviteKey = config.AppConfig.NetWork.AccountPrivateKey
	}

	ctx := context.Background()

	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		panic(fmt.Sprintf("ethclient dial failed: %v", err))
	}
	defer client.Close()

	contractAddr := common.HexToAddress(nftContractAddress)

	abiBytes, err := os.ReadFile(abiPath)
	if err != nil {
		// 兼容相对路径在不同运行目录下不一致的情况
		alt := filepath.Join("..", "abi", filepath.Base(abiPath))
		abiBytes2, err2 := os.ReadFile(alt)
		if err2 != nil {
			panic(fmt.Sprintf("read abi failed: %v (alt=%s err=%v)", err, alt, err2))
		}
		abiBytes = abiBytes2
	}

	parsedABI, err := parseContractABI(abiBytes)
	if err != nil {
		panic(fmt.Sprintf("parse abi failed: %v", err))
	}

	// auth：发送交易所用的签名
	privateKeyStr := strings.TrimPrefix(strings.TrimSpace(accountProviteKey), "0x")
	privateKeyBytes, err := hex.DecodeString(privateKeyStr)
	if err != nil {
		panic(fmt.Sprintf("invalid private key: %v", err))
	}
	privateKey, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		panic(fmt.Sprintf("to ecdsa failed: %v", err))
	}

	chainID, err := client.ChainID(ctx)
	if err != nil {
		panic(fmt.Sprintf("get chainID failed: %v", err))
	}

	toAddr := crypto.PubkeyToAddress(privateKey.PublicKey)
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		panic(fmt.Sprintf("new keyed transactor failed: %v", err))
	}
	auth.Context = ctx
	auth.GasLimit = uint64(6_000_000) // mint 给更保守一点的 gas，减少 under-gas 的概率

	// 显式设置 GasPrice（legacy），避免部分链在 EIP-1559 模式下的 fee 估算导致 RPC 内部错误
	if gp, err := client.SuggestGasPrice(ctx); err == nil {
		auth.GasPrice = gp
	}

	contract := bind.NewBoundContract(contractAddr, parsedABI, client, client, client)
	transferEventID := parsedABI.Events["Transfer"].ID
	mintEventID := parsedABI.Events["Mint"].ID

	// 幂等保护：避免超过 maxSupply 导致 estimateGas / mint 失败
	var totalSupplyOut []any
	if err := contract.Call(&bind.CallOpts{Context: ctx}, &totalSupplyOut, "totalSupply"); err != nil {
		panic(fmt.Sprintf("call totalSupply failed: %v", err))
	}
	if len(totalSupplyOut) != 1 {
		panic(fmt.Sprintf("call totalSupply unexpected return values: len=%d", len(totalSupplyOut)))
	}
	totalSupply, ok := totalSupplyOut[0].(*big.Int)
	if !ok {
		panic(fmt.Sprintf("call totalSupply return type unexpected: %T", totalSupplyOut[0]))
	}

	var maxSupplyOut []any
	if err := contract.Call(&bind.CallOpts{Context: ctx}, &maxSupplyOut, "maxSupply"); err != nil {
		panic(fmt.Sprintf("call maxSupply failed: %v", err))
	}
	if len(maxSupplyOut) != 1 {
		panic(fmt.Sprintf("call maxSupply unexpected return values: len=%d", len(maxSupplyOut)))
	}
	maxSupply, ok := maxSupplyOut[0].(*big.Int)
	if !ok {
		panic(fmt.Sprintf("call maxSupply return type unexpected: %T", maxSupplyOut[0]))
	}

	remaining := new(big.Int).Sub(maxSupply, totalSupply)
	if remaining.Sign() <= 0 {
		fmt.Printf("skip init: totalSupply=%s, maxSupply=%s (no remaining supply)\n", totalSupply.String(), maxSupply.String())
		return
	}
	remainingCount := int(remaining.Uint64())
	if remainingCount > len(files) {
		remainingCount = len(files)
	}
	if remainingCount <= 0 {
		fmt.Printf("skip init: remainingCount=%d (totalSupply=%s, maxSupply=%s)\n", remainingCount, totalSupply.String(), maxSupply.String())
		return
	}

	fmt.Printf("init mint: totalSupply=%s, maxSupply=%s, remainingCount=%d\n", totalSupply.String(), maxSupply.String(), remainingCount)
	files = files[:remainingCount]

	// 循环铸造NFT,并插入数据库
	for index, value := range files {

		// 1. 铸造NFT
		result, err := nftService.Mint(&value.request, value.fileData, false)
		if err != nil {
			panic(err)
		}

		// 2. 调用合约：mint(to, tokenURI)
		var out []any
		if err := contract.Call(&bind.CallOpts{Context: ctx}, &out, "price"); err != nil {
			panic(fmt.Sprintf("call price failed: %v", err))
		}
		if len(out) != 1 {
			panic(fmt.Sprintf("call price unexpected return values: len=%d", len(out)))
		}
		price, ok := out[0].(*big.Int)
		if !ok {
			panic(fmt.Sprintf("call price return type unexpected: %T", out[0]))
		}
		auth.Value = price

		// 用 EstimateGas 做一次预检查（如果失败，仍然尝试直接发送交易）
		mintData, err := parsedABI.Pack("mint", toAddr, result.TokenUrl)
		if err != nil {
			panic(fmt.Sprintf("pack mint calldata failed (i=%d): %v", index, err))
		}
		if _, err := client.EstimateGas(ctx, ethereum.CallMsg{
			From:  toAddr,
			To:    &contractAddr,
			Value: auth.Value,
			Data:  mintData,
		}); err != nil {
			fmt.Printf("warning: estimateGas failed (i=%d, err=%+v); will still send tx tokenUrl=%s msgValue=%s\n",
				index, err, result.TokenUrl, auth.Value.String())
		}

		// 手动构造并发送交易，避免依赖 Transact/estimateGas 内部流程
		// hardhat 开启自动出块时，pending nonce 偶发和最新 nonce 不一致，这里优先用 latest
		nonce, err := client.NonceAt(ctx, toAddr, nil)
		if err != nil {
			panic(fmt.Sprintf("get pending nonce failed (i=%d): err=%+v", index, err))
		}

		balance, err := client.BalanceAt(ctx, toAddr, nil)
		if err != nil {
			panic(fmt.Sprintf("get balance failed (i=%d): err=%+v", index, err))
		}

		gasPrice := auth.GasPrice
		if gasPrice == nil {
			gasPrice, err = client.SuggestGasPrice(ctx)
			if err != nil {
				panic(fmt.Sprintf("suggest gas price failed (i=%d): err=%+v", index, err))
			}
		}

		header, err := client.HeaderByNumber(ctx, nil)
		if err != nil {
			panic(fmt.Sprintf("get latest header failed (i=%d): err=%+v", index, err))
		}

		signer := types.LatestSignerForChainID(chainID)
		var rawTx *types.Transaction
		if header.BaseFee != nil {
			// EIP-1559 动态费用交易
			tipCap, err := client.SuggestGasTipCap(ctx)
			if err != nil {
				panic(fmt.Sprintf("suggest gas tip cap failed (i=%d): err=%+v", index, err))
			}
			// 给一个 buffer：maxFeePerGas = baseFee*2 + tipCap
			feeCap := new(big.Int).Mul(header.BaseFee, big.NewInt(2))
			feeCap.Add(feeCap, tipCap)

			dynamicTx := &types.DynamicFeeTx{
				Nonce:     nonce,
				To:        &contractAddr,
				Value:     auth.Value,
				Gas:       auth.GasLimit,
				Data:      mintData,
				GasTipCap: tipCap,
				GasFeeCap: feeCap,
			}
			rawTx = types.NewTx(dynamicTx)
		} else {
			// Legacy 交易
			rawTx = types.NewTransaction(nonce, contractAddr, auth.Value, auth.GasLimit, gasPrice, mintData)
		}

		signedTx, err := types.SignTx(rawTx, signer, privateKey)
		if err != nil {
			panic(fmt.Sprintf("sign tx failed (i=%d): err=%+v", index, err))
		}

		if err := client.SendTransaction(ctx, signedTx); err != nil {
			panic(fmt.Sprintf(
				"send transaction failed (i=%d, txHash=%s): errType=%T err=%#v errStr=%s tokenUrl=%s msgValue=%s balance=%s gasLimit=%d gasPrice=%v nonce=%d",
				index,
				signedTx.Hash().Hex(),
				err,
				err,
				err.Error(),
				result.TokenUrl,
				auth.Value.String(),
				balance.String(),
				auth.GasLimit,
				gasPrice,
				nonce,
			))
		}

		receipt, err := bind.WaitMined(ctx, client, signedTx)
		if err != nil {
			panic(fmt.Sprintf("wait mined failed (i=%d): %v", index, err))
		}
		if receipt.Status != 1 {
			panic(fmt.Sprintf("mint transaction reverted (i=%d, txHash=%s)", index, signedTx.Hash().Hex()))
		}

		// 3. 从 Mint / Transfer 事件解析 tokenId 和 owner
		var tokenID uint
		var owner string
		found := false

		// 优先从 Mint(sender, tokenId, url) 事件解析（sender 作为 owner）
		for _, lg := range receipt.Logs {
			if lg.Address != contractAddr {
				continue
			}
			if len(lg.Topics) < 3 || lg.Topics[0] != mintEventID {
				continue
			}

			senderAddr := common.BytesToAddress(lg.Topics[1].Bytes())
			tidBig := new(big.Int).SetBytes(lg.Topics[2].Bytes())
			tokenID = uint(tidBig.Uint64())
			owner = senderAddr.Hex()
			found = true
			break
		}

		// 兼容：如果没有 Mint 事件，则退回到 Transfer(from, to, tokenId) 事件解析
		if !found {
			for _, lg := range receipt.Logs {
				if lg.Address != contractAddr {
					continue
				}
				if len(lg.Topics) < 4 || lg.Topics[0] != transferEventID {
					continue
				}

				// Transfer(from, to, tokenId)，其中 from/to/tokenId 都是 indexed
				fromAddr := common.BytesToAddress(lg.Topics[1].Bytes())
				if fromAddr != (common.Address{}) {
					// 不是 mint（可能是后续转账）
					continue
				}

				toAddrFromEvent := common.BytesToAddress(lg.Topics[2].Bytes())
				tidBig := new(big.Int).SetBytes(lg.Topics[3].Bytes())
				tokenID = uint(tidBig.Uint64())
				owner = toAddrFromEvent.Hex()
				found = true
				break
			}
		}

		if !found {
			panic(fmt.Sprintf("cannot parse tokenId/owner from receipt logs (i=%d, txHash=%s)", index, signedTx.Hash().Hex()))
		}

		// 4. 链上成功后，存入数据库（tokenId 不作为主键）
		if err := nftListRepository.Insert(model.NftList{
			// nftId=1 对应上面插入的 NFT 类目
			NftID:       1,
			Name:        value.request.Name,
			Description: value.request.Description,
			MetadataUrl: result.MetadataUrl,
			ImageUrl:    result.ImageUrl,
			TokenUrl:    result.TokenUrl,
			TokenId:     tokenID,
			Owner:       owner,
			CreateTime:  time.Now(),
			UpdateTime:  time.Now(),
		}); err != nil {
			panic(fmt.Sprintf("insert nft list failed (i=%d, tokenId=%d): %v", index, tokenID, err))
		}
	}

}

// parseContractABI 兼容 Hardhat artifact（{"abi":[...], ...}）和纯 ABI 数组（[...]）
func parseContractABI(artifactBytes []byte) (abi.ABI, error) {
	// 先直接当纯 ABI 数组解析
	if parsed, err := abi.JSON(strings.NewReader(string(artifactBytes))); err == nil {
		return parsed, nil
	}

	// 再按 Hardhat artifact 结构提取 abi 字段
	var wrapper struct {
		ABI json.RawMessage `json:"abi"`
	}
	if err := json.Unmarshal(artifactBytes, &wrapper); err != nil {
		return abi.ABI{}, err
	}
	if len(wrapper.ABI) == 0 {
		return abi.ABI{}, fmt.Errorf("artifact missing `abi` field")
	}

	// wrapper.ABI 应该是 JSON 数组，交给 abi.JSON
	parsed, err := abi.JSON(strings.NewReader(string(wrapper.ABI)))
	if err != nil {
		return abi.ABI{}, err
	}
	return parsed, nil
}

// 获取文件
func getFiles() ([]FileInfo, error) {
	paths := []string{
		"../resource/Tuanzi01.jpg",
		"../resource/Tuanzi02.jpg",
		"../resource/Tuanzi03.jpg",
		// "../resource/Tuanzi04.jpg",
		// "../resource/Tuanzi05.jpg",
		// "../resource/Tuanzi06.jpg",
	}

	res := make([]FileInfo, 0, len(paths))
	for i, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read file failed: %s, err=%w", p, err)
		}

		// request.Name / Description 仅用于 Pinata metadata（以及数据库展示字段）
		name := fmt.Sprintf("Tuanzi%02d", i+1)
		desc := name

		res = append(res, FileInfo{
			request: request.MintRequest{
				Name:        name,
				Description: desc,
			},
			fileData: bytes.NewReader(data),
		})
	}

	return res, nil
}
