# Bloom-NFT

Bloom-NFT 是一个基于以太坊区块链的 NFT 市场项目，包含合约、后端与前端三部分，主要有NFT铸造、二级市场交易、订单撮合、链上日志监听等核心功能。

项目资料与架构设计可参考：[Bloom-NFT-Architecture-and-Market-Design.md](./Bloom-NFT-Architecture-and-Market-Design.md)

## 架构说明

- `bln-contracts`：合约工程（hardhat）
- `bln-backend`：后端工程（golang）
- `bln-forntend`：前端工程（react）

## 环境要求

- MySQL（用于落库）
- Go（用于启动后端）
- Hardhat（用于启动本地JSON-RPC节点与合约部署）
- Node.js（用于启动前端）
- MetaMask（钱包）
- Pinata（去中心化存储系统 提供IPFS服务）

## 启动流程

### 1、合约

- 进入`bln-contracts`目录
- 启动本地节点
  `npx hardhat node`
- 部署合约
  `make all`

### 2、后端

- 前置条件：先复制部署后的合约地址与ABI配置文件到目录 `/bin-backend/abi`
- 启动后端工程
  
  ```bash
  cd bln-backend
  go run main.go
  ```

- 本地文档地址
  `http://localhost:8081/swagger/index.html`

### 3、前端

- 前置条件：先复制部署后的合约地址与ABI配置文件到目录 `/bln-forntend/src/config`
- 启动前端工程

  ```bash
  cd bln-forntend
  npm install
  npm run dev
  ```

- 本地浏览器查看
  `http://localhost:5173/`

## 安全提示

请不要把 `config.yml/config.production.yml` 里的私钥、JWT Secret 等敏感信息公开提交。
