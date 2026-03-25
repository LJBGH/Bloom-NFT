import hardhatToolboxMochaEthersPlugin from "@nomicfoundation/hardhat-toolbox-mocha-ethers";
import { configVariable, defineConfig } from "hardhat/config";
import dotenv from "dotenv";

dotenv.config();

export default defineConfig({
  plugins: [hardhatToolboxMochaEthersPlugin],
  solidity: {
    profiles: {
      // 默认即开启优化器：BloomMarketplace 未优化时超过 24KB，部署时 eth_estimateGas 会失败（节点报 Internal error）
      default: {
        version: "0.8.28",
        settings: {
          optimizer: {
            enabled: true,
            // runs 较低有利于减小部署体积；若更在意运行时 gas 可提高到 200～1000
            runs: 200,
          },
          viaIR: true,
        },
      },
      production: {
        version: "0.8.28",
        settings: {
          optimizer: {
            enabled: true,
            runs: 200,
          },
          viaIR: true,
        },
      },
    },
  },
  networks: {
    local: {
      type: "http",
      chainId: 31337,
      url: "http://127.0.0.1:8545",
    },
    sepolia: {
      type: "http",
      chainId: 11155111,
      url: `https://eth-sepolia.g.alchemy.com/v2/${process.env.SEPOLIA_ALCHEMY_API_KEY}`,
      accounts: [ process.env.SEPOLIA_PRIVATE_KEY as string],
    },
  },
});
