package utils

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

	"bloom-nft/config"

	"github.com/ethereum/go-ethereum/ethclient"
)

// GetChainID 从 RPC 查询 chainId。
func GetChainID() (*big.Int, error) {
	rpc := strings.TrimSpace(config.AppConfig.NetWork.RpcUrl)
	if rpc == "" {
		return nil, errors.New("rpc url is empty")
	}
	client, err := ethclient.Dial(rpc)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	return client.ChainID(ctx)
}

