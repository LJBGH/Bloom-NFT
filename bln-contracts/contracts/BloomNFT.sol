// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "@openzeppelin/contracts/token/ERC721/ERC721.sol";
import "@openzeppelin/contracts/token/ERC721/extensions/ERC721URIStorage.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract BloomNFT is Ownable, ERC721URIStorage {

    uint256 private nextTokenId; // 下一个tokenId
    uint256 public price; // 价格
    uint256 public maxBalance; // 每个地址最大持有量
    uint256 public totalSupply; // 总供应量
    uint256 public maxSupply; // 最大供应量

    mapping(address => bool) public isTrader; // 是否是交易者
    mapping(address => uint256) public minted; // 用户mint的数量

    event Mint(address indexed sender, uint256 indexed tokenId, string url);
    event Withdraw(address indexed sender, uint256 amount);

    constructor() Ownable(msg.sender) ERC721("BloomNFT", "BNFT") {
        price = 10000; // 10000wei
        maxBalance = 3;
        maxSupply = 8000;
        nextTokenId = 1;
    }

    // 仅交易者可以调用
    modifier onlyTrader() {
        require(isTrader[msg.sender], "msg.sender is not a trader");
        _;
    }

    // 普通用户
    modifier onlyCommonUser(){
        require(!isTrader[msg.sender], "msg.sender is a trader");
        _;
    }

    // 铸造NFT
    function mint(address to, string memory url) public payable {
        require(to == msg.sender);
        require(to != address(0), "to is the zero address");
        require(totalSupply < maxSupply, "totalSupply is greater than maxSupply");
        require(balanceOf(to) < maxBalance, "balance of to is greater than maxBalance");
        require(bytes(url).length > 0, "url is the zero address");
        require(msg.value == price, "msg.value is not equal to price");
        require(minted[to] + 1 <= maxBalance, "exceed max mint");
        
        minted[to] += 1;
        uint256 tokenId = nextTokenId;
        nextTokenId = tokenId + 1;
        totalSupply += 1;
  
        _safeMint(to, tokenId);
        _setTokenURI(tokenId, url);
        emit Mint(to, tokenId, url);
    }

    // 批量铸造NFT
    function mintBatch(address to, string[] memory urls) public payable {
        require(to == msg.sender);
        require(to != address(0), "to is the zero address");
        require(urls.length > 0, "urls is empty");
        require(totalSupply + urls.length <= maxSupply, "exceed maxSupply");
        require(balanceOf(to) + urls.length <= maxBalance, "exceed maxBalance");
        require(msg.value == price * urls.length, "msg.value is not equal to price");
        uint256[] memory tokenIds = new uint256[](urls.length);
         
        // 提前检查总 mint 数量
        require(minted[to] + urls.length <= maxBalance, "exceed max mint per user");
        for (uint256 i = 0; i < urls.length; i++) {
            require(bytes(urls[i]).length > 0, "empty url");
            require(minted[to] + urls.length <= maxBalance, "exceed max mint");
            
            uint256 tokenId = nextTokenId;
            nextTokenId = tokenId + 1;
            totalSupply += 1;
            tokenIds[i] = tokenId;

            _safeMint(to, tokenId);
            _setTokenURI(tokenId, urls[i]);
            emit Mint(to, tokenId, urls[i]);
        }

        minted[to] += urls.length;
    }

    // 设置价格
    function setPrice(uint256 _price) external onlyOwner {
        require(_price > 0, "price is 0");
        price = _price;
    }

    // 设置交易者
    function setTrader(address trader) external onlyOwner {
        require(trader != address(0), "trader is the zero address");
        isTrader[trader] = true;
    }

    // 取消交易者
    function unsetTrader(address trader) external onlyOwner {
        require(trader != address(0), "trader is the zero address");
        isTrader[trader] = false;
    }

    // 取款
    function withdraw() external onlyOwner {
        uint256 balance = address(this).balance;
        (bool success, ) = payable(owner()).call{value: balance}("");
        require(success, "withdraw failed");
        emit Withdraw(msg.sender, balance);
    }
}