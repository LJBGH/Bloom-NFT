import {
  Alert,
  Box,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Button,
  Stack,
  TextField,
  Typography,
  Divider,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Menu,
  MenuItem,
  Tabs,
  Tab,
  Link,
  Checkbox,
} from "@mui/material";
import KeyboardArrowDownIcon from "@mui/icons-material/KeyboardArrowDown";
import { hexlify, parseUnits, randomBytes, TypedDataEncoder } from "ethers";
import {
  buildMerkleTreeFromLeaves,
  getMerkleProof,
  listingLeafHash,
  randomSalt,
} from "../utils/merkle";
import { useWeb3 } from "../web3/provider";
import { useConfirmDialog } from "../components/ConfirmDialog";
import { useCallback, useEffect, useMemo, useState } from "react";
import { API_ENDPOINTS } from "../config/api";
import {
  getBloomNFTContractAt,
  getBloomNFTAddress,
  getBloomMarketplaceContract,
  getBloomMarketplaceAddress,
} from "../web3/contracts";

interface NftCategory {
  id: number;
  name: string;
  description: string;
}

interface NftItem {
  id: number;
  name: string;
  description: string;
  imageUrl: string;
  metadataUrl: string;
  tokenUrl: string;
  tokenId: number;
  /** NFT 合约地址（来自后端 nft.address；用于 merkle leaf / EIP-712 签名） */
  nftAddress?: string;
  status?: number;
  statusDesc?: string;
}

interface EntryOrderItem {
  id: number;
  nftListId: number;
  nftAddress?: string;
  seller: string;
  buyer?: string;
  tokenId: number;
  price: number;
  deadline: string;
  status: number;
  statusDesc?: string;
  txHash?: string;
  createTime?: string;
  updateTime?: string;
  imageUrl?: string;
  listingHash?: string;
  /** Listing EIP-712 salt（十进制字符串） */
  salt?: string;
  isMerkle?: boolean;

  // Merkle 参数：用于 acceptBid 时走 merkle 校验路径
  merkleRoot?: string;
  rootDeadlineSec?: number;
  merkleProof?: string[];
}

interface BidPlacedItem {
  id: number;
  ordersId: number;
  buyer: string;
  price: number;
  deadline: string;
  salt: string;
  status: number;
  txHash?: string;
  createTime?: string;
  bidHash?: string;
}

const BID_STATUS_LABELS: Record<number, string> = {
  0: "进行中",
  1: "已成交",
  2: "已过期",
  3: "取消出价",
  4: "已下架",
  5: "未中标",
  6: "已退款",
};

/** 挂单 entry_orders.status（ListingStatus 0–3） */
const ORDER_STATUS_LABELS: Record<number, string> = {
  0: "进行中",
  1: "已成交",
  2: "已过期",
  3: "已取消",
};

/** 仅保留「当前钱包 + 进行中(0) / 已过期(2)」的挂单；
 * 同一 nftListId 取 id 最大的一条
 */
function buildActiveListingMap(
  orders: EntryOrderItem[],
  wallet: string
): Record<number, EntryOrderItem> {
  const w = wallet.toLowerCase();
  const map: Record<number, EntryOrderItem> = {};
  for (const o of orders) {
    if (o.seller.toLowerCase() !== w) continue;
    if (o.status !== 0 && o.status !== 2) continue;
    const prev = map[o.nftListId];
    if (!prev || o.id > prev.id) {
      map[o.nftListId] = o;
    }
  }
  return map;
}

interface MyBidHistoryRow {
  id: number;
  ordersId: number;
  buyer: string;
  price: number;
  deadline: string;
  salt: string;
  status: number;
  statusDesc?: string;
  nftListId: number;
  tokenId: number;
  entrySeller: string;
  imageUrl?: string;
  createTime?: string;
  listingHash?: string;
  /** 对应 entry_orders.status（挂单是否已取消等） */
  entryOrderStatus?: number;
  signature?: string;
}

/** 持有的 NFT + 所属类目（前端聚合多类目接口） */
interface OwnedNftRow extends NftItem {
  categoryId: number;
  categoryName: string;
}

function entryOrderToNftItem(o: EntryOrderItem, name: string): NftItem {
  return {
    id: o.nftListId,
    name,
    description: "",
    imageUrl: o.imageUrl || "",
    metadataUrl: "",
    tokenUrl: "",
    tokenId: o.tokenId,
  };
}

export function Profile() {
  const { account, isConnected, chainId, signer } = useWeb3();
  const { requestConfirm, confirmDialog } = useConfirmDialog();
  const [loading, setLoading] = useState(false);
  const [categories, setCategories] = useState<NftCategory[]>([]);
  /** 全部持有的 NFT（所有类目合并） */
  const [allOwnedNfts, setAllOwnedNfts] = useState<OwnedNftRow[]>([]);
  const [profileTab, setProfileTab] = useState(0);
  const [activeListingByNftListId, setActiveListingByNftListId] = useState<
    Record<number, EntryOrderItem>
  >({});

  const [historyOrdersOpen, setHistoryOrdersOpen] = useState(false);
  const [historyBidsOpen, setHistoryBidsOpen] = useState(false);
  const [historyOrders, setHistoryOrders] = useState<EntryOrderItem[]>([]);
  const [historyBids, setHistoryBids] = useState<MyBidHistoryRow[]>([]);
  const [historyOrdersLoading, setHistoryOrdersLoading] = useState(false);
  const [historyBidsLoading, setHistoryBidsLoading] = useState(false);
  const [historyError, setHistoryError] = useState<string | null>(null);
  // 新版：不再托管代币，因此不需要“取回托管/退款”相关状态与操作。

  /** NFT 卡片「操作」下拉菜单（进行中挂单时） */
  const [actionMenu, setActionMenu] = useState<{
    anchor: HTMLElement | null;
    nft: NftItem | null;
  }>({ anchor: null, nft: null });

  const closeActionMenu = () =>
    setActionMenu({ anchor: null, nft: null });
  const [error, setError] = useState<string | null>(null);

  // --- Entry order dialog state ---
  const [orderDialogOpen, setOrderDialogOpen] = useState(false);
  const [selectedNft, setSelectedNft] = useState<NftItem | null>(null);
  const [orderSubmitting, setOrderSubmitting] = useState(false);
  const [cancelSubmittingId, setCancelSubmittingId] = useState<number | null>(null);
  const [orderError, setOrderError] = useState<string | null>(null);
  const [orderSuccess, setOrderSuccess] = useState<string | null>(null);

  // --- 出价列表对话框 ---
  const [bidDialogOpen, setBidDialogOpen] = useState(false);
  const [bidDialogNft, setBidDialogNft] = useState<NftItem | null>(null);
  const [bidList, setBidList] = useState<BidPlacedItem[]>([]);
  const [bidLoading, setBidLoading] = useState(false);
  const [bidError, setBidError] = useState<string | null>(null);
  const [acceptBidId, setAcceptBidId] = useState<number | null>(null);

  const [price, setPrice] = useState("1");
  const [deadlineLocal, setDeadlineLocal] = useState("");

  /** 链上降价（不先取消上架） */
  const [changePriceDialogOpen, setChangePriceDialogOpen] = useState(false);
  const [changePriceNft, setChangePriceNft] = useState<NftItem | null>(null);
  const [changePriceValue, setChangePriceValue] = useState("");
  const [changePriceError, setChangePriceError] = useState<string | null>(null);
  const [changePriceSubmitting, setChangePriceSubmitting] = useState(false);

  /** Merkle 批量上架对话框 */
  const [batchListOpen, setBatchListOpen] = useState(false);
  const [batchListPrice, setBatchListPrice] = useState("1");
  const [batchListDeadline, setBatchListDeadline] = useState("");
  const [batchListPricesById, setBatchListPricesById] = useState<
    Record<number, string>
  >({});
  const [batchListDeadlinesById, setBatchListDeadlinesById] = useState<
    Record<number, string>
  >({});
  const [batchListPicks, setBatchListPicks] = useState<Set<number>>(() => new Set());
  const [batchListSubmitting, setBatchListSubmitting] = useState(false);
  /** 批量下架：勾选订单 id */
  const [cancelBatchPicks, setCancelBatchPicks] = useState<Set<number>>(() => new Set());
  const [cancelBatchSubmitting, setCancelBatchSubmitting] = useState(false);

  /** 拥有的 NFT — 查看详情弹窗 */
  const [ownedDetailOpen, setOwnedDetailOpen] = useState(false);
  const [ownedDetailNft, setOwnedDetailNft] = useState<OwnedNftRow | null>(
    null
  );

  const pad2 = (n: number) => String(n).padStart(2, "0");
  const toDateTimeLocal = (d: Date) => {
    // datetime-local expects local time: YYYY-MM-DDTHH:mm
    // We only keep hour precision, so we force minutes to ":00".
    return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}T${pad2(
      d.getHours()
    )}:00`;
  };
  const normalizeHourOnlyDateTimeLocal = (value: string) => {
    // Expected: YYYY-MM-DDTHH:mm (we normalize it to YYYY-MM-DDTHH:00)
    const m = value.match(/^(\d{4}-\d{2}-\d{2})T(\d{2})/);
    if (!m) return value;
    return `${m[1]}T${m[2]}:00`;
  };
  const defaultDeadline = () => {
    const d = new Date();
    d.setDate(d.getDate() + 1); // 默认 +1 天
    return toDateTimeLocal(d);
  };

  const fetchHistoryOrders = async () => {
    if (!account) return;
    setHistoryOrdersLoading(true);
    setHistoryError(null);
    try {
      const resp = await fetch(API_ENDPOINTS.myEntryOrders(account));
      const data = await resp.json();
      if (!resp.ok || data?.code !== 0) {
        throw new Error(data?.message || "获取历史挂单失败");
      }
      setHistoryOrders((data.data as EntryOrderItem[]) || []);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setHistoryError(msg || "获取历史挂单失败");
      setHistoryOrders([]);
    } finally {
      setHistoryOrdersLoading(false);
    }
  };

  const fetchHistoryBids = async () => {
    if (!account) return;
    setHistoryBidsLoading(true);
    setHistoryError(null);
    try {
      const resp = await fetch(API_ENDPOINTS.myBidHistory(account));
      const data = await resp.json();
      if (!resp.ok || data?.code !== 0) {
        throw new Error(data?.message || "获取历史出价失败");
      }
      setHistoryBids((data.data as MyBidHistoryRow[]) || []);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setHistoryError(msg || "获取历史出价失败");
      setHistoryBids([]);
    } finally {
      setHistoryBidsLoading(false);
    }
  };

  // 新版：历史出价不再提供取回/退款操作。

  /** 拉取全量挂单 + 所有类目下的持有 NFT（沿用现有接口，无需改后端） */
  const refreshPortfolio = useCallback(async () => {
    if (!account || categories.length === 0) return;
    setLoading(true);
    setError(null);
    try {
      const orderResp = await fetch(API_ENDPOINTS.orderList());
      const orderData = await orderResp.json();
      if (!orderResp.ok || orderData.code !== 0) {
        throw new Error(orderData.message || "获取挂单列表失败");
      }
      const orders: EntryOrderItem[] = orderData.data || [];
      setActiveListingByNftListId(buildActiveListingMap(orders, account));

      const merged: OwnedNftRow[] = [];
      for (const cat of categories) {
        const nftResp = await fetch(
          `${API_ENDPOINTS.nftUserListByCategory(cat.id)}?owner=${account}`
        );
        const nftData = await nftResp.json();
        if (!nftResp.ok || nftData.code !== 0) {
          throw new Error(nftData.message || "获取 NFT 列表失败");
        }
        const list = (nftData.data as NftItem[]) || [];
        for (const item of list) {
          merged.push({
            ...item,
            categoryId: cat.id,
            categoryName: cat.name,
          });
        }
      }
      setAllOwnedNfts(merged);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setError(msg || "刷新数据失败");
    } finally {
      setLoading(false);
    }
  }, [account, categories]);

  const listedOrders = useMemo(
    () =>
      Object.values(activeListingByNftListId).sort((a, b) => b.id - a.id),
    [activeListingByNftListId]
  );

  const nftNameByListId = useMemo(() => {
    const m = new Map<number, string>();
    for (const n of allOwnedNfts) {
      m.set(n.id, n.name);
    }
    return m;
  }, [allOwnedNfts]);

  /** 未挂单、可用于 Merkle 批量上架的持有项 */
  const unlistedForBatch = useMemo(
    () => allOwnedNfts.filter((n) => !activeListingByNftListId[n.id]),
    [allOwnedNfts, activeListingByNftListId]
  );

  const openEntryOrderDialog = (nft: NftItem) => {
    setSelectedNft(nft);
    setPrice("1");
    setDeadlineLocal(defaultDeadline());
    setOrderError(null);
    setOrderSuccess(null);
    setOrderDialogOpen(true);
  };

  const openBatchListDialog = () => {
    setBatchListDeadline(defaultDeadline());
    setBatchListPrice("1");
    setBatchListPicks(new Set());
    setBatchListPricesById({});
    setBatchListDeadlinesById({});
    setBatchListOpen(true);
    setOrderError(null);
  };

  const toggleBatchListPick = (nftListId: number) => {
    const wasPicked = batchListPicks.has(nftListId);
    setBatchListPicks((prev) => {
      const n = new Set(prev);
      if (n.has(nftListId)) n.delete(nftListId);
      else n.add(nftListId);
      return n;
    });
    if (wasPicked) {
      setBatchListPricesById((prev) => {
        const next = { ...prev };
        delete next[nftListId];
        return next;
      });
      setBatchListDeadlinesById((prev) => {
        const next = { ...prev };
        delete next[nftListId];
        return next;
      });
    } else {
      setBatchListPricesById((prev) => ({ ...prev, [nftListId]: batchListPrice }));
      setBatchListDeadlinesById((prev) => ({
        ...prev,
        [nftListId]: batchListDeadline,
      }));
    }
  };

  const handleSubmitBatchMerkleList = async () => {
    if (!account || !signer || chainId == null) {
      setOrderError("请先连接钱包。");
      return;
    }
    const picked = unlistedForBatch.filter((n) => batchListPicks.has(n.id));
    if (picked.length === 0) {
      setOrderError("请至少选择一件未挂单的 NFT。");
      return;
    }
    const nftAddressFallback = getBloomNFTAddress(chainId);
    const marketplaceAddress = getBloomMarketplaceAddress(chainId);

    setBatchListSubmitting(true);
    setOrderError(null);
    setOrderSuccess(null);
    try {
      const marketplaceAddressForApprove = getBloomMarketplaceAddress(chainId);
      // 批量上架可能包含不同 NFT 合约（BloomNFT/BloomNFT1），需要分别授权。
      const nftAddressesToApprove = Array.from(
        new Set(
          picked.map((n) => (n.nftAddress || nftAddressFallback).toLowerCase())
        )
      );
      for (const nftAddr of nftAddressesToApprove) {
        const nftContract = getBloomNFTContractAt(signer, nftAddr);
        const approvedForAll: boolean = await nftContract.isApprovedForAll(
          account,
          marketplaceAddressForApprove
        );
        if (!approvedForAll) {
          setOrderSuccess(`正在授权 NFT 合约 ${nftAddr}…`);
          const approveTx = await nftContract.setApprovalForAll(
            marketplaceAddressForApprove,
            true
          );
          await approveTx.wait();
        }
      }

      type LeafMeta = {
        nftListId: number;
        tokenId: number;
        priceNum: number;
        deadlineIso: string;
        salt: bigint;
        nftAddress: string;
        leaf: string;
      };

      const leafMetas: LeafMeta[] = [];
      let rootDeadlineSec = 0n;

      for (const nft of picked) {
        const priceStr = batchListPricesById[nft.id] ?? batchListPrice;
        const priceNum = Number(priceStr);
        if (!Number.isFinite(priceNum) || priceNum <= 0) {
          throw new Error(
            `无效价格（BT）：${priceStr}，NFT：${nft.name} Token#${nft.tokenId}`
          );
        }

        const deadlineStr =
          batchListDeadlinesById[nft.id] ?? batchListDeadline;
        if (!deadlineStr) {
          throw new Error(
            `请选择截止时间，NFT：${nft.name} Token#${nft.tokenId}`
          );
        }

        const deadlineIso = new Date(deadlineStr).toISOString();
        const deadlineSec = BigInt(
          Math.floor(new Date(deadlineIso).getTime() / 1000)
        );

        const priceWei = parseUnits(String(priceNum), 18);
        const salt = randomSalt();

        // 每个 NFT 可能来自不同合约（BloomNFT / BloomNFT1），merkle leaf 必须使用对应合约地址。
        const nftAddress = nft.nftAddress || nftAddressFallback;
        const leaf = listingLeafHash(
          nftAddress,
          account,
          BigInt(nft.tokenId),
          priceWei,
          deadlineSec,
          salt
        );

        leafMetas.push({
          nftListId: nft.id,
          tokenId: nft.tokenId,
          priceNum,
          deadlineIso,
          salt,
          nftAddress,
          leaf,
        });
        if (deadlineSec > rootDeadlineSec) rootDeadlineSec = deadlineSec;
      }

      if (rootDeadlineSec === 0n) {
        throw new Error("批量上架的 rootDeadlineSec 计算失败");
      }

      const leaves = leafMetas.map((m) => m.leaf);
      const { root, layers } = buildMerkleTreeFromLeaves(leaves);
      const rootDeadlineMs = Number(rootDeadlineSec) * 1000;
      const rootDeadlineIso = new Date(rootDeadlineMs).toISOString();

      const batchSig = await signer.signTypedData(
        {
          name: "BloomMarketplace",
          version: "1",
          chainId,
          verifyingContract: marketplaceAddress,
        },
        {
          BatchListing: [
            { name: "merkleRoot", type: "bytes32" },
            { name: "seller", type: "address" },
            { name: "rootDeadline", type: "uint256" },
          ],
        },
        {
          merkleRoot: root,
          seller: account,
          rootDeadline: rootDeadlineSec,
        }
      );

      const items = leafMetas.map((m, idx) => {
        const proof = getMerkleProof(layers, idx);
        return {
          nftListId: m.nftListId,
          seller: account,
          tokenId: m.tokenId,
          price: m.priceNum,
          deadline: m.deadlineIso,
          salt: m.salt.toString(),
          proof,
        };
      });

      const resp = await fetch(API_ENDPOINTS.entryOrdersBatch, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          rootDeadline: rootDeadlineIso,
          merkleRoot: root,
          batchSignature: batchSig,
          items,
        }),
      });
      const data = await resp.json();
      if (!resp.ok || data?.code !== 0) {
        throw new Error(data?.message || "批量挂单失败");
      }
      setOrderSuccess("批量挂单已提交。");
      setBatchListOpen(false);
      await refreshPortfolio();
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setOrderError(msg || "批量挂单失败");
    } finally {
      setBatchListSubmitting(false);
    }
  };

  const toggleCancelBatchPick = (orderId: number) => {
    setCancelBatchPicks((prev) => {
      const n = new Set(prev);
      if (n.has(orderId)) n.delete(orderId);
      else n.add(orderId);
      return n;
    });
  };

  const handleCancelListingsBatch = async () => {
    if (!account || !signer || chainId == null) {
      setError("请先连接钱包。");
      return;
    }
    const orders = listedOrders.filter(
      (o) =>
        cancelBatchPicks.has(o.id) && (o.status === 0 || o.status === 2)
    );
    if (orders.length === 0) {
      setError("请勾选至少一笔进行中的挂单。");
      return;
    }
    const ok = await requestConfirm({
      title: "确认批量下架",
      description: `将对 ${orders.length} 笔挂单执行链上批量下架，NFT 将退回钱包。`,
    });
    if (!ok) return;
    setCancelBatchSubmitting(true);
    setError(null);
    try {
      const mp = getBloomMarketplaceContract(signer, chainId);
      const nftAddr = getBloomNFTAddress(chainId);
      // 合约当前没有 cancelListingsBatch；这里改为逐笔 cancelListingOrder。
      for (const order of orders) {
        const origWei = parseUnits(String(order.price), 18);
        const listing = {
          nft: nftAddr,
          seller: order.seller,
          tokenId: BigInt(order.tokenId),
          price: origWei,
          deadline: BigInt(
            Math.floor(new Date(order.deadline).getTime() / 1000)
          ),
          salt: BigInt(order.salt ?? "0"),
        };
        const tx = await mp.cancelListingOrder(listing);
        await tx.wait();
      }
      setCancelBatchPicks(new Set());
      await refreshPortfolio();
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setError(msg || "批量下架失败");
    } finally {
      setCancelBatchSubmitting(false);
    }
  };

  const handleSubmitEntryOrder = async () => {
    if (!selectedNft) return;
    if (!isConnected || !account) {
      setOrderError("请先连接钱包后再挂单。");
      return;
    }
    if (chainId == null || !signer) {
      setOrderError("无法获取链信息或钱包签名器，请重试。");
      return;
    }

    const priceNum = Number(price);
    if (!Number.isFinite(priceNum) || priceNum <= 0) {
      setOrderError("price 必须大于 0（BT）。");
      return;
    }
    if (!deadlineLocal) {
      setOrderError("请选择截止时间。");
      return;
    }

    const deadlineIso = new Date(deadlineLocal).toISOString();

    // EIP-712 签名：Listing(..., uint256 salt)
    const nftAddress =
      selectedNft.nftAddress || getBloomNFTAddress(chainId);
    const marketplaceAddress = getBloomMarketplaceAddress(chainId);
    const deadlineSeconds = Math.floor(new Date(deadlineIso).getTime() / 1000);

    // 挂单前确保 marketplace 已被 NFT 合约授权（一次授权可复用）。
    try {
      const nftContract = getBloomNFTContractAt(signer, nftAddress);
      const approvedForAll: boolean = await nftContract.isApprovedForAll(
        account,
        marketplaceAddress
      );
      if (!approvedForAll) {
        setOrderError(null);
        setOrderSuccess("正在进行授权，请在钱包中确认...");
        const approveTx = await nftContract.setApprovalForAll(
          marketplaceAddress,
          true
        );
        await approveTx.wait();
      }
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setOrderSuccess(null);
      setOrderError(msg || "NFT 授权失败，请重试。");
      return;
    }

    // 无需读取链上 nonce（避免并发/确认延迟导致 bad nonce）。

    const saltBig = BigInt(hexlify(randomBytes(32)));

    const types: Record<string, Array<{ name: string; type: string }>> = {
      Listing: [
        { name: "nft", type: "address" },
        { name: "seller", type: "address" },
        { name: "tokenId", type: "uint256" },
        { name: "price", type: "uint256" },
        { name: "deadline", type: "uint256" },
        { name: "salt", type: "uint256" },
      ],
    };

    const domain = {
      name: "BloomMarketplace",
      version: "1",
      chainId,
      verifyingContract: marketplaceAddress,
    };

    const value = {
      nft: nftAddress,
      seller: account,
      tokenId: BigInt(selectedNft.tokenId),
      price: parseUnits(String(priceNum), 18),
      deadline: BigInt(deadlineSeconds),
      salt: saltBig,
    };

    let signature: string;
    try {
      // Keep local hash calculation for debugging/verification if needed.
      void TypedDataEncoder.hash(domain, types, value);
      signature = await signer.signTypedData(domain, types, value);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setOrderError(msg || "签名失败，请检查钱包授权。");
      return;
    }

    const body = {
      seller: account, // 与卖家一致
      tokenId: Number(selectedNft.tokenId),
      price: priceNum,
      deadline: deadlineIso,
      salt: saltBig.toString(),
      nftListId: Number(selectedNft.id),
      signature,
    };

    setOrderSubmitting(true);
    setOrderError(null);
    setOrderSuccess(null);
    try {
      const resp = await fetch(API_ENDPOINTS.entryOrders, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const data = await resp.json();
      if (!resp.ok || data?.code !== 0) {
        throw new Error(data?.message || "挂单失败");
      }
      setOrderSuccess("挂单成功");
      setOrderDialogOpen(false);
      await refreshPortfolio();
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setOrderError(msg || "挂单失败");
    } finally {
      setOrderSubmitting(false);
    }
  };

  const handleCancelListing = async (nft: NftItem) => {
    if (!isConnected || !account || chainId == null || !signer) {
      setError("请先连接钱包后再取消上架。");
      return;
    }

    const order = activeListingByNftListId[nft.id];
    if (!order) {
      setError("未找到对应挂单信息，无法取消上架。");
      return;
    }
    if (order.status !== 0 && order.status !== 2) {
      setError("当前状态不可取消上架。");
      return;
    }
    if (account.toLowerCase() !== order.seller.toLowerCase()) {
      setError("仅卖家本人可取消上架。");
      return;
    }

    try {
      setCancelSubmittingId(nft.id);
      setError(null);

      const marketplace = getBloomMarketplaceContract(signer, chainId);
      const lhRaw = order.listingHash?.trim();
      if (!lhRaw) {
        setError("暂无 listingHash（请等待上架同步后再试）。");
        return;
      }
      const origWei = parseUnits(String(order.price), 18);
      const listing = {
        nft: getBloomNFTAddress(chainId),
        seller: order.seller,
        tokenId: BigInt(order.tokenId),
        price: origWei,
        deadline: BigInt(Math.floor(new Date(order.deadline).getTime() / 1000)),
        salt: BigInt(order.salt ?? "0"),
      };

      const tx = await marketplace.cancelListingOrder(listing);
      await tx.wait();

      await refreshPortfolio();
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setError(msg || "取消上架失败");
    } finally {
      setCancelSubmittingId(null);
    }
  };

  const openChangePriceDialog = (nft: NftItem) => {
    const ao = activeListingByNftListId[nft.id];
    setChangePriceNft(nft);
    setChangePriceValue(ao ? String(ao.price) : "");
    setChangePriceError(null);
    setChangePriceDialogOpen(true);
  };

  const handleSubmitReducePrice = async () => {
    if (!changePriceNft || !account || !signer || chainId == null) {
      setChangePriceError("请先连接钱包。");
      return;
    }
    const ao = activeListingByNftListId[changePriceNft.id];
    if (!ao || ao.status !== 0) {
      setChangePriceError("未找到进行中的挂单。");
      return;
    }
    if (account.toLowerCase() !== ao.seller.toLowerCase()) {
      setChangePriceError("仅卖家本人可改价。");
      return;
    }
    const newNum = Number(changePriceValue);
    const curNum = Number(ao.price);
    if (!Number.isFinite(newNum) || newNum <= 0) {
      setChangePriceError("请输入大于 0 的有效价格（BT）。");
      return;
    }
    // 规则：
    // - 提高价格：必须先取消上架（下架当前订单），然后用「挂单」重新签名上架
    // - 降低价格：不需要先下架，直接生成新的挂单订单（新的签名/新的 listingHash）
    if (newNum >= curNum) {
      setChangePriceError(
        "改高价格：请先在菜单中「取消上架」下架当前订单，然后重新签名上架更高价。"
      );
      return;
    }

    setChangePriceSubmitting(true);
    setChangePriceError(null);
    try {
      const newWei = parseUnits(String(newNum), 18);
      const deadlineSeconds = Math.floor(
        new Date(ao.deadline).getTime() / 1000
      );
      if (!Number.isFinite(deadlineSeconds) || deadlineSeconds <= 0) {
        throw new Error("无效的截止时间（deadline）。");
      }

      // 复用「挂单」签名逻辑：Listing EIP-712（链上只做结算，订单签名由签名者离线完成）
      const nftAddress =
        changePriceNft.nftAddress || getBloomNFTAddress(chainId);
      const marketplaceAddress = getBloomMarketplaceAddress(chainId);
      const saltBig = BigInt(hexlify(randomBytes(32)));

      const domain = {
        name: "BloomMarketplace",
        version: "1",
        chainId,
        verifyingContract: marketplaceAddress,
      };

      const types: Record<string, Array<{ name: string; type: string }>> = {
        Listing: [
          { name: "nft", type: "address" },
          { name: "seller", type: "address" },
          { name: "tokenId", type: "uint256" },
          { name: "price", type: "uint256" },
          { name: "deadline", type: "uint256" },
          { name: "salt", type: "uint256" },
        ],
      };

      const value = {
        nft: nftAddress,
        seller: account,
        tokenId: BigInt(ao.tokenId),
        price: newWei,
        deadline: BigInt(deadlineSeconds),
        salt: saltBig,
      };

      // 挂单前确保 marketplace 已被 NFT 合约授权
      const nftContract = getBloomNFTContractAt(signer, nftAddress);
      const approvedForAll: boolean = await nftContract.isApprovedForAll(
        account,
        marketplaceAddress
      );
      if (!approvedForAll) {
        const approveTx = await nftContract.setApprovalForAll(
          marketplaceAddress,
          true
        );
        await approveTx.wait();
      }

      const signature = await signer.signTypedData(domain, types, value);

      const deadlineIso = new Date(deadlineSeconds * 1000).toISOString();
      const body = {
        seller: account,
        tokenId: Number(ao.tokenId),
        price: newNum,
        deadline: deadlineIso,
        salt: saltBig.toString(),
        nftListId: Number(changePriceNft.id),
        signature,
      };

      const resp = await fetch(API_ENDPOINTS.entryOrders, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const data = await resp.json();
      if (!resp.ok || data?.code !== 0) {
        throw new Error(data?.message || "挂单失败");
      }

      setChangePriceDialogOpen(false);
      setChangePriceNft(null);
      await refreshPortfolio();
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setChangePriceError(msg || "降价失败");
    } finally {
      setChangePriceSubmitting(false);
    }
  };

  const loadBidsForDialog = async (nft: NftItem) => {
    if (!account) return;
    setBidLoading(true);
    setBidError(null);
    try {
      const resp = await fetch(API_ENDPOINTS.bidListForSeller(nft.id, account));
      const data = await resp.json();
      if (!resp.ok || data?.code !== 0) {
        throw new Error(data?.message || "获取出价列表失败");
      }
      setBidList((data.data as BidPlacedItem[]) || []);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setBidError(msg || "获取出价列表失败");
      setBidList([]);
    } finally {
      setBidLoading(false);
    }
  };

  const openBidDialog = (nft: NftItem) => {
    if (!account) return;
    setBidDialogNft(nft);
    setBidDialogOpen(true);
    setBidError(null);
    setBidList([]);
    void loadBidsForDialog(nft);
  };

  const handleAcceptBid = async (bid: BidPlacedItem) => {
    if (!account || !bidDialogNft) return;
    const ok = await requestConfirm({
      title: "确认接受出价",
      description: `确定接受该买家的出价吗？\n\n价格：${bid.price} BT\n买家：${bid.buyer}\n\n接受后将由服务端完成成交，无需在钱包中签名，请确认无误后再操作。`,
    });
    if (!ok) return;
    setAcceptBidId(bid.id);
    setBidError(null);
    try {
      const body: Record<string, unknown> = {
        bidId: bid.id,
        seller: account,
      };
      const resp = await fetch(API_ENDPOINTS.bidAccepted, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const data = await resp.json();
      if (!resp.ok || data?.code !== 0) {
        throw new Error(data?.message || "接受出价失败");
      }
      await loadBidsForDialog(bidDialogNft);
      await refreshPortfolio();
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setBidError(msg || "接受出价失败");
    } finally {
      setAcceptBidId(null);
    }
  };

  useEffect(() => {
    if (!isConnected || !account) {
      setCategories([]);
      setAllOwnedNfts([]);
      setActiveListingByNftListId({});
      return;
    }

    const fetchCategories = async () => {
      try {
        setLoading(true);
        setError(null);
        const resp = await fetch(
          `${API_ENDPOINTS.nftUserCategories}?owner=${account}`
        );
        const data = await resp.json();
        if (!resp.ok || data.code !== 0) {
          throw new Error(data.message || "获取 NFT 类目失败");
        }
        const list: NftCategory[] = data.data || [];
        setCategories(list);
        if (list.length === 0) {
          setAllOwnedNfts([]);
        }
      } catch (e: unknown) {
        const msg = e instanceof Error ? e.message : undefined;
        setError(msg || "获取 NFT 类目失败");
      } finally {
        setLoading(false);
      }
    };

    void fetchCategories();
  }, [isConnected, account]);

  useEffect(() => {
    if (!isConnected || !account || categories.length === 0) {
      setAllOwnedNfts([]);
      setActiveListingByNftListId({});
      return;
    }
    void refreshPortfolio();
  }, [isConnected, account, categories, refreshPortfolio]);

  return (
    <>
      <Stack
        direction="row"
        flexWrap="wrap"
        alignItems="center"
        justifyContent="space-between"
        gap={2}
        sx={{ mb: 3 }}
      >
        <Typography variant="h4" sx={{ fontWeight: 700 }}>
          个人中心
        </Typography>
        {isConnected && (
          <Stack direction="row" spacing={1}>
            <Button
              variant="outlined"
              size="small"
              onClick={() => {
                setHistoryOrdersOpen(true);
                void fetchHistoryOrders();
              }}
            >
              历史挂单
            </Button>
            <Button
              variant="outlined"
              size="small"
              onClick={() => {
                setHistoryBidsOpen(true);
                void fetchHistoryBids();
              }}
            >
              历史出价
            </Button>
            <Button
              variant="contained"
              size="small"
              onClick={() => openBatchListDialog()}
            >
              批量上架（Merkle）
            </Button>
          </Stack>
        )}
      </Stack>
      {!isConnected ? (
        <Typography>请先连接钱包查看个人信息。</Typography>
      ) : (
        <>
          {loading && (
            <Box sx={{ display: "flex", justifyContent: "center", my: 4 }}>
              <CircularProgress />
            </Box>
          )}

          {error && (
            <Typography color="error" sx={{ mb: 2 }}>
              {error}
            </Typography>
          )}

          {categories.length === 0 && !loading ? (
            <Typography>当前地址暂无持有的 NFT 类目。</Typography>
          ) : (
            <>
              <Tabs
                value={profileTab}
                onChange={(_, v) => setProfileTab(v)}
                sx={{ mb: 2 }}
              >
                <Tab label="我持有的" />
                <Tab label={`我上架的 (${listedOrders.length})`} />
              </Tabs>

              {profileTab === 0 && (
                <TableContainer component={Paper} variant="outlined">
                  <Table size="small">
                    <TableHead>
                      <TableRow>
                        <TableCell width={80}>图片</TableCell>
                        <TableCell>名称</TableCell>
                        <TableCell>类目</TableCell>
                        <TableCell>Token ID</TableCell>
                        <TableCell align="right" width={200}>
                          操作
                        </TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {allOwnedNfts.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={5} align="center">
                            暂无持有的 NFT
                          </TableCell>
                        </TableRow>
                      ) : (
                        allOwnedNfts.map((nft) => (
                          <TableRow
                            key={`${nft.categoryId}-${nft.id}`}
                            hover
                          >
                            <TableCell>
                              <Box
                                component="img"
                                src={nft.imageUrl}
                                alt=""
                                sx={{
                                  width: 48,
                                  height: 48,
                                  objectFit: "cover",
                                  borderRadius: 1,
                                  display: "block",
                                }}
                              />
                            </TableCell>
                            <TableCell>{nft.name}</TableCell>
                            <TableCell>{nft.categoryName}</TableCell>
                            <TableCell>{nft.tokenId}</TableCell>
                            <TableCell align="right">
                              <Stack
                                direction="row"
                                spacing={1}
                                justifyContent="flex-end"
                                flexWrap="wrap"
                                useFlexGap
                              >
                                <Button
                                  variant="contained"
                                  size="small"
                                  disabled={!isConnected}
                                  onClick={() => openEntryOrderDialog(nft)}
                                >
                                  挂单
                                </Button>
                                <Button
                                  variant="outlined"
                                  size="small"
                                  onClick={() => {
                                    setOwnedDetailNft(nft);
                                    setOwnedDetailOpen(true);
                                  }}
                                >
                                  查看详情
                                </Button>
                              </Stack>
                            </TableCell>
                          </TableRow>
                        ))
                      )}
                    </TableBody>
                  </Table>
                </TableContainer>
              )}

              {profileTab === 1 && (
                <TableContainer component={Paper} variant="outlined">
                  <Stack direction="row" justifyContent="flex-end" sx={{ mb: 1 }}>
                    <Button
                      variant="outlined"
                      color="warning"
                      size="small"
                      disabled={
                        cancelBatchSubmitting || cancelBatchPicks.size === 0
                      }
                      onClick={() => void handleCancelListingsBatch()}
                    >
                      {cancelBatchSubmitting
                        ? "批量下架中…"
                        : `批量下架 (${cancelBatchPicks.size})`}
                    </Button>
                  </Stack>
                  <Table size="small">
                    <TableHead>
                      <TableRow>
                        <TableCell padding="checkbox" width={40}>
                          选
                        </TableCell>
                        <TableCell width={80}>图片</TableCell>
                        <TableCell>名称</TableCell>
                        <TableCell>Token ID</TableCell>
                        <TableCell align="right">价格 (BT)</TableCell>
                        <TableCell>截止时间</TableCell>
                        <TableCell>状态</TableCell>
                        <TableCell>订单 ID</TableCell>
                        <TableCell align="right" width={140}>
                          操作
                        </TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {listedOrders.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={9} align="center">
                            暂无进行中的上架
                          </TableCell>
                        </TableRow>
                      ) : (
                        listedOrders.map((order) => {
                          const displayName =
                            nftNameByListId.get(order.nftListId) ??
                            `Token #${order.tokenId}`;
                          const rowNft = entryOrderToNftItem(
                            order,
                            displayName
                          );
                          return (
                            <TableRow key={order.id} hover>
                              <TableCell padding="checkbox">
                                <Checkbox
                                  size="small"
                                  checked={cancelBatchPicks.has(order.id)}
                                    disabled={order.status !== 0 && order.status !== 2}
                                  onChange={() => toggleCancelBatchPick(order.id)}
                                />
                              </TableCell>
                              <TableCell>
                                <Box
                                  component="img"
                                  src={order.imageUrl || ""}
                                  alt=""
                                  sx={{
                                    width: 48,
                                    height: 48,
                                    objectFit: "cover",
                                    borderRadius: 1,
                                    display: "block",
                                  }}
                                />
                              </TableCell>
                              <TableCell>{displayName}</TableCell>
                              <TableCell>{order.tokenId}</TableCell>
                              <TableCell align="right">{order.price}</TableCell>
                              <TableCell>
                                {order.deadline
                                  ? new Date(order.deadline).toLocaleString()
                                  : "—"}
                              </TableCell>
                              <TableCell>
                                {order.statusDesc ??
                                  ORDER_STATUS_LABELS[order.status] ??
                                  order.status}
                              </TableCell>
                              <TableCell>{order.id}</TableCell>
                              <TableCell align="right">
                                <Button
                                  variant="contained"
                                  size="small"
                                  disabled={!isConnected}
                                  endIcon={<KeyboardArrowDownIcon />}
                                  onClick={(e) =>
                                    setActionMenu({
                                      anchor: e.currentTarget,
                                      nft: rowNft,
                                    })
                                  }
                                  sx={{
                                    minWidth: 100,
                                    justifyContent: "space-between",
                                  }}
                                >
                                  操作
                                </Button>
                              </TableCell>
                            </TableRow>
                          );
                        })
                      )}
                    </TableBody>
                  </Table>
                </TableContainer>
              )}
            </>
          )}
        </>
      )}

      <Dialog
        open={ownedDetailOpen}
        onClose={() => {
          setOwnedDetailOpen(false);
          setOwnedDetailNft(null);
        }}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>NFT 详情</DialogTitle>
        <DialogContent dividers>
          {ownedDetailNft ? (
            <Stack spacing={2} sx={{ pt: 0.5 }}>
              <Box
                component="img"
                src={ownedDetailNft.imageUrl}
                alt={ownedDetailNft.name}
                sx={{
                  width: "100%",
                  maxHeight: 280,
                  objectFit: "contain",
                  borderRadius: 1,
                  bgcolor: "action.hover",
                }}
              />
              <Typography variant="h6">{ownedDetailNft.name}</Typography>
              <Typography variant="body2" color="text.secondary">
                {ownedDetailNft.description || "暂无描述"}
              </Typography>
              <Divider />
              <Stack spacing={0.75}>
                <Typography variant="body2">
                  <strong>类目：</strong>
                  {ownedDetailNft.categoryName}
                </Typography>
                <Typography variant="body2">
                  <strong>Token ID：</strong>
                  {ownedDetailNft.tokenId}
                </Typography>
                <Typography variant="body2">
                  <strong>列表 ID：</strong>
                  {ownedDetailNft.id}
                </Typography>
                {ownedDetailNft.metadataUrl ? (
                  <Typography variant="body2">
                    <strong>元数据：</strong>
                    <Link
                      href={ownedDetailNft.metadataUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      打开链接
                    </Link>
                  </Typography>
                ) : null}
                {ownedDetailNft.tokenUrl ? (
                  <Typography variant="body2" sx={{ wordBreak: "break-all" }}>
                    <strong>Token URL：</strong>
                    {ownedDetailNft.tokenUrl}
                  </Typography>
                ) : null}
                {(() => {
                  const ao = activeListingByNftListId[ownedDetailNft.id];
                  if (!ao) {
                    return (
                      <Typography variant="body2" color="text.secondary">
                        <strong>挂单：</strong>当前无进行中的上架（可在「上架中的
                        NFT」中管理已上架项）
                      </Typography>
                    );
                  }
                  return (
                    <Typography variant="body2">
                      <strong>挂单状态：</strong>
                      {ao.statusDesc ??
                        ORDER_STATUS_LABELS[ao.status] ??
                        ao.status}
                      （订单 #{ao.id}，请到「上架中的 NFT」进行出价/取消等操作）
                    </Typography>
                  );
                })()}
              </Stack>
            </Stack>
          ) : null}
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => {
              setOwnedDetailOpen(false);
              setOwnedDetailNft(null);
            }}
          >
            关闭
          </Button>
        </DialogActions>
      </Dialog>

      <Menu
        anchorEl={actionMenu.anchor}
        open={Boolean(actionMenu.anchor)}
        onClose={closeActionMenu}
        anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
        transformOrigin={{ vertical: "top", horizontal: "right" }}
        slotProps={{
          paper: {
            elevation: 8,
            sx: { minWidth: 168, borderRadius: 2, mt: 0.5 },
          },
        }}
      >
        {actionMenu.nft
          ? (() => {
              const n = actionMenu.nft;
              const ao = activeListingByNftListId[n.id];
              const showViewBids =
                !!account &&
                !!ao &&
                ao.status === 0 &&
                ao.seller.toLowerCase() === account.toLowerCase();
              return [
                showViewBids ? (
                  <MenuItem
                    key="view-bids"
                    onClick={() => {
                      closeActionMenu();
                      openBidDialog(n);
                    }}
                  >
                    查看出价
                  </MenuItem>
                ) : null,
                showViewBids ? (
                  <MenuItem
                    key="change-price"
                    onClick={() => {
                      closeActionMenu();
                      openChangePriceDialog(n);
                    }}
                  >
                    修改价格
                  </MenuItem>
                ) : null,
                <MenuItem
                  key="cancel-listing"
                  onClick={() => {
                    closeActionMenu();
                    void handleCancelListing(n);
                  }}
                  disabled={cancelSubmittingId === n.id}
                >
                  {cancelSubmittingId === n.id ? "取消中…" : "取消上架"}
                </MenuItem>,
                <Divider key="after-cancel" />,
                <MenuItem
                  key="create-order"
                  onClick={() => {
                    closeActionMenu();
                    openEntryOrderDialog(n);
                  }}
                >
                  挂单
                </MenuItem>,
              ];
            })()
          : null}
      </Menu>

      <Dialog
        open={orderDialogOpen}
        onClose={() => {
          if (orderSubmitting) return;
          setOrderDialogOpen(false);
        }}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>填写挂单信息</DialogTitle>
        <DialogContent sx={{ py: 1 }}>
          <Stack spacing={1.5} sx={{ mt: 1 }}>
            {orderError && <Alert severity="error">{orderError}</Alert>}
            {orderSuccess && <Alert severity="success">{orderSuccess}</Alert>}

            <TextField
              label="挂单价（BT）"
              type="number"
              inputProps={{ min: 0, step: "0.0001" }}
              value={price}
              fullWidth
              size="small"
              onChange={(e) => setPrice(e.target.value)}
            />

            <TextField
              label="截止时间（deadline）"
              type="datetime-local"
              value={deadlineLocal}
              fullWidth
              InputLabelProps={{ shrink: true }}
              onChange={(e) => {
                setDeadlineLocal(
                  normalizeHourOnlyDateTimeLocal(e.target.value)
                );
              }}
              inputProps={{ step: "3600" }}
              size="small"
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setOrderDialogOpen(false)} disabled={orderSubmitting}>
            取消
          </Button>
          <Button variant="contained" onClick={() => void handleSubmitEntryOrder()} disabled={orderSubmitting}>
            {orderSubmitting ? "提交中..." : "确认挂单"}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={batchListOpen}
        onClose={(_, reason) => {
          if (batchListSubmitting) return;
          // datetime-local 的交互可能触发 backdrop/escape 关闭；这里忽略这些关闭原因，
          // 避免用户选完时间后对话框被意外关闭。
          if (reason === "backdropClick" || reason === "escapeKeyDown") return;
          setBatchListOpen(false);
        }}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>Merkle 批量上架</DialogTitle>
        <DialogContent sx={{ py: 1 }}>
          <Stack spacing={1.5} sx={{ mt: 1 }}>
            <Alert severity="info">
              勾选未挂单的 NFT 后，可为每个 NFT 单独填写价格与截止时间；将签名一次 Merkle
              根并由服务端逐笔 listWithMerkleProof 上链。Listing.nonce 为 0，请勿与单笔挂单混淆。
            </Alert>
            {orderError && <Alert severity="error">{orderError}</Alert>}
            {orderSuccess && <Alert severity="success">{orderSuccess}</Alert>}
            <TextField
              label="默认价格（BT）"
              type="number"
              inputProps={{ min: 0, step: "0.0001" }}
              value={batchListPrice}
              fullWidth
              size="small"
              onChange={(e) => setBatchListPrice(e.target.value)}
            />
            <TextField
              label="默认截止时间"
              type="datetime-local"
              value={batchListDeadline}
              fullWidth
              InputLabelProps={{ shrink: true }}
              size="small"
              onChange={(e) => {
                setBatchListDeadline(
                  normalizeHourOnlyDateTimeLocal(e.target.value)
                );
              }}
              inputProps={{ step: "3600" }}
            />
            <Typography variant="subtitle2">选择 NFT（未挂单）</Typography>
            <Stack spacing={0.5} sx={{ maxHeight: 240, overflow: "auto" }}>
              {unlistedForBatch.length === 0 ? (
                <Typography color="text.secondary">暂无可批量上架的 NFT</Typography>
              ) : (
                unlistedForBatch.map((nft) => {
                  const picked = batchListPicks.has(nft.id);
                  const priceValue = batchListPricesById[nft.id] ?? batchListPrice;
                  const deadlineValue =
                    batchListDeadlinesById[nft.id] ?? batchListDeadline;
                  return (
                    <Stack
                      key={`${nft.categoryId}-${nft.id}`}
                      spacing={0.5}
                      sx={{ py: 0.5 }}
                    >
                      <Stack
                        direction="row"
                        alignItems="center"
                        spacing={1}
                      >
                        <Checkbox
                          size="small"
                          checked={picked}
                          onChange={() => toggleBatchListPick(nft.id)}
                        />
                        <Typography variant="body2">
                          {nft.name} · Token #{nft.tokenId}
                        </Typography>
                      </Stack>

                      {picked ? (
                        <Stack
                          direction="row"
                          spacing={1}
                          alignItems="center"
                          sx={{ pl: 4, pr: 1, width: "100%" }}
                        >
                          <TextField
                            label="价格（BT）"
                            type="number"
                            inputProps={{ min: 0, step: "0.0001" }}
                            value={priceValue}
                            size="small"
                            sx={{ width: 140 }}
                            onChange={(e) =>
                              setBatchListPricesById((prev) => ({
                                ...prev,
                                [nft.id]: e.target.value,
                              }))
                            }
                          />
                          <TextField
                            label="截止时间"
                            type="datetime-local"
                            value={deadlineValue}
                            InputLabelProps={{ shrink: true }}
                            size="small"
                            sx={{ flex: 1, minWidth: 220 }}
                            onChange={(e) =>
                              setBatchListDeadlinesById((prev) => ({
                                ...prev,
                                [nft.id]: normalizeHourOnlyDateTimeLocal(
                                  e.target.value
                                ),
                              }))
                            }
                            inputProps={{ step: "3600" }}
                          />
                        </Stack>
                      ) : null}
                    </Stack>
                  );
                })
              )}
            </Stack>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => setBatchListOpen(false)}
            disabled={batchListSubmitting}
          >
            关闭
          </Button>
          <Button
            variant="contained"
            disabled={batchListSubmitting || batchListPicks.size === 0}
            onClick={() => void handleSubmitBatchMerkleList()}
          >
            {batchListSubmitting ? "提交中…" : "签名并提交"}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={changePriceDialogOpen}
        onClose={() => !changePriceSubmitting && setChangePriceDialogOpen(false)}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>修改挂单价</DialogTitle>
        <DialogContent sx={{ py: 1 }}>
          <Stack spacing={1.5} sx={{ mt: 1 }}>
            <Alert severity="info">
              降价：在下方填写<strong>低于当前价</strong>的新价格，直接重新签名上架新的订单（无需先取消上架）。
              提价：请先「取消上架」下架当前订单，然后再「挂单」以更高价重新上架。
            </Alert>
            {changePriceError && (
              <Alert severity="error">{changePriceError}</Alert>
            )}
            {changePriceNft ? (
              <Typography variant="body2" color="text.secondary">
                {changePriceNft.name}（当前价{" "}
                {activeListingByNftListId[changePriceNft.id]?.price ?? "-"} BT）
              </Typography>
            ) : null}
            <TextField
              label="新价格（BT）"
              type="number"
              inputProps={{ min: 0, step: "0.0001" }}
              value={changePriceValue}
              fullWidth
              size="small"
              onChange={(e) => setChangePriceValue(e.target.value)}
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => setChangePriceDialogOpen(false)}
            disabled={changePriceSubmitting}
          >
            关闭
          </Button>
          <Button
            variant="contained"
            onClick={() => void handleSubmitReducePrice()}
            disabled={changePriceSubmitting}
          >
            {changePriceSubmitting ? "链上确认中…" : "确认降价"}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={bidDialogOpen}
        onClose={() => setBidDialogOpen(false)}
        maxWidth="md"
        fullWidth
      >
        <DialogTitle>
          出价列表{bidDialogNft ? ` — ${bidDialogNft.name}` : ""}
        </DialogTitle>
        <DialogContent>
          {bidError && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {bidError}
            </Alert>
          )}
          {bidLoading ? (
            <Box sx={{ display: "flex", justifyContent: "center", py: 4 }}>
              <CircularProgress />
            </Box>
          ) : (
            <TableContainer component={Paper} variant="outlined">
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>买家</TableCell>
                    <TableCell align="right">价格 (BT)</TableCell>
                    <TableCell>截止时间</TableCell>
                    <TableCell>状态</TableCell>
                    <TableCell align="right">操作</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {bidList.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5} align="center">
                        暂无出价
                      </TableCell>
                    </TableRow>
                  ) : (
                    bidList.map((bid) => {
                      const listingOrder = bidDialogNft
                        ? activeListingByNftListId[bidDialogNft.id]
                        : undefined;
                      const canAccept =
                        bid.status === 0 && listingOrder?.status === 0;
                      return (
                        <TableRow key={bid.id}>
                          <TableCell sx={{ maxWidth: 140, wordBreak: "break-all" }}>
                            {bid.buyer}
                          </TableCell>
                          <TableCell align="right">{bid.price}</TableCell>
                          <TableCell>
                            {bid.deadline
                              ? new Date(bid.deadline).toLocaleString()
                              : "-"}
                          </TableCell>
                          <TableCell>
                            {BID_STATUS_LABELS[bid.status] ?? String(bid.status)}
                          </TableCell>
                          <TableCell align="right">
                            <Button
                              size="small"
                              variant="contained"
                              color="success"
                              disabled={
                                !canAccept || acceptBidId === bid.id
                              }
                              onClick={() => void handleAcceptBid(bid)}
                            >
                              {acceptBidId === bid.id ? "处理中..." : "接受出价"}
                            </Button>
                          </TableCell>
                        </TableRow>
                      );
                    })
                  )}
                </TableBody>
              </Table>
            </TableContainer>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setBidDialogOpen(false)}>关闭</Button>
          <Button
            onClick={() => bidDialogNft && void loadBidsForDialog(bidDialogNft)}
            disabled={bidLoading || !bidDialogNft}
          >
            刷新
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={historyOrdersOpen}
        onClose={() => setHistoryOrdersOpen(false)}
        maxWidth="lg"
        fullWidth
      >
        <DialogTitle>我的挂单记录</DialogTitle>
        <DialogContent>
          {historyError && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {historyError}
            </Alert>
          )}
          {historyOrdersLoading ? (
            <Box sx={{ display: "flex", justifyContent: "center", py: 4 }}>
              <CircularProgress />
            </Box>
          ) : (
            <TableContainer component={Paper} variant="outlined" sx={{ mt: 1 }}>
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>订单ID</TableCell>
                    <TableCell>NFT</TableCell>
                    <TableCell>Token</TableCell>
                    <TableCell align="right">价格(BT)</TableCell>
                    <TableCell>状态</TableCell>
                    <TableCell>截止时间</TableCell>
                    <TableCell>买家</TableCell>
                    <TableCell>创建时间</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {historyOrders.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={8} align="center">
                        暂无挂单记录
                      </TableCell>
                    </TableRow>
                  ) : (
                    historyOrders.map((row) => (
                      <TableRow key={row.id}>
                        <TableCell>{row.id}</TableCell>
                        <TableCell>
                          {row.imageUrl ? (
                            <Box
                              component="img"
                              src={row.imageUrl}
                              alt=""
                              sx={{ width: 40, height: 40, objectFit: "cover", borderRadius: 1 }}
                            />
                          ) : (
                            "-"
                          )}
                        </TableCell>
                        <TableCell>{row.tokenId}</TableCell>
                        <TableCell align="right">{row.price}</TableCell>
                        <TableCell>
                          {row.statusDesc ??
                            ORDER_STATUS_LABELS[row.status] ??
                            row.status}
                        </TableCell>
                        <TableCell>
                          {row.deadline
                            ? new Date(row.deadline).toLocaleString()
                            : "-"}
                        </TableCell>
                        <TableCell sx={{ maxWidth: 120, wordBreak: "break-all" }}>
                          {row.buyer || "-"}
                        </TableCell>
                        <TableCell>
                          {row.createTime
                            ? new Date(row.createTime).toLocaleString()
                            : "-"}
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </TableContainer>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setHistoryOrdersOpen(false)}>关闭</Button>
          <Button onClick={() => void fetchHistoryOrders()} disabled={historyOrdersLoading}>
            刷新
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog
        open={historyBidsOpen}
        onClose={() => setHistoryBidsOpen(false)}
        maxWidth="lg"
        fullWidth
      >
        <DialogTitle>我的出价记录</DialogTitle>
        <DialogContent>
          {historyError && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {historyError}
            </Alert>
          )}
          {historyBidsLoading ? (
            <Box sx={{ display: "flex", justifyContent: "center", py: 4 }}>
              <CircularProgress />
            </Box>
          ) : (
            <TableContainer component={Paper} variant="outlined" sx={{ mt: 1 }}>
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>NFT</TableCell>
                    <TableCell>Token</TableCell>
                    <TableCell>卖家</TableCell>
                    <TableCell align="right">出价(BT)</TableCell>
                    <TableCell>状态</TableCell>
                    <TableCell>出价截止</TableCell>
                    <TableCell>挂单ID</TableCell>
                    <TableCell>创建时间</TableCell>
                    <TableCell align="right">操作</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {historyBids.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={9} align="center">
                        暂无出价记录
                      </TableCell>
                    </TableRow>
                  ) : (
                    historyBids.map((row) => (
                      <TableRow key={row.id}>
                        <TableCell>
                          {row.imageUrl ? (
                            <Box
                              component="img"
                              src={row.imageUrl}
                              alt=""
                              sx={{ width: 40, height: 40, objectFit: "cover", borderRadius: 1 }}
                            />
                          ) : (
                            "-"
                          )}
                        </TableCell>
                        <TableCell>{row.tokenId}</TableCell>
                        <TableCell sx={{ maxWidth: 120, wordBreak: "break-all" }}>
                          {row.entrySeller}
                        </TableCell>
                        <TableCell align="right">{row.price}</TableCell>
                        <TableCell>
                          {row.statusDesc ??
                            BID_STATUS_LABELS[row.status] ??
                            row.status}
                        </TableCell>
                        <TableCell>
                          {row.deadline
                            ? new Date(row.deadline).toLocaleString()
                            : "-"}
                        </TableCell>
                        <TableCell>{row.ordersId}</TableCell>
                        <TableCell>
                          {row.createTime
                            ? new Date(row.createTime).toLocaleString()
                            : "-"}
                        </TableCell>
                        <TableCell align="right">
                        {null}
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </TableContainer>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setHistoryBidsOpen(false)}>关闭</Button>
          <Button onClick={() => void fetchHistoryBids()} disabled={historyBidsLoading}>
            刷新
          </Button>
        </DialogActions>
      </Dialog>
      {confirmDialog}
    </>
  );
}

