import { network } from "hardhat";
import { saveContractAddress, getContractAddress } from "../utils.js";

async function main() {
  const { ethers } = await network.connect();
  const [deployer] = await ethers.getSigners();
  console.log("Deploying BloomTokenAirdrop with:", deployer.address);
  const networkName =
  (network as any).name || process.env.HARDHAT_NETWORK || "local";

  const bloomTokenAddress = await getContractAddress(
    "BloomToken",
    networkName
  );

  const BloomToken = await ethers.getContractFactory("BloomTokenAirdrop");
  const bloomTokenAirdrop = await BloomToken.deploy(bloomTokenAddress);
  await bloomTokenAirdrop.waitForDeployment();

  const address = await bloomTokenAirdrop.getAddress();
  console.log("BloomTokenAirdrop deployed to:", address);

  await saveContractAddress(networkName, "BloomTokenAirdrop", address);

  // 预授权：给空投合约授权 1000 * 1e18 的 BloomToken，避免 withdrawTokens() 因余额不足而 revert
  const bloomToken = await ethers.getContractAt(
    "BloomToken",
    bloomTokenAddress
  );
  const approveAmount = ethers.parseUnits("1000", 18);
  console.log(
    `Approving BloomTokenAirdrop(${address}) for ${approveAmount.toString()}...`
  );
  const approveTx = await bloomToken.approve(address, approveAmount);
  await approveTx.wait();
  console.log("Approve tx confirmed.");

  // 注意：BloomTokenAirdrop.withdrawTokens() 内部使用的是 ERC20.transfer，
  // 需要空投合约自身拥有 BT 余额，因此这里把 1000 BT 转入合约。
  const transferTx = await bloomToken.transfer(address, approveAmount);
  await transferTx.wait();
  console.log("Transfer to Airdrop contract confirmed.");
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});

