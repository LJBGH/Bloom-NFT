package main

import "testing"

// 测试命令： go test ./test -run ^TestInitNft$ -v -count=1
func TestInitNft(t *testing.T) {
	InitNft()
}
