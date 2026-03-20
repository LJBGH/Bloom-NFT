import { network } from "hardhat";
import { saveContractAddress } from "../utils.js";

async function main() {
  const { ethers } = await network.connect();
  const [deployer] = await ethers.getSigners();
  console.log("Deploying BloomNFT with:", deployer.address);

  const BloomNFT = await ethers.getContractFactory("BloomNFT");
  const bloomNFT = await BloomNFT.deploy();
  await bloomNFT.waitForDeployment();

  const address = await bloomNFT.getAddress();
  console.log("BloomNFT deployed to:", address);

  const networkName =
    (network as any).name || process.env.HARDHAT_NETWORK || "local";

  await saveContractAddress(networkName, "BloomNFT", address);
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});

