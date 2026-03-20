export const API_BASE_URL = "http://localhost:8081";

export const API_ENDPOINTS = {
  nftMint: `${API_BASE_URL}/api/nft/mint`,
  nftUserCategories: `${API_BASE_URL}/api/nft/user/categories`,
  nftUserListByCategory: (nftId: number) =>
    `${API_BASE_URL}/api/nft/user/list/${nftId}`,
  nftUpdate: `${API_BASE_URL}/api/nft/update`,
} as const;

