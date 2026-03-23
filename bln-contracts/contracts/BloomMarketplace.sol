// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/cryptography/EIP712.sol";
import "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";
import "@openzeppelin/contracts/utils/cryptography/MerkleProof.sol";
import "@openzeppelin/contracts/token/ERC721/IERC721.sol";
import "./BloomToken.sol";

// 市场合约
contract BloomMarketplace is Ownable, EIP712 {
    using ECDSA for bytes32;

    // 代币合约
    BloomToken public token;

    // 手续费率
    uint256 public feeRate = 25;

    // 手续费精度
    uint256 public feePrecision = 1000;
    
    // 当前托管于本合约的出价 BT 总额（与 bidEscrow 映射同步）
    uint256 public totalBidEscrow;

    // 提款（原生币，误转入等）
    event Withdraw(address indexed sender, uint256 amount);

    /// @notice 提取市场累计的 BT 手续费（合约内 BT 余额减去买家出价托管总额）
    event FeesWithdrawn(address indexed to, uint256 amount);

    // 上架
    event Listed(bytes32 indexed listingHash, address indexed nft, address indexed seller, uint256 tokenId, uint256 price, uint256 deadline, uint256 salt);
    
    // 取消上架
    event ListingCancelled(bytes32 indexed listingHash, address indexed seller);
    // 购买
    event Buy(bytes32 indexed listingHash, address indexed buyer);

    // 出价
    event BidPlaced(bytes32 indexed bidhash, bytes32 indexed listingHash, address indexed buyer, uint256 price, uint256 deadline, uint256 salt);
    
    // 撤回出价
    event BidCancelled(bytes32 indexed bidhash, address indexed buyer);
    
    // 接受出价
    event BidAccepted(bytes32 indexed listingHash, bytes32 indexed bidHash, address seller, address buyer);
    
    // 未中标出价退款完成
    event BidRefunded(bytes32 indexed bidHash, address indexed buyer, uint256 amount);
    
    // 卖家链上降价（NFT 仍托管在市场合约，不先 cancel）
    event ListingPriceReduced(bytes32 indexed listingHash, address indexed seller, uint256 newPrice);

    // 上架信息
    struct Listing {
        address nft; // NFT合约地址
        address seller; // 卖家地址
        uint256 tokenId; // tokenId
        uint256 price; // 价格
        uint256 deadline; // 截止时间
        uint256 salt; // 订单盐（保证 EIP-712 哈希唯一，可与 nonce 并存）
    }

    // 出价信息
    struct Bid {
        bytes32 listingHash; // 上架hash
        address buyer; // 买家地址
        uint256 price; // 价格
        uint256 deadline; // 截止时间
        uint256 salt; // 出价盐（用于唯一标识 bid）
    }

    // 上架 上架hash=>bool
    mapping(bytes32 => bool) public listings;

    // 出价 出价hash=>bool
    mapping(bytes32 => bool) public bids;

    // 托管出价金额 出价hash=>金额
    mapping(bytes32 => uint256) public bidEscrow;

    // 成交状态（仅针对 listingHash）
    mapping(bytes32 => bool) public sold;

    /// @notice 某 listing 成交后的中标 bidHash（用于区分未中标出价，以便退款）
    mapping(bytes32 => bytes32) public winningBidByListing;

    /// @notice 上架时的卖家（用于降价校验）
    mapping(bytes32 => address) public listingSeller;
    
    /// @notice 上架签名中的原价（wei）
    mapping(bytes32 => uint256) public listingOriginalPrice;

    /// @notice 非零时表示当前有效售价为覆盖价（仅可 <= 原价，且仅能通过 reduceListingPrice 降低）
    mapping(bytes32 => uint256) public listingPriceOverride;

    /// @notice 卖家「降价签名」专用 nonce，与挂单/出价 nonces 独立
    mapping(address => uint256) public reductionNonces;

    /// @notice 通过 Merkle 批量授权上架的订单：购买/接受出价时无需 Listing EIP-712 单笔签名
    mapping(bytes32 => bool) public merkleListed;

    // 上架类型hash (nft, seller, tokenId, price, deadline, salt)
    bytes32 private constant LISTING_TYPEHASH =
        keccak256(
            "Listing(address nft,address seller,uint256 tokenId,uint256 price,uint256 deadline,uint256 salt)"
        );

    // 上架叶子类型hash (nft, seller, tokenId, price, deadline, salt)
    bytes32 private constant LISTING_LEAF_TYPEHASH =
        keccak256(
            "ListingLeaf(address nft,address seller,uint256 tokenId,uint256 price,uint256 deadline,uint256 salt)"
        );

    // 批量上架类型hash (merkleRoot, seller, rootDeadline)
    bytes32 private constant BATCH_LISTING_TYPEHASH =
        keccak256("BatchListing(bytes32 merkleRoot,address seller,uint256 rootDeadline)");

    // 出价类型hash (listingHash, buyer, price, deadline, salt)
    bytes32 private constant BID_TYPEHASH =
        keccak256(
            "Bid(bytes32 listingHash,address buyer,uint256 price,uint256 deadline,uint256 salt)"
        );

    // 降价类型hash (listingHash, seller, newPrice, nonce)
    bytes32 private constant PRICE_REDUCTION_TYPEHASH =
        keccak256(
            "PriceReduction(bytes32 listingHash,address seller,uint256 newPrice,uint256 nonce)"
        );

    // 是否上架
    modifier isListing(bytes32 listingHash) {
        require(listings[listingHash], "not listing");
        _;
    }

    // 是否出价
    modifier isBid(bytes32 bidHash) {
        require(bids[bidHash], "not bid");
        _;
    }

    // 构造函数
    constructor(address _token) Ownable(msg.sender) EIP712("BloomMarketplace", "1") {
        token = BloomToken(_token);
    }

    // EIP712 签名上架（托管到市场合约）
    // 前置条件：卖家对市场合约做过 approve 或 setApprovalForAll
    function listWithSig(Listing calldata listing, bytes calldata signature) external returns (bytes32 listingHash) {
        require(listing.seller != address(0), "seller=0");
        require(listing.nft != address(0), "nft=0");
        require(listing.price > 0, "price=0");
        require(listing.deadline >= block.timestamp, "expired");

        listingHash = _verifyListingStrict(listing, signature);
        require(!listings[listingHash], "listing is exist");
        require(!sold[listingHash], "already sold");
        listings[listingHash] = true;
        listingSeller[listingHash] = listing.seller;
        listingOriginalPrice[listingHash] = listing.price;
        listingPriceOverride[listingHash] = 0;

        IERC721(listing.nft).safeTransferFrom(listing.seller, address(this), listing.tokenId);
        _emitListed(listingHash, listing);
    }

    /// @notice Merkle 批量上架：叶子为 ListingLeaf；Listing.nonce 必须为 0；salt 在批量内唯一
    /// @param batchSignature 对 (merkleRoot, seller, rootDeadline) 的 EIP-712 签名
    function listWithMerkleProof(
        Listing calldata listing,
        bytes32[] calldata proof,
        bytes32 merkleRoot,
        uint256 rootDeadline,
        bytes calldata batchSignature
    ) external returns (bytes32 listingHash) {
        require(listing.seller != address(0), "seller=0");
        require(listing.nft != address(0), "nft=0");
        require(listing.price > 0, "price=0");
        require(listing.deadline >= block.timestamp, "expired");
        require(rootDeadline >= block.timestamp, "root expired");

        // 验证 Merkle 证明
        bytes32 leaf = _listingLeafHash(listing);
        require(MerkleProof.verify(proof, merkleRoot, leaf), "bad merkle proof");

        // 组装批次签名 
        bytes32 batchStructHash = keccak256(abi.encode(BATCH_LISTING_TYPEHASH, merkleRoot, listing.seller, rootDeadline));
        // 计算批次签名
        bytes32 batchDigest = _hashTypedDataV4(batchStructHash);
        require(batchDigest.recover(batchSignature) == listing.seller, "bad batch sig");

        // 计算 listing hash
        listingHash = _listingDigest(listing);
        // 检查 listing 是否存在
        require(!listings[listingHash], "listing is exist");
        // 检查 listing 是否已售出
        require(!sold[listingHash], "already sold");
        // 设置 listing 状态
        listings[listingHash] = true;
        // 设置 merkleListed 状态
        merkleListed[listingHash] = true;
        // 设置 listingSeller
        listingSeller[listingHash] = listing.seller;
        listingOriginalPrice[listingHash] = listing.price;
        listingPriceOverride[listingHash] = 0;

        IERC721(listing.nft).safeTransferFrom(listing.seller, address(this), listing.tokenId);
        _emitListed(listingHash, listing);
    }

    /// @notice 卖家签名降价：NFT 继续托管，不调用 cancelListing
    function reduceListingPrice(bytes32 listingHash_, address seller, uint256 newPrice, uint256 nonce, bytes calldata signature) external {
        require(listings[listingHash_], "not listing");
        require(!sold[listingHash_], "already sold");
        require(listingSeller[listingHash_] == seller, "not seller");
        require(newPrice > 0, "price=0");
        uint256 eff = _effectivePrice(listingHash_);
        require(newPrice < eff, "not lower");
        require(nonce == reductionNonces[seller], "bad reduction nonce");

        bytes32 structHash = keccak256(abi.encode(PRICE_REDUCTION_TYPEHASH, listingHash_, seller, newPrice, nonce));
        bytes32 digest = _hashTypedDataV4(structHash);
        address signer = digest.recover(signature);
        require(signer == seller, "bad reduction sig");

        reductionNonces[seller] = nonce + 1;
        listingPriceOverride[listingHash_] = newPrice;
        emit ListingPriceReduced(listingHash_, seller, newPrice);
    }

    // 取消上架
    function cancelListing(Listing calldata listing) external {
        _cancelListingInner(listing);
    }

    /// @notice 批量下架：每笔须为同一 msg.sender 且等于对应 listing.seller
    function cancelListingsBatch(Listing[] calldata listings_) external {
        for (uint256 i = 0; i < listings_.length; i++) {
            _cancelListingInner(listings_[i]);
        }
    }

    // 取消上架内部实现
    function _cancelListingInner(Listing calldata listing) internal {
        require(msg.sender == listing.seller, "not seller");
        bytes32 listingHash = _listingDigest(listing);
        require(listings[listingHash], "not listing");
        require(!sold[listingHash], "already sold");

        listings[listingHash] = false;
        _clearListingPricing(listingHash);

        IERC721(listing.nft).safeTransferFrom(address(this), listing.seller, listing.tokenId);

        emit ListingCancelled(listingHash, msg.sender);
    }

    // 购买
    function buy(Listing calldata listing, bytes calldata signature) external returns (bytes32 buyHash) {
        buyHash = _buyInternal(listing, signature, msg.sender);
    }

    /// @notice 购物车：多笔挂单一次成交（原子性：任一笔失败则整笔回滚）
    function buyBatch(Listing[] calldata listings_, bytes[] calldata listingSignatures) external returns (bytes32[] memory buyHashes) {
        uint256 n = listings_.length;
        require(n == listingSignatures.length, "len mismatch");
        require(n > 0, "empty");
        buyHashes = new bytes32[](n);
        bytes32[] memory seen = new bytes32[](n);
        for (uint256 i = 0; i < n; i++) {
            bytes32 h = _listingDigest(listings_[i]);
            for (uint256 j = 0; j < i; j++) {
                require(h != seen[j], "duplicate listing");
            }
            seen[i] = h;
        }
        for (uint256 i = 0; i < n; i++) {
            buyHashes[i] = _buyInternal(listings_[i], listingSignatures[i], msg.sender);
        }
    }

    // 购买内部实现
    function _buyInternal(Listing calldata listing, bytes calldata signature, address buyer) internal returns (bytes32 buyHash) {
        require(listing.deadline >= block.timestamp, "expired");
        require(listing.seller != address(0), "seller=0");
        require(listing.seller != buyer, "seller=buyer");
        require(listing.price > 0, "price=0");

        bytes32 listingHash = _verifyListingForPurchase(listing, signature);
        require(listings[listingHash], "not listing");
        require(!sold[listingHash], "already sold");

        uint256 payPrice = _effectivePrice(listingHash);
        uint256 fee = payPrice * feeRate / feePrecision;
        require(token.transferFrom(buyer, address(this), fee), "pay fee failed");
        require(token.transferFrom(buyer, listing.seller, payPrice - fee), "pay price failed");

        IERC721(listing.nft).safeTransferFrom(address(this), buyer, listing.tokenId);

        sold[listingHash] = true;
        listings[listingHash] = false;
        _clearListingPricing(listingHash);

        buyHash = keccak256(abi.encode(listingHash, buyer));
        emit Buy(listingHash, buyer);
    }

    // 用户出价（签名订单）
    function bidWithSig(Bid calldata bid, bytes calldata signature) external returns (bytes32 bidHash) {
        require(bid.buyer != address(0), "buyer=0");
        require(bid.price > 0, "price=0");
        require(bid.deadline >= block.timestamp, "expired");
        require(listings[bid.listingHash], "not listing");
        require(!sold[bid.listingHash], "already sold");
        require(bid.price <= _effectivePrice(bid.listingHash), "bid too high");
        bidHash = _verifyBid(bid, signature);
        require(!bids[bidHash], "bis is exsit");

        require(token.transferFrom(bid.buyer, address(this), bid.price), "escrow failed");
        bids[bidHash] = true;
        bidEscrow[bidHash] = bid.price;
        totalBidEscrow += bid.price;

        emit BidPlaced(bidHash, bid.listingHash, bid.buyer, bid.price, bid.deadline, bid.salt);
    }

    // 撤回出价（参考 cancelListing，不依赖签名参数）
    function cancelBid(Bid calldata bid) external {
        bytes32 structHash = keccak256(
            abi.encode(BID_TYPEHASH, bid.listingHash, bid.buyer, bid.price, bid.deadline, bid.salt)
        );
        bytes32 bidHash = _hashTypedDataV4(structHash);

        require(bids[bidHash], "not bid");
        require(msg.sender == bid.buyer, "not buyer");
        require(!sold[bid.listingHash], "already sold");

        uint256 escrowAmount = bidEscrow[bidHash];
        require(escrowAmount > 0, "no escrow");

        bids[bidHash] = false;
        bidEscrow[bidHash] = 0;
        totalBidEscrow -= escrowAmount;

        require(token.transfer(bid.buyer, escrowAmount), "refund failed");
        emit BidCancelled(bidHash, msg.sender);
    }

    /// @notice 挂单已成交后，未中标的买家取回托管的 BT（原 cancelBid 在 sold 后禁止调用）
    function refundLosingBid(Bid calldata bid, bytes calldata bidSignature) external {
        bytes32 bidHash = _verifyBid(bid, bidSignature);
        require(sold[bid.listingHash], "listing not sold");
        bytes32 winner = winningBidByListing[bid.listingHash];
        if (winner != bytes32(0)) {
            require(bidHash != winner, "winning bid");
        }
        require(bids[bidHash], "not bid");
        require(msg.sender == bid.buyer, "not buyer");

        uint256 escrowAmount = bidEscrow[bidHash];
        require(escrowAmount > 0, "no escrow");

        bids[bidHash] = false;
        bidEscrow[bidHash] = 0;
        totalBidEscrow -= escrowAmount;

        require(token.transfer(bid.buyer, escrowAmount), "refund failed");
        emit BidRefunded(bidHash, bid.buyer, escrowAmount);
    }

    // 接受出价（链下订单：Listing + Bid 均由对应方 EIP-712 签名；任意地址可提交，便于后端/中继代付 gas）
    function acceptBid(Listing calldata listing, bytes calldata listingSignature, Bid calldata bid, bytes calldata bidSignature) external {
        require(listing.deadline >= block.timestamp, "listing expired");
        require(bid.deadline >= block.timestamp, "bid expired");
        require(listing.seller != bid.buyer, "seller=buyer");

        bytes32 listingHash = _verifyListingForPurchase(listing, listingSignature);
        bytes32 bidHash = _verifyBid(bid, bidSignature);
        require(listings[listingHash], "not listing");
        require(!sold[listingHash], "already sold");
        require(bids[bidHash], "bot bid");
        require(bid.listingHash == listingHash, "listingHash mismatch");

        uint256 escrowAmount = bidEscrow[bidHash];
        require(escrowAmount == bid.price, "bad escrow");

        winningBidByListing[listingHash] = bidHash;

        uint256 fee = bid.price * feeRate / feePrecision;
        require(token.transfer(listing.seller, bid.price - fee), "pay price failed");

        IERC721(listing.nft).safeTransferFrom(address(this), bid.buyer, listing.tokenId);

        sold[listingHash] = true;
        listings[listingHash] = false;
        bids[bidHash] = false;
        bidEscrow[bidHash] = 0;
        totalBidEscrow -= bid.price;
        _clearListingPricing(listingHash);

        emit BidAccepted(listingHash, bidHash, listing.seller, bid.buyer);
    }

    // 设置手续费率
    function setFeeRate(uint256 _feeRate) external onlyOwner {
        require(_feeRate <= feePrecision, "feeRate is greater than feePrecision");
        feeRate = _feeRate;
    }

    // 取款（原生币）
    function withdraw() external onlyOwner {
        uint256 balance = address(this).balance;
        (bool success, ) = payable(owner()).call{value: balance}("");
        require(success, "withdraw failed");
        emit Withdraw(msg.sender, balance);
    }

    /// @notice 将合约内累计的 BT 手续费提至 owner（余额扣除当前全部出价托管额，仅 owner）
    function withdrawFees() external onlyOwner {
        uint256 bal = token.balanceOf(address(this));
        require(bal >= totalBidEscrow, "escrow invariant");
        uint256 withdrawable = bal - totalBidEscrow;
        require(withdrawable > 0, "no fees");
        require(token.transfer(owner(), withdrawable), "fee transfer failed");
        emit FeesWithdrawn(owner(), withdrawable);
    }

    // 让本合约能接收 safeTransferFrom 的 ERC721
    function onERC721Received(address, address, uint256, bytes calldata) external pure returns (bytes4) {
        return this.onERC721Received.selector;
    }

    // @notice 当前有效挂单价（wei）：有 override 用 override，否则原价
    function effectiveListingPrice(bytes32 listingHash_) external view returns (uint256) {
        return _effectivePrice(listingHash_);
    }

    function _effectivePrice(bytes32 listingHash_) internal view returns (uint256) {
        uint256 orig = listingOriginalPrice[listingHash_];
        require(orig > 0, "unknown listing");
        uint256 ov = listingPriceOverride[listingHash_];
        return ov != 0 ? ov : orig;
    }

    // 清除上架价格
    function _clearListingPricing(bytes32 listingHash_) internal {
        delete listingSeller[listingHash_];
        delete listingOriginalPrice[listingHash_];
        delete listingPriceOverride[listingHash_];
        delete merkleListed[listingHash_];
    }

    // 触发上架事件
    function _emitListed(bytes32 listingHash, Listing calldata listing) private {
        emit Listed(
            listingHash,
            listing.nft,
            listing.seller,
            listing.tokenId,
            listing.price,
            listing.deadline,
            listing.salt
        );
    }

    // 计算 listing leaf hash
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

    // 计算 listing digest
    function _listingDigest(Listing calldata listing) private view returns (bytes32) {
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

    /// @notice listWithSig：必须单笔 Listing EIP-712 签名
    function _verifyListingStrict(Listing calldata listing, bytes calldata signature) private view returns (bytes32 listingHash) {
        listingHash = _listingDigest(listing);
        address signer = listingHash.recover(signature);
        require(signer == listing.seller, "bad listing sig");
    }

    /// @notice buy / acceptBid：Merkle 上架则 listingSignature 为空
    function _verifyListingForPurchase(Listing calldata listing, bytes calldata signature) private view returns (bytes32 listingHash) {
        listingHash = _listingDigest(listing);
        if (merkleListed[listingHash]) {
            require(signature.length == 0, "merkle listing: empty sig");
            return listingHash;
        }
        address signer = listingHash.recover(signature);
        require(signer == listing.seller, "bad listing sig");
        return listingHash;
    }

    // 验证出价
    function _verifyBid(Bid calldata bid, bytes calldata signature) private view returns (bytes32 bidHash) {
        bytes32 structHash = keccak256(abi.encode(BID_TYPEHASH, bid.listingHash, bid.buyer, bid.price, bid.deadline, bid.salt));
        bidHash = _hashTypedDataV4(structHash);
        address signer = bidHash.recover(signature);
        require(signer == bid.buyer, "bad bid sig");
    }
}
