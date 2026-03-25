package utils

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	// 与 BloomMarketplace.sol 中 LISTING_TYPEHASH / BID_TYPEHASH 字符串逐字一致。
	listingTypeHash = crypto.Keccak256Hash([]byte("Listing(address nft,address seller,uint256 tokenId,uint256 price,uint256 deadline,uint256 salt)"))
	bidTypeHash     = crypto.Keccak256Hash([]byte("Bid(address nft,address buyer,uint256 tokenId,uint256 price,uint256 deadline,uint256 salt)"))

	domainTypeHash = crypto.Keccak256Hash([]byte("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"))
	nameHash       = crypto.Keccak256Hash([]byte("BloomMarketplace"))
	versionHash    = crypto.Keccak256Hash([]byte("1"))
)

// CalcEIP712Digest 实现 EIP-712 规则的可签名消息哈希，与 OZ EIP712._hashTypedDataV4(structHash) 一致。
func CalcEIP712Digest(chainID *big.Int, verifyingContract common.Address, structHash common.Hash) common.Hash {
	domainData, _ := abi.Arguments{
		{Type: mustType("bytes32")}, // 域类型哈希
		{Type: mustType("bytes32")}, // 名称哈希
		{Type: mustType("bytes32")}, // 版本哈希
		{Type: mustType("uint256")}, // 链ID
		{Type: mustType("address")}, // 验证合约地址
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

// CalcListingHash 计算上架订单的hash
func CalcListingHash(
	chainID *big.Int,
	verifyingContract common.Address,
	nftAddr common.Address,
	sellerAddr common.Address,
	tokenID uint64,
	price *big.Int,
	deadline uint64,
	salt *big.Int,
) common.Hash {
	encoded, _ := abi.Arguments{
		{Type: mustType("bytes32")},
		{Type: mustType("address")},
		{Type: mustType("address")},
		{Type: mustType("uint256")},
		{Type: mustType("uint256")},
		{Type: mustType("uint256")},
		{Type: mustType("uint256")},
	}.Pack(
		listingTypeHash,
		nftAddr,
		sellerAddr,
		new(big.Int).SetUint64(tokenID),
		price,
		new(big.Int).SetUint64(deadline),
		salt,
	)
	structHash := crypto.Keccak256Hash(encoded)
	return CalcEIP712Digest(chainID, verifyingContract, structHash)
}

func CalcBidHash(
	chainID *big.Int,
	verifyingContract common.Address,
	nftAddr common.Address,
	buyerAddr common.Address,
	tokenID uint64,
	price *big.Int,
	deadline uint64,
	salt *big.Int,
) common.Hash {
	encoded, _ := abi.Arguments{
		{Type: mustType("bytes32")},
		{Type: mustType("address")},
		{Type: mustType("address")},
		{Type: mustType("uint256")},
		{Type: mustType("uint256")},
		{Type: mustType("uint256")},
		{Type: mustType("uint256")},
	}.Pack(
		bidTypeHash,
		nftAddr,
		buyerAddr,
		new(big.Int).SetUint64(tokenID),
		price,
		new(big.Int).SetUint64(deadline),
		salt,
	)
	structHash := crypto.Keccak256Hash(encoded)
	return CalcEIP712Digest(chainID, verifyingContract, structHash)
}

// mustType 用于静态 ABI 类型声明；初始化失败直接 panic。
func mustType(t string) abi.Type {
	typ, err := abi.NewType(t, "", nil)
	if err != nil {
		panic(err)
	}
	return typ
}
