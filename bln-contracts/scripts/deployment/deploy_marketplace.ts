import { network } from "hardhat";
import { saveContractAddress, getContractAddress } from "../utils.js";

async function main() {
  const { ethers } = await network.connect();
  const networkName =
    (network as any).name || process.env.HARDHAT_NETWORK || "local";
  const token = await getContractAddress("BloomToken", networkName);

  const [deployer] = await ethers.getSigners();
  console.log("Deploying BloomMarketplace with:", deployer.address);
  console.log("Using BloomToken:", token);

  const BloomMarketplace = await ethers.getContractFactory("BloomMarketplace");
  const marketplace = await BloomMarketplace.deploy(token);
  await marketplace.waitForDeployment();

  const address = await marketplace.getAddress();
  console.log("BloomMarketplace deployed to:", address);

  await saveContractAddress(networkName, "BloomMarketplace", address);
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});

