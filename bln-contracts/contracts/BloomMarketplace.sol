// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/token/ERC721/IERC721.sol";
import "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";
import "@openzeppelin/contracts/utils/cryptography/EIP712.sol";
import "@openzeppelin/contracts/utils/cryptography/MerkleProof.sol";
import "./BloomToken.sol";

/// @notice 仅做链上结算：卖单/买单均链下签名，成交时上链。
contract BloomMarketplace is Ownable, EIP712 {
    using ECDSA for bytes32;

    BloomToken public immutable token;

    // 手续费 = 金额 * 手续费率 / 手续费精度
    uint256 public feeRate = 25;
    uint256 public constant feePrecision = 1000;

    /// @notice 订单状态：true 表示已成交或已取消（防重放）
    mapping(bytes32 => bool) public orderInvalidated;

    /// @notice 用户最小有效 nonce（salt < minNonce 的订单无效，可用于批量取消历史订单）
    mapping(address => uint256) public minNonce;

    struct Listing {
        address nft;
        address seller;
        uint256 tokenId;
        uint256 price;
        uint256 deadline;
        uint256 salt;
    }

    struct Bid {
        address nft;
        address buyer;
        uint256 tokenId;
        uint256 price;
        uint256 deadline;
        uint256 salt;
    }

    /// @notice 上架订单类型hash
    bytes32 private constant LISTING_TYPEHASH =
        keccak256(
            "Listing(address nft,address seller,uint256 tokenId,uint256 price,uint256 deadline,uint256 salt)"
        );

    /// @notice 出价订单类型hash
    bytes32 private constant BID_TYPEHASH =
        keccak256(
            "Bid(address nft,address buyer,uint256 tokenId,uint256 price,uint256 deadline,uint256 salt)"
        );

    // Merkle leaf 与 batch 根签名类型
    bytes32 private constant LISTING_LEAF_TYPEHASH =
        keccak256(
            "ListingLeaf(address nft,address seller,uint256 tokenId,uint256 price,uint256 deadline,uint256 salt)"
        );

    /// @notice 批量上架订单类型hash
    bytes32 private constant BATCH_LISTING_TYPEHASH =
        keccak256("BatchListing(bytes32 merkleRoot,address seller,uint256 rootDeadline)");

    /// @notice 购买事件
    event Buy(
        bytes32 indexed listingHash,
        address indexed seller,
        address indexed buyer,
        address nft,
        uint256 tokenId,
        uint256 price,
        uint256 fee
    );

    /// @notice 出价接受事件
    event BidAccepted(
        bytes32 indexed listingHash,
        bytes32 indexed bidHash,
        address indexed seller,
        address buyer,
        address nft,
        uint256 tokenId,
        uint256 price,
        uint256 fee
    );

    /// @notice 订单取消事件
    event OrderCancelled(bytes32 indexed orderHash, address indexed maker, bool isListing);
    /// @notice 最小nonce更新事件
    event MinNonceUpdated(address indexed maker, uint256 newMinNonce);
    /// @notice 提现事件
    event Withdraw(address indexed sender, uint256 amount);
    /// @notice 手续费提现事件
    event FeesWithdrawn(address indexed to, uint256 amount);

    constructor(address _token) Ownable(msg.sender) EIP712("BloomMarketplace", "1") {
        require(_token != address(0), "token=0");
        token = BloomToken(_token);
    }

    /// @notice 买家购买
    function buy(
        Listing calldata listing,
        bytes calldata listingSignature,
        bytes32[] calldata proof,
        bytes32 merkleRoot,
        uint256 rootDeadline,
        bytes calldata batchSignature
    ) external returns (bytes32 listingHash) {
        listingHash = _buyInternal(listing, listingSignature, proof, merkleRoot, rootDeadline, batchSignature);
    }

    /// @notice 买家批量购买（同一笔交易内逐个结算）。
    /// @dev 支持 mixed：对非 Merkle listing 传入非空 listingSignature；对 Merkle listing 传入空 listingSignature（0x），并提供 proof/merkleRoot/rootDeadline/batchSignature。
    function buyBatch(
        Listing[] calldata listings,
        bytes[] calldata listingSignatures,
        bytes32[][] calldata proofs,
        bytes32[] calldata merkleRoots,
        uint256[] calldata rootDeadlines,
        bytes[] calldata batchSignatures
    ) external returns (bytes32[] memory listingHashes) {
        uint256 len = listings.length;
        require(
            listingSignatures.length == len &&
                proofs.length == len &&
                merkleRoots.length == len &&
                rootDeadlines.length == len &&
                batchSignatures.length == len,
            "length mismatch"
        );

        listingHashes = new bytes32[](len);
        for (uint256 i = 0; i < len; i++) {
            listingHashes[i] = _buyInternal(
                listings[i],
                listingSignatures[i],
                proofs[i],
                merkleRoots[i],
                rootDeadlines[i],
                batchSignatures[i]
            );
        }
    }

    /// @notice 单笔 buy 的内部实现（供 buy / buyBatch 复用）。
    function _buyInternal(
        Listing calldata listing,
        bytes calldata listingSignature,
        bytes32[] calldata proof,
        bytes32 merkleRoot,
        uint256 rootDeadline,
        bytes calldata batchSignature
    ) internal returns (bytes32 listingHash) {
        listingHash = _verifyListingForPurchase(
            listing,
            listingSignature,
            proof,
            merkleRoot,
            rootDeadline,
            batchSignature
        );
        require(!orderInvalidated[listingHash], "listing invalidated");
        require(listing.deadline >= block.timestamp, "listing expired");
        require(listing.seller != msg.sender, "seller=buyer");

        _validateListingNonce(listing);
        _validateSellerOwnsToken(listing.nft, listing.tokenId, listing.seller);

        _pay(msg.sender, listing.seller, listing.price);
        IERC721(listing.nft).safeTransferFrom(listing.seller, msg.sender, listing.tokenId);

        orderInvalidated[listingHash] = true;
        uint256 fee = _calcFee(listing.price);
        emit Buy(listingHash, listing.seller, msg.sender, listing.nft, listing.tokenId, listing.price, fee);
    }

    /// @notice 卖家接受买家离线出价（任意地址可代提交，便于 relay）。
    function acceptBid(
        Listing calldata listing,
        bytes calldata listingSignature,
        bytes32[] calldata proof,
        bytes32 merkleRoot,
        uint256 rootDeadline,
        bytes calldata batchSignature,
        Bid calldata bid,
        bytes calldata bidSignature
    ) external returns (bytes32 listingHash, bytes32 bidHash) {
        listingHash = _verifyListingForPurchase(listing, listingSignature, proof, merkleRoot, rootDeadline, batchSignature);
        bidHash = _verifyBid(bid, bidSignature);

        require(!orderInvalidated[listingHash], "listing invalidated");
        require(!orderInvalidated[bidHash], "bid invalidated");
        require(listing.deadline >= block.timestamp, "listing expired");
        require(bid.deadline >= block.timestamp, "bid expired");
        require(listing.seller != bid.buyer, "seller=buyer");
        require(listing.nft == bid.nft, "nft mismatch");
        require(listing.tokenId == bid.tokenId, "tokenId mismatch");

        _validateListingNonce(listing);
        _validateBidNonce(bid);
        _validateSellerOwnsToken(listing.nft, listing.tokenId, listing.seller);

        // 取消 bid.price >= listing.price 限制后，为了更清晰的错误信息，这里显式检查余额/授权。
        // 实际转账仍由 _pay 内部的 token.transferFrom 保障最终一致性。
        require(token.balanceOf(bid.buyer) >= bid.price, "insufficient bid balance");
        require(token.allowance(bid.buyer, address(this)) >= bid.price, "insufficient bid allowance");

        _pay(bid.buyer, listing.seller, bid.price);
        IERC721(listing.nft).safeTransferFrom(listing.seller, bid.buyer, listing.tokenId);

        orderInvalidated[listingHash] = true;
        orderInvalidated[bidHash] = true;

        uint256 fee = _calcFee(bid.price);
        emit BidAccepted(
            listingHash,
            bidHash,
            listing.seller,
            bid.buyer,
            listing.nft,
            listing.tokenId,
            bid.price,
            fee
        );
    }

    /// @notice 订单取消（链下订单的链上失效开关）
    function cancelListingOrder(Listing calldata listing) external returns (bytes32 listingHash) {
        require(msg.sender == listing.seller, "not seller");
        listingHash = _listingDigest(listing);
        require(!orderInvalidated[listingHash], "already invalidated");
        orderInvalidated[listingHash] = true;
        emit OrderCancelled(listingHash, msg.sender, true);
    }

    /// @notice 出价取消（链下订单的链上失效开关）
    function cancelBidOrder(Bid calldata bid) external returns (bytes32 bidHash) {
        require(msg.sender == bid.buyer, "not buyer");
        bidHash = _bidDigest(bid);
        require(!orderInvalidated[bidHash], "already invalidated");
        orderInvalidated[bidHash] = true;
        emit OrderCancelled(bidHash, msg.sender, false);
    }

    /// @notice 批量作废历史订单：订单 salt 必须 >= minNonce[maker]
    function updateMinNonce(uint256 newMinNonce) external {
        require(newMinNonce > minNonce[msg.sender], "nonce not increasing");
        minNonce[msg.sender] = newMinNonce;
        emit MinNonceUpdated(msg.sender, newMinNonce);
    }

    /// @notice 设置手续费率
    function setFeeRate(uint256 _feeRate) external onlyOwner {
        require(_feeRate <= feePrecision, "fee too high");
        feeRate = _feeRate;
    }

    /// @notice 提取合约中的原生币（误转入等）
    function withdraw() external onlyOwner {
        uint256 balance = address(this).balance;
        (bool success, ) = payable(owner()).call{value: balance}("");
        require(success, "withdraw failed");
        emit Withdraw(msg.sender, balance);
    }

    /// @notice 提取累计 BT 手续费
    function withdrawFees() external onlyOwner {
        uint256 bal = token.balanceOf(address(this));
        require(bal > 0, "no fees");
        require(token.transfer(owner(), bal), "fee transfer failed");
        emit FeesWithdrawn(owner(), bal);
    }

    /// @notice 支付手续费和卖家
    function _pay(address payer, address seller, uint256 grossAmount) internal {
        require(grossAmount > 0, "price=0");
        uint256 fee = _calcFee(grossAmount);
        require(token.transferFrom(payer, address(this), fee), "pay fee failed");
        require(token.transferFrom(payer, seller, grossAmount - fee), "pay seller failed");
    }

    /// @notice 计算手续费
    function _calcFee(uint256 amount) internal view returns (uint256) {
        return (amount * feeRate) / feePrecision;
    }

    /// @notice 验证卖家是否拥有该NFT
    function _validateSellerOwnsToken(address nft, uint256 tokenId, address seller) internal view {
        require(seller != address(0), "seller=0");
        require(nft != address(0), "nft=0");
        require(IERC721(nft).ownerOf(tokenId) == seller, "seller not owner");
    }

    /// @notice 验证上架订单的nonce
    function _validateListingNonce(Listing calldata listing) internal view {
        require(listing.salt >= minNonce[listing.seller], "listing nonce invalid");
    }

    /// @notice 验证出价订单的nonce
    function _validateBidNonce(Bid calldata bid) internal view {
        require(bid.salt >= minNonce[bid.buyer], "bid nonce invalid");
    }

    /// @notice 验证上架订单的签名
    function _verifyListingForPurchase(
        Listing calldata listing,
        bytes calldata listingSignature,
        bytes32[] calldata proof,
        bytes32 merkleRoot,
        uint256 rootDeadline,
        bytes calldata batchSignature
    ) internal view returns (bytes32 listingHash) {
        listingHash = _listingDigest(listing);

        // Merkle 路径：listingSignature 为空字节
        if (listingSignature.length == 0) {
            require(merkleRoot != bytes32(0), "merkleRoot=0");
            require(rootDeadline >= block.timestamp, "root expired");
            bytes32 leaf = _listingLeafHash(listing);
            require(MerkleProof.verify(proof, merkleRoot, leaf), "bad merkle proof");

            // 验证卖家对 (merkleRoot, seller, rootDeadline) 的 batch 签名
            bytes32 batchStructHash = keccak256(
                abi.encode(BATCH_LISTING_TYPEHASH, merkleRoot, listing.seller, rootDeadline)
            );
            bytes32 batchDigest = _hashTypedDataV4(batchStructHash);
            require(batchDigest.recover(batchSignature) == listing.seller, "bad batch sig");
            return listingHash;
        }

        // 单笔签名路径
        address signer = listingHash.recover(listingSignature);
        require(signer == listing.seller, "bad listing sig");
    }

    /// @notice 验证出价订单的签名
    function _verifyBid(Bid calldata bid, bytes calldata signature) internal view returns (bytes32 bidHash) {
        bidHash = _bidDigest(bid);
        require(bidHash.recover(signature) == bid.buyer, "bad bid sig");
    }

    /// @notice 计算上架订单的digest
    function _listingDigest(Listing calldata listing) internal view returns (bytes32) {
        bytes32 structHash = keccak256(
            abi.encode(
                LISTING_TYPEHASH,
                listing.nft,
                listing.seller,
                listing.tokenId,
                listing.price,
                listing.deadline,
                listing.salt
            )
        );
        return _hashTypedDataV4(structHash);
    }

    /// @notice 计算出价订单的digest
    function _bidDigest(Bid calldata bid) internal view returns (bytes32) {
        bytes32 structHash = keccak256(
            abi.encode(
                BID_TYPEHASH,
                bid.nft,
                bid.buyer,
                bid.tokenId,
                bid.price,
                bid.deadline,
                bid.salt
            )
        );
        return _hashTypedDataV4(structHash);
    }

    /// @notice 计算 listing leaf hash（供 Merkle 校验）
    function _listingLeafHash(Listing calldata listing) internal pure returns (bytes32) {
        return
            keccak256(
                abi.encode(
                    LISTING_LEAF_TYPEHASH,
                    listing.nft,
                    listing.seller,
                    listing.tokenId,
                    listing.price,
                    listing.deadline,
                    listing.salt
                )
            );
    }
}
