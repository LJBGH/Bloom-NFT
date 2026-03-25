package utils

import (
	"errors"
	"math/big"
	"strconv"
	"strings"
)

// BtToWei 将 BT 浮点价格按 18 位精度转成 wei（与原 bloom_marketplace.go 的 btToWei 保持一致）。
func BtToWei(bt float64) (*big.Int, error) {
	// 与前端 parseUnits(String(priceNum), 18) 对齐：把浮点价格字符串转成 18 位最小单位
	btStr := strconv.FormatFloat(bt, 'f', -1, 64)
	rat, ok := new(big.Rat).SetString(btStr)
	if !ok {
		return nil, errors.New("invalid price format")
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	rat.Mul(rat, new(big.Rat).SetInt(scale))
	// floor
	out := new(big.Int).Quo(rat.Num(), rat.Denom())
	if out.Sign() <= 0 {
		return nil, errors.New("price must be > 0")
	}
	return out, nil
}

// ParseListingSalt 解析挂单/出价 EIP-712 中的 salt（十进制字符串，或 0x 十六进制）。
func ParseListingSalt(s string) (*big.Int, error) {
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

