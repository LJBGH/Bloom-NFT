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

    // 提款
    event Withdraw(address indexed sender, uint256 amount);

    // 上架
    event Listed(address indexed nft, address indexed seller, uint256 indexed tokenId, uint256 price, uint256 deadline, uint256 nonce);
    // 更改价格
    event ListingPriceChanged(bytes32 indexed listingHash, address indexed seller, uint256 newPrice);
    // 取消上架
    event ListingCancelled(bytes32 indexed listingHash, address indexed seller);
 
    // 出价
    event BidPlaced(bytes32 indexed listingHash, address indexed buyer, uint256 price, uint256 deadline, uint256 nonce);
    // 撤回出价
    event BidCancelled(bytes32 indexed bidhash, address indexed buyer);
    // 接受出价
    event BidAccepted(bytes32 indexed listingHash, bytes32 indexed bidhash, address indexed buyer);

    // 上架信息
    struct Listing {
        address nft; // NFT合约地址
        address seller; // 卖家地址
        uint256 tokenId; // tokenId
        uint256 price; // 价格
        uint256 deadline; // 截止时间
        uint256 nonce; // 非重复 nonce
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
    // 购买 出价hash=>bool
    mapping(bytes32 => bool) public sold;


    // // 出价信息 
    // mapping(bytes32 => Bid) public bids;
    // // 上架信息 
    // mapping(bytes32 => Listing) public listings;

    // 上架类型hash
    bytes32 private constant LISTING_TYPEHASH =
        keccak256(
            "Listing(address nft,address seller,uint256 tokenId,uint256 price,uint256 deadline,uint256 nonce)"
        );

    // 出价类型hash
    bytes32 private constant BID_TYPEHASH =
        keccak256(
            "Bid(bytes32 listingHash,address buyer,uint256 price,uint256 deadline,uint256 nonce)"
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

        // 托管 NFT 到市场合约（任何人都可代提交签名，上架由合约执行）
        IERC721(listing.nft).safeTransferFrom(listing.seller, address(this), listing.tokenId);
        emit Listed(
            listing.nft,
            listing.seller,
            listing.tokenId,
            listing.price,
            listing.deadline,
            listing.nonce
        );
    }

    // 更改价格
    function changePrice(bytes32 listingHash, uint256 newPrice) external isListing(listingHash) returns(bytes32){
        require(!sold[listingHash], "already sold");
        require(newPrice > 0, "newPrice is 0");

        // listing.price = newPrice;
        emit ListingPriceChanged(listingHash, msg.sender, newPrice);
        return listingHash;
    }

    // 取消上架
    function cancelListing(bytes32 listingHash) external isListing(listingHash) {
        require(!sold[listingHash], "already sold");
        listings[listingHash] = false;
        emit ListingCancelled(listingHash, msg.sender);
    }

    // 用户出价（签名订单）
    function placeBid(Bid calldata bid, bytes calldata signature) external  returns(bytes32 bidHash){
        require(bid.deadline >= block.timestamp, "expired");
        require(listings[bid.listingHash], "not listing");
        require(!sold[bid.listingHash], "already sold");

        require(bid.nonce == nonces[bid.buyer], "bad nonce");
        nonces[bid.buyer] = bid.nonce + 1;
        bidHash = _verifyBid(bid, signature);
        require(!bids[bidHash], "bis is exsit");
        bids[bidHash] = true;

        emit BidPlaced(bid.listingHash, bid.buyer, bid.price, bid.deadline, bid.nonce);
    }

    // 撤回出价
    function _cancelBid(bytes32 bidHash) external isBid(bidHash){
        bids[bidHash] = false;
        emit BidCancelled(bidHash, msg.sender);
    }

    // 卖家接受出价（链下订单：Listing + Bid 都由双方签名）
    function acceptBid(
        Listing calldata listing,
        bytes calldata listingSignature,
        Bid calldata bid,
        bytes calldata bidSignature
    ) external {

        require(listing.deadline >= block.timestamp, "listing expired");
        require(bid.deadline >= block.timestamp, "bid expired");

        bytes32 listingHash = _verifyListing(listing, listingSignature);
        bytes32 bidHash = _verifyBid(bid, bidSignature);

        require(listings[bid.listingHash], "not listing");
        require(bids[bidHash], "bot bid");
        require(!sold[bid.listingHash], "already sold");

        // Bid 中记录的 listingHash 必须匹配
        require(bid.listingHash == listingHash, "listingHash mismatch");

        // 3. 结算：买家付款，市场抽成，NFT 转给买家
        uint256 fee = bid.price * feeRate / feePrecision;

        // 买家需事先对本合约 approve BloomToken
        require(token.transferFrom(bid.buyer, address(this), fee), "pay fee failed");
        require(
            token.transferFrom(bid.buyer, listing.seller, bid.price - fee),
            "pay price failed"
        );

        // NFT 从市场托管地址转给买家
        IERC721(listing.nft).safeTransferFrom(address(this), bid.buyer, listing.tokenId);

        // 4. 状态更新，防止重复成交
        sold[listingHash] = true;
        listings[listingHash] = false;
        bids[bidHash] = false;

        emit BidAccepted(listingHash, bidHash, bid.buyer);
    }

    // 设置手续费率
    function setFeeRate(uint256 _feeRate) external onlyOwner{
        require(_feeRate <= feePrecision, "feeRate is greater than feePrecision");
        feeRate = _feeRate;
    }

    // 取款
    function withdraw() external onlyOwner {
        uint256 balance = address(this).balance;
        (bool success, ) = payable(owner()).call{value: balance}("");
        require(success, "withdraw failed");
        emit Withdraw(msg.sender, balance);
    }
    
    // 让本合约能接收 safeTransferFrom 的 ERC721
    function onERC721Received(address, address, uint256, bytes calldata) external pure returns (bytes4) {
        return this.onERC721Received.selector;
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
                listing.nonce
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