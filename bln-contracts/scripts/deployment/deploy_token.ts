import { network } from "hardhat";
import { saveContractAddress } from "../utils.js";

async function main() {
  const { ethers } = await network.connect();
  const [deployer] = await ethers.getSigners();
  console.log("Deploying BloomToken with:", deployer.address);

  const BloomToken = await ethers.getContractFactory("BloomToken");
  const bloomToken = await BloomToken.deploy();
  await bloomToken.waitForDeployment();

  const address = await bloomToken.getAddress();
  console.log("BloomToken deployed to:", address);

  const networkName =
    (network as any).name || process.env.HARDHAT_NETWORK || "local";

  await saveContractAddress(networkName, "BloomToken", address);
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});

