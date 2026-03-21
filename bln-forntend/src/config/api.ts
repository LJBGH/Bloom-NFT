export const API_BASE_URL = "http://localhost:8081";

export const API_ENDPOINTS = {
  nftMint: `${API_BASE_URL}/api/nft/mint`,
  nftUserCategories: `${API_BASE_URL}/api/nft/user/categories`,
  nftUserListByCategory: (nftId: number) =>
    `${API_BASE_URL}/api/nft/user/list/${nftId}`,
  nftUpdate: `${API_BASE_URL}/api/nft/update`,

  // Orders
  entryOrders: `${API_BASE_URL}/api/order/entryorders`,
  bidPlaced: `${API_BASE_URL}/api/order/bidplaced`,
  bidAccepted: `${API_BASE_URL}/api/order/bidaccepted`,
  bidList: (ordersId: number) => `${API_BASE_URL}/api/order/bidlist/${ordersId}`,
  orderList: (nftId?: number) =>
    nftId ? `${API_BASE_URL}/api/order/orderlist?nftId=${nftId}` : `${API_BASE_URL}/api/order/orderlist`,
} as const;

