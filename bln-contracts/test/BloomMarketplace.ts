import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.connect();

describe("BloomMarketplace", () => {
  let token: any;
  let nft: any;
  let marketplace: any;
  let seller: any;
  let buyer: any;

  beforeEach(async () => {
    const [deployer, s, b] = await ethers.getSigners();
    seller = s;
    buyer = b;

    // 部署 BloomToken
    const Token = await ethers.getContractFactory("BloomToken");
    token = await Token.deploy();
    await token.waitForDeployment();

    // 部署 BloomNFT
    const NFT = await ethers.getContractFactory("BloomNFT");
    nft = await NFT.deploy();
    await nft.waitForDeployment();

    // 部署 Marketplace
    const Marketplace = await ethers.getContractFactory("BloomMarketplace");
    marketplace = await Marketplace.deploy(await token.getAddress());
    await marketplace.waitForDeployment();

    // 给卖家一些 Token 和 NFT
    await token.mint(await buyer.getAddress(), ethers.parseEther("1000"));
    await nft.connect(seller).setTrader(await seller.getAddress());
    await nft
      .connect(seller)
      .mintBatch(await seller.getAddress(), ["ipfs://1"]);
  });

  async function signListing(
    sellerSigner: any,
    data: {
      nft: string;
      seller: string;
      tokenId: bigint;
      price: bigint;
      deadline: bigint;
      nonce: bigint;
      salt: bigint;
    }
  ) {
    const domain = {
      name: "BloomMarketplace",
      version: "1",
      chainId: (await sellerSigner.provider!.getNetwork()).chainId,
      verifyingContract: await marketplace.getAddress(),
    };

    const types = {
      Listing: [
        { name: "nft", type: "address" },
        { name: "seller", type: "address" },
        { name: "tokenId", type: "uint256" },
        { name: "price", type: "uint256" },
        { name: "deadline", type: "uint256" },
        { name: "nonce", type: "uint256" },
        { name: "salt", type: "uint256" },
      ],
    };

    const signature = await sellerSigner.signTypedData(domain, types, data);
    return signature;
  }

  async function signBid(
    buyerSigner: any,
    data: {
      listingHash: string;
      buyer: string;
      price: bigint;
      deadline: bigint;
      nonce: bigint;
    }
  ) {
    const domain = {
      name: "BloomMarketplace",
      version: "1",
      chainId: (await buyerSigner.provider!.getNetwork()).chainId,
      verifyingContract: await marketplace.getAddress(),
    };

    const types = {
      Bid: [
        { name: "listingHash", type: "bytes32" },
        { name: "buyer", type: "address" },
        { name: "price", type: "uint256" },
        { name: "deadline", type: "uint256" },
        { name: "nonce", type: "uint256" },
      ],
    };

    const signature = await buyerSigner.signTypedData(domain, types, data);
    return signature;
  }

  it("should list, bid and acceptBid with off-chain signatures", async () => {
    const tokenId = 0n;
    const price = ethers.parseEther("10");
    const listingDeadline = BigInt(
      (await ethers.provider.getBlock("latest"))!.timestamp + 3600
    );
    const bidDeadline = listingDeadline;

    const listing = {
      nft: await nft.getAddress(),
      seller: await seller.getAddress(),
      tokenId,
      price,
      deadline: listingDeadline,
      nonce: 0n,
      salt: 0n,
    };

    // 卖家允许市场合约转移 NFT
    await nft
      .connect(seller)
      .setApprovalForAll(await marketplace.getAddress(), true);

    const listingSig = await signListing(seller, listing);

    // 调用 listWithSig 上架
    await expect(
      marketplace.connect(buyer).listWithSig(listing, listingSig)
    ).to.emit(marketplace, "Listed");

    // 构造 listingHash 与 bid
    const listingHash = await marketplace
      .connect(buyer)
      .callStatic.listWithSig(listing, listingSig)
      .catch(() => {
        return ethers.ZeroHash;
      });

    const bid = {
      listingHash,
      buyer: await buyer.getAddress(),
      price,
      deadline: bidDeadline,
      nonce: 0n,
    };

    const bidSig = await signBid(buyer, bid);

    // 买家 approve 代币给市场
    await token
      .connect(buyer)
      .approve(await marketplace.getAddress(), ethers.MaxUint256);

    // 卖家接受出价
    await expect(
      marketplace.connect(seller).acceptBid(listing, listingSig, bid, bidSig)
    ).to.emit(marketplace, "BidAccepted");
  });
});

