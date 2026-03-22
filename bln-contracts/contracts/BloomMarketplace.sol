// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/cryptography/EIP712.sol";
import "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";
import "@openzeppelin/contracts/token/ERC721/IERC721.sol";
import "./BloomToken.sol";

// 市场合约
contract BloomMarketplace is Ownable, EIP712 {
    using ECDSA for bytes32;

    // 手续费率
    uint256 public feeRate = 25;

    // 手续费精度
    uint256 public feePrecision = 1000;

    // 代币合约
    BloomToken public token;

    // 提款（原生币，误转入等）
    event Withdraw(address indexed sender, uint256 amount);

    /// @notice 提取市场累计的 BT 手续费（合约内 BT 余额减去买家出价托管总额）
    event FeesWithdrawn(address indexed to, uint256 amount);

    /// @notice 当前托管于本合约的出价 BT 总额（与 bidEscrow 映射同步）
    uint256 public totalBidEscrow;

    // 上架
    event Listed(bytes32 indexed listingHash, address indexed nft, address indexed seller, uint256 tokenId, uint256 price, uint256 deadline, uint256 nonce, uint256 salt);
    // 取消上架
    event ListingCancelled(bytes32 indexed listingHash, address indexed seller);
    // 购买
    event Buy(bytes32 indexed listingHash, address indexed buyer);
 
    // 出价
    event BidPlaced(bytes32 indexed bidhash, bytes32 indexed listingHash, address indexed buyer, uint256 price, uint256 deadline, uint256 nonce);
    // 撤回出价
    event BidCancelled(bytes32 indexed bidhash, address indexed buyer);
    // 接受出价
    event BidAccepted(bytes32 indexed listingHash, bytes32 indexed bidHash, address seller, address buyer);
    // 未中标出价退款完成
    event BidRefunded(bytes32 indexed bidHash, address indexed buyer, uint256 amount);
    /// @notice 卖家链上降价（NFT 仍托管在市场合约，不先 cancel）
    event ListingPriceReduced(bytes32 indexed listingHash, address indexed seller, uint256 newPrice);

    // 上架信息
    struct Listing {
        address nft; // NFT合约地址
        address seller; // 卖家地址
        uint256 tokenId; // tokenId
        uint256 price; // 价格
        uint256 deadline; // 截止时间
        uint256 nonce; // 非重复 nonce（链上顺序）
        uint256 salt; // 订单盐（保证 EIP-712 哈希唯一，可与 nonce 并存）
    }

    // 出价信息
    struct Bid {
        bytes32 listingHash; // 上架hash
        address buyer; // 买家地址
        uint256 price; // 价格
        uint256 deadline; // 截止时间
        uint256 nonce; // 非重复 nonce
    }

    // 非重复 nonce
    mapping(address => uint256) public nonces;

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

    // 上架类型hash
    bytes32 private constant LISTING_TYPEHASH =
        keccak256(
            "Listing(address nft,address seller,uint256 tokenId,uint256 price,uint256 deadline,uint256 nonce,uint256 salt)"
        );

    // 出价类型hash
    bytes32 private constant BID_TYPEHASH =
        keccak256(
            "Bid(bytes32 listingHash,address buyer,uint256 price,uint256 deadline,uint256 nonce)"
        );

    bytes32 private constant PRICE_REDUCTION_TYPEHASH =
        keccak256(
            "PriceReduction(bytes32 listingHash,address seller,uint256 newPrice,uint256 nonce)"
        );

    // 是否上架
    modifier isListing (bytes32 listingHash){
        require(listings[listingHash], "not listing");
        _;
    }

    // 是否出价
    modifier isBid (bytes32 bidHash){
        require(bids[bidHash], "not bid");
        _;
    }

    // 构造函数
    constructor(address _token) Ownable(msg.sender) EIP712("BloomMarketplace", "1")
    {
        token = BloomToken(_token);
    }

    // EIP712 签名上架（托管到市场合约）
    // 前置条件：卖家对市场合约做过 approve 或 setApprovalForAll
    function listWithSig( Listing calldata listing, bytes calldata signature) external returns (bytes32 listingHash) {
        require(listing.seller != address(0), "seller=0");
        require(listing.nft != address(0), "nft=0");
        require(listing.price > 0, "price=0");
        require(listing.deadline >= block.timestamp, "expired");

        // 
        require(listing.nonce == nonces[listing.seller], "bad nonce");
        nonces[listing.seller] = listing.nonce + 1;

        listingHash = _verifyListing(listing, signature);
        require(!listings[listingHash], "listing is exist");
        require(!sold[listingHash], "already sold");
        listings[listingHash] = true;
        listingSeller[listingHash] = listing.seller;
        listingOriginalPrice[listingHash] = listing.price;
        listingPriceOverride[listingHash] = 0;

        // 托管 NFT 到市场合约（任何人都可代提交签名，上架由合约执行）
        IERC721(listing.nft).safeTransferFrom(listing.seller, address(this), listing.tokenId);
        emit Listed(
            listingHash,
            listing.nft,
            listing.seller,
            listing.tokenId,
            listing.price,
            listing.deadline,
            listing.nonce,
            listing.salt
        );
    }

    /// @notice 卖家签名降价：NFT 继续托管，不调用 cancelListing
    function reduceListingPrice(
        bytes32 listingHash_,
        address seller,
        uint256 newPrice,
        uint256 nonce,
        bytes calldata signature
    ) external {
        require(listings[listingHash_], "not listing");
        require(!sold[listingHash_], "already sold");
        require(listingSeller[listingHash_] == seller, "not seller");
        require(newPrice > 0, "price=0");
        uint256 eff = _effectivePrice(listingHash_);
        require(newPrice < eff, "not lower");
        require(nonce == reductionNonces[seller], "bad reduction nonce");

        bytes32 structHash = keccak256(
            abi.encode(PRICE_REDUCTION_TYPEHASH, listingHash_, seller, newPrice, nonce)
        );
        bytes32 digest = _hashTypedDataV4(structHash);
        address signer = digest.recover(signature);
        require(signer == seller, "bad reduction sig");

        reductionNonces[seller] = nonce + 1;
        listingPriceOverride[listingHash_] = newPrice;
        emit ListingPriceReduced(listingHash_, seller, newPrice);
    }

    // 取消上架（不依赖链上存储 Listing 元数据）
    // 调用方必须提供原始 listing 字段，合约重算 hash 后校验并退回 NFT
    function cancelListing(Listing calldata listing) external {
        // 只允许卖家本人取消
        require(msg.sender == listing.seller, "not seller");

        // 与 listWithSig 同一规则重算 listingHash
        bytes32 structHash = keccak256(
            abi.encode(
                LISTING_TYPEHASH,
                listing.nft,
                listing.seller,
                listing.tokenId,
                listing.price,
                listing.deadline,
                listing.nonce,
                listing.salt
            )
        );
        bytes32 listingHash = _hashTypedDataV4(structHash);

        // 必须当前处于上架状态且未成交
        require(listings[listingHash], "not listing");
        require(!sold[listingHash], "already sold");

        // 先关单再转 NFT（防重入）
        listings[listingHash] = false;
        _clearListingPricing(listingHash);

        // NFT 从市场合约托管地址退回卖家
        IERC721(listing.nft).safeTransferFrom(address(this), listing.seller, listing.tokenId);

        emit ListingCancelled(listingHash, msg.sender);
    }

    // 用户购买
    function buy(Listing calldata listing, bytes calldata signature) external returns(bytes32 buyHash){
        require(listing.deadline >= block.timestamp, "expired");
        require(listing.seller != address(0), "seller=0");
        require(listing.seller != msg.sender, "seller=buyer");
        require(listing.price > 0, "price=0");

        bytes32 listingHash = _verifyListing(listing, signature);
        require(listings[listingHash], "not listing");
        require(!sold[listingHash], "already sold");

        uint256 payPrice = _effectivePrice(listingHash);
        // 结算：买家付款（BloomToken），市场抽成，卖家收款（按当前有效价）
        uint256 fee = payPrice * feeRate / feePrecision;
        require(token.transferFrom(msg.sender, address(this), fee), "pay fee failed");
        require(
            token.transferFrom(msg.sender, listing.seller, payPrice - fee),
            "pay price failed"
        );

        // NFT 从市场托管地址转给买家
        IERC721(listing.nft).safeTransferFrom(address(this), msg.sender, listing.tokenId);

        // 标记成交并关闭挂单
        sold[listingHash] = true;
        listings[listingHash] = false;
        _clearListingPricing(listingHash);

        // 返回购买哈希（不入库存储，仅用于链下追踪）
        buyHash = keccak256(abi.encode(listingHash, msg.sender));

        emit Buy(listingHash, msg.sender);
    }

    // 用户出价（签名订单）
    function bidWithSig(Bid calldata bid, bytes calldata signature) external  returns(bytes32 bidHash){
        require(bid.buyer != address(0), "buyer=0");
        require(bid.price > 0, "price=0");
        require(bid.deadline >= block.timestamp, "expired");
        require(listings[bid.listingHash], "not listing");
        require(!sold[bid.listingHash], "already sold");
        require(bid.price <= _effectivePrice(bid.listingHash), "bid too high");
        require(bid.nonce == nonces[bid.buyer], "bad nonce");
        nonces[bid.buyer] = bid.nonce + 1;
        bidHash = _verifyBid(bid, signature);
        require(!bids[bidHash], "bis is exsit");

        // 出价即托管资金到市场合约
        require(token.transferFrom(bid.buyer, address(this), bid.price), "escrow failed");
        bids[bidHash] = true;
        bidEscrow[bidHash] = bid.price;
        totalBidEscrow += bid.price;

        emit BidPlaced(bidHash, bid.listingHash, bid.buyer, bid.price, bid.deadline, bid.nonce);
    }

    // 撤回出价（参考 cancelListing，不依赖签名参数）
    function cancelBid(Bid calldata bid) external {
        // 与 placeBid 同一规则重算 bidHash
        bytes32 structHash = keccak256(
            abi.encode(
                BID_TYPEHASH,
                bid.listingHash,
                bid.buyer,
                bid.price,
                bid.deadline,
                bid.nonce
            )
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

        // 撤回时原路退款给买家
        require(token.transfer(bid.buyer, escrowAmount), "refund failed");
        emit BidCancelled(bidHash, msg.sender);
    }

    /// @notice 挂单已成交后，未中标的买家取回托管的 BT（原 cancelBid 在 sold 后禁止调用）
    /// @dev acceptBid 会写入 winningBid；若通过 buy() 成交则 winner 为空，此时任意仍托管的出价均可退款
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
    function acceptBid(
        Listing calldata listing,
        bytes calldata listingSignature,
        Bid calldata bid,
        bytes calldata bidSignature
    ) external {

        require(listing.deadline >= block.timestamp, "listing expired");
        require(bid.deadline >= block.timestamp, "bid expired");
        require(listing.seller != bid.buyer, "seller=buyer");

        bytes32 listingHash = _verifyListing(listing, listingSignature);
        bytes32 bidHash = _verifyBid(bid, bidSignature);
        require(listings[listingHash], "not listing");
        require(!sold[listingHash], "already sold");
        require(bids[bidHash], "bot bid");
        // Bid 中记录的 listingHash 必须匹配
        require(bid.listingHash == listingHash, "listingHash mismatch");
        // 不在此用 _effectivePrice 限制 bid.price：卖家降价后，历史出价可能高于当前有效挂单价，
        // 只要 bid 当时已 bidWithSig 成功且托管一致，仍应允许成交。

        uint256 escrowAmount = bidEscrow[bidHash];
        require(escrowAmount == bid.price, "bad escrow");

        // 先记录中标 bidHash，再清空托管（便于未中标出价后续退款时校验）
        winningBidByListing[listingHash] = bidHash;

        // 3. 结算：从托管款分账，市场抽成，卖家收款，NFT 转给买家
        uint256 fee = bid.price * feeRate / feePrecision;
        require(token.transfer(listing.seller, bid.price - fee), "pay price failed");

        // NFT 从市场托管地址转给买家
        IERC721(listing.nft).safeTransferFrom(address(this), bid.buyer, listing.tokenId);

        // 4. 状态更新，防止重复成交
        sold[listingHash] = true;
        listings[listingHash] = false;
        bids[bidHash] = false;
        bidEscrow[bidHash] = 0;
        totalBidEscrow -= bid.price;
        _clearListingPricing(listingHash);

        emit BidAccepted(listingHash, bidHash, listing.seller, bid.buyer);
    }

    // 设置手续费率
    function setFeeRate(uint256 _feeRate) external onlyOwner{
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

    // 有效价格
    function _effectivePrice(bytes32 listingHash_) internal view returns (uint256) {
        uint256 orig = listingOriginalPrice[listingHash_];
        require(orig > 0, "unknown listing");
        uint256 ov = listingPriceOverride[listingHash_];
        return ov != 0 ? ov : orig;
    }

    // 清空上架价格
    function _clearListingPricing(bytes32 listingHash_) internal {
        delete listingSeller[listingHash_];
        delete listingOriginalPrice[listingHash_];
        delete listingPriceOverride[listingHash_];
    }

    // 验证Listing签名
    function _verifyListing(Listing calldata listing, bytes calldata signature)
        private
        view
        returns (bytes32 listingHash)
    {
        bytes32 structHash = keccak256(
            abi.encode(
                LISTING_TYPEHASH,
                listing.nft,
                listing.seller,
                listing.tokenId,
                listing.price,
                listing.deadline,
                listing.nonce,
                listing.salt
            )
        );
        listingHash = _hashTypedDataV4(structHash);
        address signer = listingHash.recover(signature);
        require(signer == listing.seller, "bad listing sig");
    }

    // 验证bid签名
    function _verifyBid(Bid calldata bid, bytes calldata signature)
        private
        view
        returns (bytes32 bidHash)
    {
        bytes32 structHash = keccak256(
            abi.encode(
                BID_TYPEHASH,
                bid.listingHash,
                bid.buyer,
                bid.price,
                bid.deadline,
                bid.nonce
            )
        );
        bidHash = _hashTypedDataV4(structHash);
        address signer = bidHash.recover(signature);
        require(signer == bid.buyer, "bad bid sig");
    }
}