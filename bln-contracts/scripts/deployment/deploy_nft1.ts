import { network } from "hardhat";
import { saveContractAddress } from "../utils.js";

async function main() {
  const { ethers } = await network.connect();
  const [deployer] = await ethers.getSigners();
  console.log("Deploying BloomNFT1 with:", deployer.address);

  const BloomNFT1 = await ethers.getContractFactory("BloomNFT");
  const bloomNFT1 = await BloomNFT1.deploy("BloomNFT1", "BNFT1");
  await bloomNFT1.waitForDeployment();

  const address = await bloomNFT1.getAddress();
  console.log("BloomNFT1 deployed to:", address);

  const networkName =
    (network as any).name || process.env.HARDHAT_NETWORK || "local";

  await saveContractAddress(networkName, "BloomNFT1", address);
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});

