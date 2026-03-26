package main

import (
	"errors"
	"testing"
)

// 测试命令： go test ./test -run ^TestInitNft$ -v -count=1
func TestInitNft(t *testing.T) {
	if err := InitNft(); err != nil {
		if errors.Is(err, ErrContractNotDeployed) {
			t.Skipf("skip: local RPC has no deployed contracts. Please start local chain and deploy contracts, then update abi/contract-addresses.json if needed. err=%v", err)
		}
		t.Fatalf("InitNft failed: %v", err)
	}
}
