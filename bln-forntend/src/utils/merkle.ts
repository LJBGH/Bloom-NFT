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
        // 与 OpenZeppelin MerkleTree 兼容：当节点数量为奇数，最后一个节点与自己配对 hash
        next.push(commutativeKeccak256(level[i], level[i]));
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
    // 与 buildMerkleTreeFromLeaves 相同规则：奇数层末尾缺失 sibling 时，用自身作为 sibling
    if (siblingIdx < row.length) proof.push(row[siblingIdx]);
    else proof.push(row[idx]);
    idx = Math.floor(idx / 2);
  }
  return proof;
}

/** 与 OpenZeppelin MerkleProof.verify 等价的前端校验（使用 commutativeKeccak256） */
export function verifyMerkleProof(
  proof: string[],
  root: string,
  leaf: string
): boolean {
  let computed = leaf.startsWith("0x") ? leaf : `0x${leaf}`;
  const r = root.toLowerCase();
  for (const p of proof) {
    const pp = p.startsWith("0x") ? p : `0x${p}`;
    computed = commutativeKeccak256(computed, pp);
  }
  return computed.toLowerCase() === r;
}

export function randomSalt(): bigint {
  return BigInt(hexlify(randomBytes(32)));
}
