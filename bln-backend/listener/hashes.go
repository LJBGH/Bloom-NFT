// Package listener 中的本文件用于在链下按与 BloomMarketplace 合约完全相同的规则
// 计算 EIP-712 digest（listingHash / bidHash），供事件解析、对账或与链上 topic 比对。
//
// 对应合约位置：
//   - LISTING_TYPEHASH / BID_TYPEHASH：BloomMarketplace.sol 中常量字符串的 keccak256
//   - Domain：constructor EIP712("BloomMarketplace", "1") 与 chainId、verifyingContract
//   - 最终 digest：OpenZeppelin EIP712._hashTypedDataV4(structHash)，即对
//     keccak256(0x19 ‖ 0x01 ‖ domainSeparator ‖ structHash) 再取 keccak256（见 calcEIP712Digest）
//
// 若合约修改了 EIP712 名称/版本/结构体字段，须同步修改本文件中的字符串与 Pack 顺序。
package listener

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	// 以下 type hash 与 BloomMarketplace.sol 中 LISTING_TYPEHASH、BID_TYPEHASH 的字符串必须逐字一致。
	listingTypeHash = crypto.Keccak256Hash([]byte("Listing(address nft,address seller,uint256 tokenId,uint256 price,uint256 deadline,uint256 salt)"))
	bidTypeHash     = crypto.Keccak256Hash([]byte("Bid(bytes32 listingHash,address buyer,uint256 price,uint256 deadline,uint256 salt)"))
	// LISTING_LEAF_TYPEHASH、BATCH_LISTING_TYPEHASH：与 BloomMarketplace.sol 中常量一致。
	listingLeafTypeHash = crypto.Keccak256Hash([]byte("ListingLeaf(address nft,address seller,uint256 tokenId,uint256 price,uint256 deadline,uint256 salt)"))
	batchListingTypeHash = crypto.Keccak256Hash([]byte("BatchListing(bytes32 merkleRoot,address seller,uint256 rootDeadline)"))
	// EIP712Domain 类型定义（EIP-712 标准）；与 OZ EIP712 计算 domainSeparator 时所用一致。
	domainTypeHash = crypto.Keccak256Hash([]byte("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"))
	// 与合约 constructor EIP712("BloomMarketplace", "1") 中的 name、version 对应。
	nameHash    = crypto.Keccak256Hash([]byte("BloomMarketplace"))
	versionHash = crypto.Keccak256Hash([]byte("1"))
)

// calcListingHash 重建 Listing 的 EIP-712 最终摘要（与链上 _verifyListing 中 listingHash 一致）。
// 步骤：abi.encode(LISTING_TYPEHASH, nft, seller, ...) → keccak256 得 structHash → calcEIP712Digest。
func calcListingHash(
	chainID *big.Int,
	verifyingContract common.Address,
	nftAddr string,
	sellerAddr string,
	tokenID uint64,
	price *big.Int,
	deadline uint64,
	salt *big.Int,
) common.Hash {
	bytes, _ := abi.Arguments{
		{Type: mustType("bytes32")}, // 类型哈希
		{Type: mustType("address")}, // NFT地址
		{Type: mustType("address")}, // 卖家地址
		{Type: mustType("uint256")}, // 代币ID
		{Type: mustType("uint256")}, // 价格
		{Type: mustType("uint256")}, // 截止时间
		{Type: mustType("uint256")}, // salt
	}.Pack(
		listingTypeHash,
		common.HexToAddress(nftAddr),
		common.HexToAddress(sellerAddr),
		new(big.Int).SetUint64(tokenID),
		price,
		new(big.Int).SetUint64(deadline),
		salt,
	)

	// structHash：与合约 keccak256(abi.encode(LISTING_TYPEHASH, listing 各字段)) 相同。
	structHash := crypto.Keccak256Hash(bytes)
	return calcEIP712Digest(chainID, verifyingContract, structHash)
}

// calcBidHash 重建 Bid 的 EIP-712 最终摘要（与链上 _verifyBid 中 bidHash 一致）。
func calcBidHash(
	chainID *big.Int,
	verifyingContract common.Address,
	listingHash common.Hash,
	buyer common.Address,
	price *big.Int,
	deadline uint64,
	salt *big.Int,
) common.Hash {
	bytes, _ := abi.Arguments{
		{Type: mustType("bytes32")}, // 类型哈希
		{Type: mustType("bytes32")}, // 列表哈希
		{Type: mustType("address")}, // 买家地址
		{Type: mustType("uint256")}, // 价格
		{Type: mustType("uint256")}, // 截止时间
		{Type: mustType("uint256")}, // salt
	}.Pack(
		bidTypeHash,
		listingHash,
		buyer,
		price,
		new(big.Int).SetUint64(deadline),
		salt,
	)

	structHash := crypto.Keccak256Hash(bytes)
	return calcEIP712Digest(chainID, verifyingContract, structHash)
}

// calcEIP712Digest 实现 EIP-712 规定的「待签名消息」哈希，与 OpenZeppelin EIP712._hashTypedDataV4(structHash) 一致。
//
// 公式：digest = keccak256(0x19 ‖ 0x01 ‖ domainSeparator ‖ structHash)
//   - 0x19：以太坊中带前缀的可签名消息惯例；0x01：表示后续为 EIP-712 v4 数据（区别于 personal_sign 等）。
//   - domainSeparator：由 name/version/chainId/verifyingContract 编码后再 keccak256，绑定「在哪条链、哪个合约」签名。
//   - structHash：仅由 Listing/Bid 字段算出的哈希；三者拼接后再 keccak256 才是钱包签名与合约 recover 使用的 digest。
func calcEIP712Digest(chainID *big.Int, verifyingContract common.Address, structHash common.Hash) common.Hash {
	domainData, _ := abi.Arguments{
		{Type: mustType("bytes32")}, // 域哈希
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

	// 拼接 EIP-712 消息字节流，长度固定为 2 + 32 + 32 = 66 字节（不含 0x19/0x01 时 domain+struct 为 64 字节）。
	raw := append([]byte{0x19, 0x01}, domainSeparator.Bytes()...)
	raw = append(raw, structHash.Bytes()...)

	// 对整段 raw 做一次 keccak256，得到与合约 _hashTypedDataV4 相同的 32 字节 digest。
	return crypto.Keccak256Hash(raw)
}

// CalcListingLeafHash 与链上 _listingLeafHash 一致（Merkle 叶子）。
func CalcListingLeafHash(
	nftAddr string,
	sellerAddr string,
	tokenID uint64,
	price *big.Int,
	deadline uint64,
	salt *big.Int,
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
		listingLeafTypeHash,
		common.HexToAddress(nftAddr),
		common.HexToAddress(sellerAddr),
		new(big.Int).SetUint64(tokenID),
		price,
		new(big.Int).SetUint64(deadline),
		salt,
	)
	return crypto.Keccak256Hash(bytes)
}

// CalcBatchListingDigest 与链上 BatchListing EIP-712 digest 一致（卖家对 Merkle 根的一次性签名）。
func CalcBatchListingDigest(
	chainID *big.Int,
	verifyingContract common.Address,
	merkleRoot common.Hash,
	seller common.Address,
	rootDeadline uint64,
) common.Hash {
	bytes, _ := abi.Arguments{
		{Type: mustType("bytes32")},
		{Type: mustType("bytes32")},
		{Type: mustType("address")},
		{Type: mustType("uint256")},
	}.Pack(
		batchListingTypeHash,
		merkleRoot,
		seller,
		new(big.Int).SetUint64(rootDeadline),
	)
	structHash := crypto.Keccak256Hash(bytes)
	return calcEIP712Digest(chainID, verifyingContract, structHash)
}

// mustType 用于静态 ABI 类型声明，初始化失败时直接 panic。
func mustType(t string) abi.Type {
	typ, err := abi.NewType(t, "", nil)
	if err != nil {
		panic(err)
	}
	return typ
}
