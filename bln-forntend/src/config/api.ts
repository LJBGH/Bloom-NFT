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
  /** 卖家按 nft_list_id + seller 查询该挂单下的出价列表 */
  bidListForSeller: (nftListId: number, seller: string) =>
    `${API_BASE_URL}/api/order/bidlist-for-seller?nftListId=${nftListId}&seller=${encodeURIComponent(seller)}`,
  orderList: (nftId?: number) =>
    nftId ? `${API_BASE_URL}/api/order/orderlist?nftId=${nftId}` : `${API_BASE_URL}/api/order/orderlist`,
  /** 当前地址作为卖家的挂单历史 */
  myEntryOrders: (seller: string) =>
    `${API_BASE_URL}/api/order/my-entryorders?seller=${encodeURIComponent(seller)}`,
  /** 当前地址作为买家的出价历史 */
  myBidHistory: (buyer: string) =>
    `${API_BASE_URL}/api/order/my-bids?buyer=${encodeURIComponent(buyer)}`,
} as const;

