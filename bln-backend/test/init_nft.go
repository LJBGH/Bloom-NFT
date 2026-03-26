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
	"errors"
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

var ErrContractNotDeployed = errors.New("contract not deployed at address")

func InitNft() error {
	config.LoadConfig()
	database.InitializeDB("")
	database.AutoMigrate()
	db := database.DB

	nftReposity := repository.NewNftRepository(db)
	nftListRepository := repository.NewNftListRepository(db)

	nftService := services.NewNftService(nftReposity, nftListRepository)

	// BloomNFT0 / BloomNFT1 两套合约地址（用于区分 NFT 类型）
	nftContractAddress0 := utils.GetContractAddress("BloomNFT")
	if nftContractAddress0 == "" {
		return fmt.Errorf("nft contract address BloomNFT is empty")
	}
	nftContractAddress1 := utils.GetContractAddress("BloomNFT1")
	if nftContractAddress1 == "" {
		return fmt.Errorf("nft contract address BloomNFT1 is empty")
	}

	// 先插入NFT类目
	nftReposity.Insert(model.Nft{
		ID:          1,
		Name:        "Bloom-NFT",
		Description: "Bloom-NFT",
		Address:     nftContractAddress0,
		CreateTime:  time.Now(),
		UpdateTime:  time.Now(),
	})

	nftReposity.Insert(model.Nft{
		ID:          2,
		Name:        "Bloom-NFT1",
		Description: "Bloom-NFT1",
		Address:     nftContractAddress1,
		CreateTime:  time.Now(),
		UpdateTime:  time.Now(),
	})

	// 读取文件
	files, err := getBloonNFT0Files()
	if err != nil {
		return err
	}

	files1, err := getBloonNFT1Files()
	if err != nil {
		return err
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
		return fmt.Errorf("ethclient dial failed: %v", err)
	}
	defer client.Close()

	// 默认先初始化 BloomNFT0（也就是你现在的 NFT0 列表）
	contractAddr := common.HexToAddress(nftContractAddress0)
	contractAddr1 := common.HexToAddress(nftContractAddress1)

	abiBytes, err := os.ReadFile(abiPath)
	if err != nil {
		// 兼容相对路径在不同运行目录下不一致的情况
		alt := filepath.Join("..", "abi", filepath.Base(abiPath))
		abiBytes2, err2 := os.ReadFile(alt)
		if err2 != nil {
			return fmt.Errorf("read abi failed: %v (alt=%s err=%v)", err, alt, err2)
		}
		abiBytes = abiBytes2
	}

	parsedABI, err := parseContractABI(abiBytes)
	if err != nil {
		return fmt.Errorf("parse abi failed: %v", err)
	}

	// auth：发送交易所用的签名
	privateKeyStr := strings.TrimPrefix(strings.TrimSpace(accountProviteKey), "0x")
	privateKeyBytes, err := hex.DecodeString(privateKeyStr)
	if err != nil {
		return fmt.Errorf("invalid private key: %v", err)
	}
	privateKey, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return fmt.Errorf("to ecdsa failed: %v", err)
	}

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("get chainID failed: %v", err)
	}

	toAddr := crypto.PubkeyToAddress(privateKey.PublicKey)
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return fmt.Errorf("new keyed transactor failed: %v", err)
	}
	auth.Context = ctx
	auth.GasLimit = uint64(6_000_000) // mint 给更保守一点的 gas，减少 under-gas 的概率

	// 显式设置 GasPrice（legacy），避免部分链在 EIP-1559 模式下的 fee 估算导致 RPC 内部错误
	if gp, err := client.SuggestGasPrice(ctx); err == nil {
		auth.GasPrice = gp
	}

	// 在调用合约方法前先校验目标地址是否真的部署了代码（避免 "no contract code at given address" 直接 panic）
	if err := requireContractCode(ctx, client, "BloomNFT", contractAddr, chainID, rpcUrl); err != nil {
		return err
	}
	if err := requireContractCode(ctx, client, "BloomNFT1", contractAddr1, chainID, rpcUrl); err != nil {
		return err
	}

	contract := bind.NewBoundContract(contractAddr, parsedABI, client, client, client)
	transferEventID := parsedABI.Events["Transfer"].ID
	mintEventID := parsedABI.Events["Mint"].ID

	// 幂等保护：避免超过 maxSupply 导致 estimateGas / mint 失败
	var totalSupplyOut []any
	if err := contract.Call(&bind.CallOpts{Context: ctx}, &totalSupplyOut, "totalSupply"); err != nil {
		return fmt.Errorf("call totalSupply failed: %v", err)
	}
	if len(totalSupplyOut) != 1 {
		return fmt.Errorf("call totalSupply unexpected return values: len=%d", len(totalSupplyOut))
	}
	totalSupply, ok := totalSupplyOut[0].(*big.Int)
	if !ok {
		return fmt.Errorf("call totalSupply return type unexpected: %T", totalSupplyOut[0])
	}

	var maxSupplyOut []any
	if err := contract.Call(&bind.CallOpts{Context: ctx}, &maxSupplyOut, "maxSupply"); err != nil {
		return fmt.Errorf("call maxSupply failed: %v", err)
	}
	if len(maxSupplyOut) != 1 {
		return fmt.Errorf("call maxSupply unexpected return values: len=%d", len(maxSupplyOut))
	}
	maxSupply, ok := maxSupplyOut[0].(*big.Int)
	if !ok {
		return fmt.Errorf("call maxSupply return type unexpected: %T", maxSupplyOut[0])
	}

	remaining := new(big.Int).Sub(maxSupply, totalSupply)
	if remaining.Sign() > 0 {
		remainingCount := int(remaining.Uint64())
		if remainingCount > len(files) {
			remainingCount = len(files)
		}
		if remainingCount > 0 {
			fmt.Printf("init mint BloomNFT: totalSupply=%s, maxSupply=%s, remainingCount=%d\n", totalSupply.String(), maxSupply.String(), remainingCount)
			files = files[:remainingCount]

			// 循环铸造NFT,并插入数据库
			for index, value := range files {

				// 1. 铸造NFT
				result, err := nftService.Mint(&value.request, value.fileData, false)
				if err != nil {
					return err
				}

				// 2. 调用合约：mint(to, tokenURI)
				var out []any
				if err := contract.Call(&bind.CallOpts{Context: ctx}, &out, "price"); err != nil {
					return fmt.Errorf("call price failed: %v", err)
				}
				if len(out) != 1 {
					return fmt.Errorf("call price unexpected return values: len=%d", len(out))
				}
				price, ok := out[0].(*big.Int)
				if !ok {
					return fmt.Errorf("call price return type unexpected: %T", out[0])
				}
				auth.Value = price

				// 用 EstimateGas 做一次预检查（如果失败，仍然尝试直接发送交易）
				mintData, err := parsedABI.Pack("mint", toAddr, result.TokenUrl)
				if err != nil {
					return fmt.Errorf("pack mint calldata failed (i=%d): %v", index, err)
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
					return fmt.Errorf("get pending nonce failed (i=%d): err=%+v", index, err)
				}

				balance, err := client.BalanceAt(ctx, toAddr, nil)
				if err != nil {
					return fmt.Errorf("get balance failed (i=%d): err=%+v", index, err)
				}

				gasPrice := auth.GasPrice
				if gasPrice == nil {
					gasPrice, err = client.SuggestGasPrice(ctx)
					if err != nil {
						return fmt.Errorf("suggest gas price failed (i=%d): err=%+v", index, err)
					}
				}

				header, err := client.HeaderByNumber(ctx, nil)
				if err != nil {
					return fmt.Errorf("get latest header failed (i=%d): err=%+v", index, err)
				}

				signer := types.LatestSignerForChainID(chainID)
				var rawTx *types.Transaction
				if header.BaseFee != nil {
					// EIP-1559 动态费用交易
					tipCap, err := client.SuggestGasTipCap(ctx)
					if err != nil {
						return fmt.Errorf("suggest gas tip cap failed (i=%d): err=%+v", index, err)
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
					return fmt.Errorf("sign tx failed (i=%d): err=%+v", index, err)
				}

				if err := client.SendTransaction(ctx, signedTx); err != nil {
					return fmt.Errorf(
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
					)
				}

				receipt, err := bind.WaitMined(ctx, client, signedTx)
				if err != nil {
					return fmt.Errorf("wait mined failed (i=%d): %v", index, err)
				}
				if receipt.Status != 1 {
					return fmt.Errorf("mint transaction reverted (i=%d, txHash=%s)", index, signedTx.Hash().Hex())
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
					return fmt.Errorf("cannot parse tokenId/owner from receipt logs (i=%d, txHash=%s)", index, signedTx.Hash().Hex())
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
					return fmt.Errorf("insert nft list failed (i=%d, tokenId=%d): %v", index, tokenID, err)
				}
			}
		}
	}

	// 初始化 BloomNFT1（NFT 类型 1 -> nft_id = 2）
	contract1 := bind.NewBoundContract(contractAddr1, parsedABI, client, client, client)

	// 幂等保护：避免超过 maxSupply 导致 estimateGas / mint 失败
	var totalSupplyOut1 []any
	if err := contract1.Call(&bind.CallOpts{Context: ctx}, &totalSupplyOut1, "totalSupply"); err != nil {
		return fmt.Errorf("call totalSupply1 failed: %v", err)
	}
	if len(totalSupplyOut1) != 1 {
		return fmt.Errorf("call totalSupply1 unexpected return values: len=%d", len(totalSupplyOut1))
	}
	totalSupply1, ok := totalSupplyOut1[0].(*big.Int)
	if !ok {
		return fmt.Errorf("call totalSupply1 return type unexpected: %T", totalSupplyOut1[0])
	}

	var maxSupplyOut1 []any
	if err := contract1.Call(&bind.CallOpts{Context: ctx}, &maxSupplyOut1, "maxSupply"); err != nil {
		return fmt.Errorf("call maxSupply1 failed: %v", err)
	}
	if len(maxSupplyOut1) != 1 {
		return fmt.Errorf("call maxSupply1 unexpected return values: len=%d", len(maxSupplyOut1))
	}
	maxSupply1, ok := maxSupplyOut1[0].(*big.Int)
	if !ok {
		return fmt.Errorf("call maxSupply1 return type unexpected: %T", maxSupplyOut1[0])
	}

	remaining1 := new(big.Int).Sub(maxSupply1, totalSupply1)
	if remaining1.Sign() > 0 {
		remainingCount1 := int(remaining1.Uint64())
		if remainingCount1 > len(files1) {
			remainingCount1 = len(files1)
		}
		if remainingCount1 > 0 {
			fmt.Printf("init mint BloomNFT1: totalSupply=%s, maxSupply=%s, remainingCount=%d\n", totalSupply1.String(), maxSupply1.String(), remainingCount1)
			files1 = files1[:remainingCount1]

			// 循环铸造NFT1,并插入数据库
			for index, value := range files1 {
				// 1. 铸造NFT
				result, err := nftService.Mint(&value.request, value.fileData, false)
				if err != nil {
					return err
				}

				// 2. 调用合约：mint(to, tokenURI)
				var out []any
				if err := contract1.Call(&bind.CallOpts{Context: ctx}, &out, "price"); err != nil {
					return fmt.Errorf("call price1 failed: %v", err)
				}
				if len(out) != 1 {
					return fmt.Errorf("call price1 unexpected return values: len=%d", len(out))
				}
				price1, ok := out[0].(*big.Int)
				if !ok {
					return fmt.Errorf("call price1 return type unexpected: %T", out[0])
				}
				auth.Value = price1

				// 用 EstimateGas 做一次预检查（如果失败，仍然尝试直接发送交易）
				mintData, err := parsedABI.Pack("mint", toAddr, result.TokenUrl)
				if err != nil {
					return fmt.Errorf("pack mint calldata failed (i=%d): %v", index, err)
				}
				if _, err := client.EstimateGas(ctx, ethereum.CallMsg{
					From:  toAddr,
					To:    &contractAddr1,
					Value: auth.Value,
					Data:  mintData,
				}); err != nil {
					fmt.Printf("warning: estimateGas failed (i=%d, contract=BloomNFT1, err=%+v); will still send tx tokenUrl=%s msgValue=%s\n",
						index, err, result.TokenUrl, auth.Value.String())
				}

				// 手动构造并发送交易，避免依赖 Transact/estimateGas 内部流程
				nonce, err := client.NonceAt(ctx, toAddr, nil)
				if err != nil {
					return fmt.Errorf("get pending nonce failed (i=%d): err=%+v", index, err)
				}

				balance, err := client.BalanceAt(ctx, toAddr, nil)
				if err != nil {
					return fmt.Errorf("get balance failed (i=%d): err=%+v", index, err)
				}

				gasPrice := auth.GasPrice
				if gasPrice == nil {
					gasPrice, err = client.SuggestGasPrice(ctx)
					if err != nil {
						return fmt.Errorf("suggest gas price failed (i=%d): err=%+v", index, err)
					}
				}

				header, err := client.HeaderByNumber(ctx, nil)
				if err != nil {
					return fmt.Errorf("get latest header failed (i=%d): err=%+v", index, err)
				}

				signer := types.LatestSignerForChainID(chainID)
				var rawTx *types.Transaction
				if header.BaseFee != nil {
					// EIP-1559 动态费用交易
					tipCap, err := client.SuggestGasTipCap(ctx)
					if err != nil {
						return fmt.Errorf("suggest gas tip cap failed (i=%d): err=%+v", index, err)
					}
					// 给一个 buffer：maxFeePerGas = baseFee*2 + tipCap
					feeCap := new(big.Int).Mul(header.BaseFee, big.NewInt(2))
					feeCap.Add(feeCap, tipCap)

					dynamicTx := &types.DynamicFeeTx{
						Nonce:     nonce,
						To:        &contractAddr1,
						Value:     auth.Value,
						Gas:       auth.GasLimit,
						Data:      mintData,
						GasTipCap: tipCap,
						GasFeeCap: feeCap,
					}
					rawTx = types.NewTx(dynamicTx)
				} else {
					// Legacy 交易
					rawTx = types.NewTransaction(nonce, contractAddr1, auth.Value, auth.GasLimit, gasPrice, mintData)
				}

				signedTx, err := types.SignTx(rawTx, signer, privateKey)
				if err != nil {
					return fmt.Errorf("sign tx failed (i=%d): err=%+v", index, err)
				}

				if err := client.SendTransaction(ctx, signedTx); err != nil {
					return fmt.Errorf(
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
					)
				}

				receipt, err := bind.WaitMined(ctx, client, signedTx)
				if err != nil {
					return fmt.Errorf("wait mined failed (i=%d): %v", index, err)
				}
				if receipt.Status != 1 {
					return fmt.Errorf("mint transaction reverted (i=%d, txHash=%s)", index, signedTx.Hash().Hex())
				}

				// 3. 从 Mint / Transfer 事件解析 tokenId 和 owner
				var tokenID uint
				var owner string
				found := false

				// 优先从 Mint(sender, tokenId, url) 事件解析（sender 作为 owner）
				for _, lg := range receipt.Logs {
					if lg.Address != contractAddr1 {
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
						if lg.Address != contractAddr1 {
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
					return fmt.Errorf("cannot parse tokenId/owner from receipt logs (i=%d, contract=BloomNFT1, txHash=%s)", index, signedTx.Hash().Hex())
				}

				// 4. 链上成功后，存入数据库（tokenId 不作为主键）
				if err := nftListRepository.Insert(model.NftList{
					// nftId=2 对应上面插入的 BloomNFT1 类目
					NftID:       2,
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
					return fmt.Errorf("insert nft list failed (i=%d, tokenId=%d, contract=BloomNFT1): %v", index, tokenID, err)
				}
			}
		} else {
			fmt.Printf("skip init BloomNFT1: remainingCount=%d (totalSupply=%s, maxSupply=%s)\n", remainingCount1, totalSupply1.String(), maxSupply1.String())
		}
	} else {
		fmt.Printf("skip init BloomNFT1: totalSupply=%s, maxSupply=%s (no remaining supply)\n", totalSupply1.String(), maxSupply1.String())
	}

	return nil
}

func requireContractCode(ctx context.Context, client *ethclient.Client, name string, addr common.Address, chainID *big.Int, rpc string) error {
	code, err := client.CodeAt(ctx, addr, nil)
	if err != nil {
		return fmt.Errorf("check contract code failed (name=%s addr=%s chainID=%s rpc=%s): %w", name, addr.Hex(), chainID.String(), rpc, err)
	}
	if len(code) == 0 {
		return fmt.Errorf("%w (name=%s addr=%s chainID=%s rpc=%s)", ErrContractNotDeployed, name, addr.Hex(), chainID.String(), rpc)
	}
	return nil
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

// 获取BloonNFT0文件
func getBloonNFT0Files() ([]FileInfo, error) {
	paths := []string{
		"../resource/BloomNFT0-1.jpg",
		"../resource/BloomNFT0-2.jpg",
		"../resource/BloomNFT0-3.jpg",
	}

	res := make([]FileInfo, 0, len(paths))
	for i, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read file failed: %s, err=%w", p, err)
		}

		// request.Name / Description 仅用于 Pinata metadata（以及数据库展示字段）
		name := fmt.Sprintf("BloomNFT0-%02d", i+1)
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

// 获取BloonNFT1文件
func getBloonNFT1Files() ([]FileInfo, error) {
	paths := []string{
		"../resource/BloomNFT1-1.jpg",
		"../resource/BloomNFT1-2.jpg",
		"../resource/BloomNFT1-3.jpg",
	}

	res := make([]FileInfo, 0, len(paths))
	for i, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read file failed: %s, err=%w", p, err)
		}

		// request.Name / Description 仅用于 Pinata metadata（以及数据库展示字段）
		name := fmt.Sprintf("BloomNFT1-%02d", i+1)
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
