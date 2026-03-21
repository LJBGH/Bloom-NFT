import { BrowserProvider, Contract, JsonRpcSigner } from "ethers";
import addresses from "../config/contract-addresses.json";
import BloomNFTArtifact from "../config/abi/BloomNFT.json";
import BloomTokenArtifact from "../config/abi/BloomToken.json";
import BloomMarketplaceArtifact from "../config/abi/BloomMarketplace.json";
import BloomTokenAirdropArtifact from "../config/abi/BloomTokenAirdrop.json";

type ContractName =
  | "BloomNFT"
  | "BloomToken"
  | "BloomMarketplace"
  | "BloomTokenAirdrop";

type AddressesJson = {
  [network: string]: {
    [key in ContractName]?: string;
  };
};

const typedAddresses = addresses as AddressesJson;

function getNetworkNameFromChainId(chainId: number | null): string {
  // For now, we only care about local hardhat network.
  // You can extend this mapping later for testnets/mainnet.
  if (chainId === 31337) return "local"; // hardhat 默认链 ID
  if (chainId === 1337) return "local";
  if (chainId === 8545) return "local";
  throw new Error(
    `Unsupported chainId=${chainId}. Please add addresses in contract-addresses.json or switch to a local network.`
  );
}

function getAddress(name: ContractName, chainId: number | null): string {
  const networkName = getNetworkNameFromChainId(chainId);
  const addr = typedAddresses[networkName]?.[name];
  if (!addr) {
    throw new Error(`Address for ${name} on network "${networkName}" not found`);
  }
  return addr;
}

export function getBloomNFTContract(
  providerOrSigner: BrowserProvider | JsonRpcSigner,
  chainId: number | null
) {
  const address = getAddress("BloomNFT", chainId);
  return new Contract(address, BloomNFTArtifact.abi, providerOrSigner);
}

export function getBloomNFTAddress(chainId: number | null): string {
  return getAddress("BloomNFT", chainId);
}

export function getBloomTokenContract(
  providerOrSigner: BrowserProvider | JsonRpcSigner,
  chainId: number | null
) {
  const address = getAddress("BloomToken", chainId);
  return new Contract(address, BloomTokenArtifact.abi, providerOrSigner);
}

export function getBloomTokenAddress(chainId: number | null): string {
  return getAddress("BloomToken", chainId);
}

export function getBloomMarketplaceContract(
  providerOrSigner: BrowserProvider | JsonRpcSigner,
  chainId: number | null
) {
  const address = getAddress("BloomMarketplace", chainId);
  return new Contract(address, BloomMarketplaceArtifact.abi, providerOrSigner);
}

export function getBloomMarketplaceAddress(chainId: number | null): string {
  return getAddress("BloomMarketplace", chainId);
}

export function getBloomTokenAirdropContract(
  providerOrSigner: BrowserProvider | JsonRpcSigner,
  chainId: number | null
) {
  const address = getAddress("BloomTokenAirdrop", chainId);
  return new Contract(address, BloomTokenAirdropArtifact.abi, providerOrSigner);
}

export function getBloomTokenAirdropAddress(chainId: number | null): string {
  return getAddress("BloomTokenAirdrop", chainId);
}

