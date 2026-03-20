import { promises as fs } from "fs";
import path from "path";

const CONFIG_RELATIVE_PATH = path.join("scripts", "config", "contract-addresses.json");

export type ContractName = "BloomNFT" | "BloomToken" | "BloomMarketplace";

export async function saveContractAddress(
  network: string,
  name: ContractName,
  address: string
) {
  if (!/^0x[a-fA-F0-9]{40}$/.test(address)) {
    throw new Error(`Invalid address for ${name}: ${address}`);
  }

  const filePath = path.join(process.cwd(), CONFIG_RELATIVE_PATH);

  let current: Record<
    string,
    {
      [key in ContractName]?: string;
    }
  > = {};
  try {
    const content = await fs.readFile(filePath, "utf8");
    current = JSON.parse(content);
  } catch (e) {
    // 如果文件不存在或 JSON 无效，使用空对象重建
    current = {};
  }

  const networkEntry = current[network] ?? {};
  const updatedNetworkEntry: { [key in ContractName]?: string } = {
    ...networkEntry,
    [name]: address,
  };

  const updated = {
    ...current,
    [network]: updatedNetworkEntry,
  };

  await fs.writeFile(filePath, JSON.stringify(updated, null, 4), "utf8");
  console.log(
    `Saved ${name} address for network "${network}" to ${CONFIG_RELATIVE_PATH}:`,
    address
  );
}

export async function getContractAddress(
  name: ContractName,
  network?: string
): Promise<string> {
  const filePath = path.join(process.cwd(), CONFIG_RELATIVE_PATH);
  const content = await fs.readFile(filePath, "utf8");
  const json = JSON.parse(content) as Record<
    string,
    {
      [key in ContractName]?: string;
    }
  >;

  const resolvedNetwork =
    network ||
    process.env.HARDHAT_NETWORK ||
    process.env.NEXT_PUBLIC_NETWORK ||
    "local";

  const networkEntry = json[resolvedNetwork];
  const address = networkEntry?.[name];

  if (!address) {
    throw new Error(`Address for ${name} not found in ${CONFIG_RELATIVE_PATH}`);
  }
  if (!/^0x[a-fA-F0-9]{40}$/.test(address)) {
    throw new Error(
      `Invalid address for ${name} in ${CONFIG_RELATIVE_PATH}: ${address}`
    );
  }

  return address;
}


