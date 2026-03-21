package listener

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
)

var (
	// EIP-712 结构体哈希所需的 type hash 常量。
	listingTypeHash = crypto.Keccak256Hash([]byte("Listing(address nft,address seller,uint256 tokenId,uint256 price,uint256 deadline,uint256 nonce)"))
	bidTypeHash     = crypto.Keccak256Hash([]byte("Bid(bytes32 listingHash,address buyer,uint256 price,uint256 deadline,uint256 nonce)"))
	domainTypeHash  = crypto.Keccak256Hash([]byte("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"))
	nameHash        = crypto.Keccak256Hash([]byte("BloomMarketplace"))
	versionHash     = crypto.Keccak256Hash([]byte("1"))
)

// calcListingHash 重建合约中 Listing 的 EIP-712 typed-data 摘要。
func calcListingHash(
	chainID *big.Int,
	verifyingContract common.Address,
	nftAddr string,
	sellerAddr string,
	tokenID uint64,
	price *big.Int,
	deadline uint64,
	nonce uint64,
) common.Hash {
	bytes, _ := abi.Arguments{
		{Type: mustType("bytes32")},
		{Type: mustType("address")},
		{Type: mustType("address")},
		{Type: mustType("uint256")},
		{Type: mustType("uint256")},
		{Type: mustType("uint256")},
		{Type: mustType("uint256")},
	}.Pack(
		listingTypeHash,
		common.HexToAddress(nftAddr),
		common.HexToAddress(sellerAddr),
		new(big.Int).SetUint64(tokenID),
		price,
		new(big.Int).SetUint64(deadline),
		new(big.Int).SetUint64(nonce),
	)
	structHash := crypto.Keccak256Hash(bytes)
	return calcEIP712Digest(chainID, verifyingContract, structHash)
}

// calcBidHash 重建合约中 Bid 的 EIP-712 typed-data 摘要。
func calcBidHash(
	chainID *big.Int,
	verifyingContract common.Address,
	listingHash common.Hash,
	buyer common.Address,
	price *big.Int,
	deadline uint64,
	nonce uint64,
) common.Hash {
	bytes, _ := abi.Arguments{
		{Type: mustType("bytes32")},
		{Type: mustType("bytes32")},
		{Type: mustType("address")},
		{Type: mustType("uint256")},
		{Type: mustType("uint256")},
		{Type: mustType("uint256")},
	}.Pack(
		bidTypeHash,
		listingHash,
		buyer,
		price,
		new(big.Int).SetUint64(deadline),
		new(big.Int).SetUint64(nonce),
	)
	structHash := crypto.Keccak256Hash(bytes)
	return calcEIP712Digest(chainID, verifyingContract, structHash)
}

// calcEIP712Digest 计算 keccak256("\x19\x01" || domainSeparator || structHash)。
func calcEIP712Digest(chainID *big.Int, verifyingContract common.Address, structHash common.Hash) common.Hash {
	domainData, _ := abi.Arguments{
		{Type: mustType("bytes32")},
		{Type: mustType("bytes32")},
		{Type: mustType("bytes32")},
		{Type: mustType("uint256")},
		{Type: mustType("address")},
	}.Pack(
		domainTypeHash,
		nameHash,
		versionHash,
		chainID,
		verifyingContract,
	)
	domainSeparator := crypto.Keccak256Hash(domainData)
	raw := append([]byte{0x19, 0x01}, domainSeparator.Bytes()...)
	raw = append(raw, structHash.Bytes()...)
	return crypto.Keccak256Hash(raw)
}

// mustType 用于静态 ABI 类型声明，初始化失败时直接 panic。
func mustType(t string) abi.Type {
	typ, err := abi.NewType(t, "", nil)
	if err != nil {
		panic(err)
	}
	return typ
}
