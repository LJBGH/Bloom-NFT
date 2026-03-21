package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func readFileWithFallback(path string) ([]byte, error) {
	if b, err := os.ReadFile(path); err == nil {
		return b, nil
	}
	return os.ReadFile(filepath.Join("..", path))
}

// 获取合约地址
// ../abi/contract-addresses.json 文件中获取
// 传入合约名称，返回合约地址
func GetContractAddress(contractName string) string {
	b, err := readFileWithFallback(filepath.Join("abi", "contract-addresses.json"))
	if err != nil {
		return ""
	}

	// { "local": { "BloomMarketplace": "0x..." } }
	var m map[string]map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return ""
	}

	if local, ok := m["local"]; ok {
		return local[contractName]
	}
	return ""
}

// 获取合约ABI
// ../abi/...json 文件中获取
// 传入合约名称，返回合约ABI
func GetContractABI(contractName string) string {
	b, err := readFileWithFallback(filepath.Join("abi", contractName+".json"))
	if err != nil {
		return ""
	}

	// 先尝试作为 Hardhat artifact 提取 abi 字段
	var wrap struct {
		ABI json.RawMessage `json:"abi"`
	}
	if err := json.Unmarshal(b, &wrap); err == nil && len(wrap.ABI) > 0 {
		return string(wrap.ABI)
	}

	// 否则退化为文件原文（兼容纯 ABI 数组）
	return string(b)
}
