import { AbiCoder, concat, getBytes, hexlify, keccak256, randomBytes, toUtf8Bytes } from "ethers";

/** 与 BloomMarketplace.sol LISTING_LEAF_TYPEHASH 字符串一致 */
export const LISTING_LEAF_TYPEHASH = keccak256(
  toUtf8Bytes(
    "ListingLeaf(address nft,address seller,uint256 tokenId,uint256 price,uint256 deadline,uint256 salt)"
  )
);

/** 与链上 _listingLeafHash 一致 */
export function listingLeafHash(
  nft: string,
  seller: string,
  tokenId: bigint,
  priceWei: bigint,
  deadlineSec: bigint,
  salt: bigint
): string {
  const enc = AbiCoder.defaultAbiCoder().encode(
    ["bytes32", "address", "address", "uint256", "uint256", "uint256", "uint256"],
    [LISTING_LEAF_TYPEHASH, nft, seller, tokenId, priceWei, deadlineSec, salt]
  );
  return keccak256(enc);
}

/** OpenZeppelin Hashes.commutativeKeccak256 */
export function commutativeKeccak256(a: string, b: string): string {
  const aa = BigInt(a);
  const bb = BigInt(b);
  const [x, y] = aa < bb ? [a, b] : [b, a];
  return keccak256(concat([getBytes(x), getBytes(y)]));
}

export type MerkleTreeResult = {
  root: string;
  layers: string[][];
};

/** 自底向上构建 Merkle 树（与 OpenZeppelin MerkleProof.verify 兼容） */
export function buildMerkleTreeFromLeaves(leaves: string[]): MerkleTreeResult {
  let level = leaves.map((l) => (l.startsWith("0x") ? l : `0x${l}`) as `0x${string}`);
  const layers: string[][] = [level.slice()];
  while (level.length > 1) {
    const next: string[] = [];
    for (let i = 0; i < level.length; i += 2) {
      if (i + 1 < level.length) {
        next.push(commutativeKeccak256(level[i], level[i + 1]));
      } else {
        next.push(level[i]);
      }
    }
    level = next as `0x${string}`[];
    layers.push(level.slice());
  }
  return { root: level[0], layers };
}

/** 某叶子在 layers[0] 中的下标对应的 Merkle proof */
export function getMerkleProof(layers: string[][], leafIndex: number): string[] {
  const proof: string[] = [];
  let idx = leafIndex;
  for (let layer = 0; layer < layers.length - 1; layer++) {
    const row = layers[layer];
    const siblingIdx = idx % 2 === 1 ? idx - 1 : idx + 1;
    if (siblingIdx < row.length) {
      proof.push(row[siblingIdx]);
    }
    idx = Math.floor(idx / 2);
  }
  return proof;
}

export function randomSalt(): bigint {
  return BigInt(hexlify(randomBytes(32)));
}
