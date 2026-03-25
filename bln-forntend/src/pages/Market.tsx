import {
  Alert,
  Box,
  Breadcrumbs,
  Button,
  Card,
  CardContent,
  CircularProgress,
  Dialog,
  Link,
  Paper,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  InputLabel,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Select,
  MenuItem,
  Stack,
  TextField,
  Typography,
  Checkbox,
} from "@mui/material";
import { hexlify, parseUnits, randomBytes } from "ethers";
import { useEffect, useState } from "react";
import { API_ENDPOINTS } from "../config/api";
import { useConfirmDialog } from "../components/ConfirmDialog";
import { useWeb3 } from "../web3/provider";
import {
  getBloomMarketplaceAddress,
  getBloomMarketplaceContract,
  getBloomNFTAddress,
  getBloomTokenContract,
} from "../web3/contracts";

interface EntryOrder {
  id: number;
  nftListId: number;
  seller: string;
  buyer: string;
  tokenId: number;
  price: number;
  deadline: string; // backend time.Time (string)
  /** Listing EIP-712 salt（十进制字符串） */
  salt?: string;
  status: number;
  statusDesc?: string;
  signature: string;
  createTime: string;
  updateTime: string;
  imageUrl: string;
  /** 链上 listingHash，未上链同步前可能为空 */
  listingHash?: string;
  /** Merkle 批量上架：购买时无需 listing 单笔签名 */
  isMerkle?: boolean;

  // buy / buyBatch 的 Merkle 参数（仅当 isMerkle=true 时生效）
  merkleRoot?: string;
  rootDeadlineSec?: number;
  merkleProof?: string[];
}

interface BidItem {
  id: number;
  ordersId: number;
  buyer: string;
  price: number;
  deadline: string;
  salt: string;
  status: number;
  txHash?: string;
  signature?: string;
  bidHash?: string;
}

function bidStatusText(status: number) {
  switch (status) {
    case 0:
      return "进行中";
    case 1:
      return "已成交";
    case 2:
      return "已过期";
    case 3:
      return "取消出价";
    case 4:
      return "已下架";
    case 5:
      return "未中标";
    case 6:
      return "已退款";
    default:
      return String(status);
  }
}

function formatDateTime(v: string) {
  if (!v) return "";
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return v;
  return d.toLocaleString();
}

export function Market() {
  const { account, isConnected, chainId, signer } = useWeb3();
  const { requestConfirm, confirmDialog } = useConfirmDialog();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [orders, setOrders] = useState<EntryOrder[]>([]);
  const [statusFilter, setStatusFilter] = useState<number | "">("");
  const [buySubmittingId, setBuySubmittingId] = useState<number | null>(null);
  const [bidSubmittingId, setBidSubmittingId] = useState<number | null>(null);
  const [bidDialogOpen, setBidDialogOpen] = useState(false);
  const [selectedOrder, setSelectedOrder] = useState<EntryOrder | null>(null);
  const [bidPrice, setBidPrice] = useState("1");
  const [bidDeadlineLocal, setBidDeadlineLocal] = useState("");
  const [detailOrder, setDetailOrder] = useState<EntryOrder | null>(null);
  const [bidList, setBidList] = useState<BidItem[]>([]);
  const [bidListLoading, setBidListLoading] = useState(false);
  const [cancelBidId, setCancelBidId] = useState<number | null>(null);
  /** 购物车：订单 id 集合，用于 buyBatch */
  const [cartIds, setCartIds] = useState<Set<number>>(() => new Set());
  const [buyBatchSubmitting, setBuyBatchSubmitting] = useState(false);

  const defaultDeadlineLocal = () => {
    const d = new Date();
    d.setDate(d.getDate() + 1);
    const pad = (v: number) => String(v).padStart(2, "0");
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(
      d.getMinutes()
    )}`;
  };

  const fetchOrders = async () => {
    setLoading(true);
    setError(null);
    try {
      const resp = await fetch(
        API_ENDPOINTS.orderList({
          status: statusFilter === "" ? undefined : statusFilter,
        })
      );
      const data = await resp.json();
      if (!resp.ok || data?.code !== 0) {
        throw new Error(data?.message || "获取挂单列表失败");
      }
      setOrders((data?.data as EntryOrder[]) || []);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setError(msg || "获取挂单列表失败");
    } finally {
      setLoading(false);
    }
  };

  const openBidDialog = (order: EntryOrder) => {
    setSelectedOrder(order);
    setBidPrice(String(order.price));
    setBidDeadlineLocal(defaultDeadlineLocal());
    setBidDialogOpen(true);
    setError(null);
    setSuccess(null);
  };

  const handleBuy = async (order: EntryOrder) => {
    if (!isConnected || !account || chainId == null || !signer) {
      setError("请先连接钱包。");
      return;
    }
    if (order.status !== 0) {
      setError("当前订单状态不可购买。");
      return;
    }
    try {
      setBuySubmittingId(order.id);
      setError(null);
      setSuccess(null);

      const marketplaceAddress = getBloomMarketplaceAddress(chainId);
      const tokenContract = getBloomTokenContract(signer, chainId);
      const marketplace = getBloomMarketplaceContract(signer, chainId);
      const isMerkle = order.isMerkle === true;
      const zeroBytes32 = "0x" + "0".repeat(64);
      if (isMerkle) {
        if (!order.merkleRoot || !order.rootDeadlineSec || !order.merkleProof) {
          throw new Error("Merkle 订单缺少 merkle 参数（merkleRoot/rootDeadlineSec/merkleProof）。");
        }
      }
      // 当前合约 ABI 中没有 listingOriginalPrice/effectiveListingPrice，
      // 用订单当前 price 作为 listing.price 来构造 Listing，确保签名验签使用同一组字段。
      const origWei: bigint = parseUnits(String(order.price), 18);
      const payWei: bigint = origWei;
      const allowance: bigint = await tokenContract.allowance(account, marketplaceAddress);
      if (allowance < payWei) {
        const approveTx = await tokenContract.approve(marketplaceAddress, payWei);
        await approveTx.wait();
      }

      const listing = {
        nft: getBloomNFTAddress(chainId),
        seller: order.seller,
        tokenId: BigInt(order.tokenId),
        price: origWei,
        deadline: BigInt(Math.floor(new Date(order.deadline).getTime() / 1000)),
        salt: BigInt(order.salt || "0"),
      };

      const normalizeSig = (s: string) =>
        s.startsWith("0x") ? s : `0x${s}`;
      const listingSig = isMerkle ? "0x" : normalizeSig(order.signature);

      const proof = isMerkle
        ? (order.merkleProof || []).map((p) =>
            p.startsWith("0x") ? p : `0x${p}`
          )
        : [];
      const merkleRoot = isMerkle
        ? order.merkleRoot!.startsWith("0x")
          ? order.merkleRoot!
          : `0x${order.merkleRoot!}`
        : zeroBytes32;
      const rootDeadline = isMerkle ? BigInt(order.rootDeadlineSec || 0) : 0n;
      const batchSig = isMerkle ? normalizeSig(order.signature) : "0x";
      // ABI：buy(Listing, listingSignature, proof, merkleRoot, rootDeadline, batchSignature)
      const tx = await marketplace.buy(
        listing,
        listingSig,
        proof,
        merkleRoot,
        rootDeadline,
        batchSig
      );
      await tx.wait();
      setSuccess("购买交易已提交并确认。");
      await fetchOrders();
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setError(msg || "购买失败");
    } finally {
      setBuySubmittingId(null);
    }
  };

  const toggleCart = (orderId: number) => {
    setCartIds((prev) => {
      const n = new Set(prev);
      if (n.has(orderId)) n.delete(orderId);
      else n.add(orderId);
      return n;
    });
  };

  const handleBuyBatch = async () => {
    if (!isConnected || !account || chainId == null || !signer) {
      setError("请先连接钱包。");
      return;
    }
    const selected = orders.filter((o) => cartIds.has(o.id) && o.status === 0);
    if (selected.length < 2) {
      setError("请至少勾选 2 个「进行中」订单用于批量购买。");
      return;
    }
    setBuyBatchSubmitting(true);
    setError(null);
    setSuccess(null);
    try {
      const marketplaceAddress = getBloomMarketplaceAddress(chainId);
      const tokenContract = getBloomTokenContract(signer, chainId);
      const marketplace = getBloomMarketplaceContract(signer, chainId);
      const nftAddr = getBloomNFTAddress(chainId);

      const zeroBytes32 = "0x" + "0".repeat(64);
      const normalizeSig = (s: string) => (s.startsWith("0x") ? s : `0x${s}`);

      const listings: Array<{
        nft: string;
        seller: string;
        tokenId: bigint;
        price: bigint;
        deadline: bigint;
        salt: bigint;
      }> = [];
      const listingSignatures: string[] = [];
      const proofs: string[][] = [];
      const merkleRoots: string[] = [];
      const rootDeadlines: bigint[] = [];
      const batchSignatures: string[] = [];

      let totalPay = 0n;
      for (const o of selected) {
        const priceWei = parseUnits(String(o.price), 18);
        totalPay += priceWei;
        listings.push({
          nft: nftAddr,
          seller: o.seller,
          tokenId: BigInt(o.tokenId),
          price: priceWei,
          deadline: BigInt(Math.floor(new Date(o.deadline).getTime() / 1000)),
          salt: BigInt(o.salt || "0"),
        });

        const isMerkle = o.isMerkle === true;
        if (isMerkle) {
          if (!o.merkleRoot || o.rootDeadlineSec == null || !o.merkleProof) {
            throw new Error(
              "Merkle 订单缺少 merkleRoot/rootDeadlineSec/merkleProof，无法批量购买。"
            );
          }
          listingSignatures.push("0x");
          proofs.push(
            o.merkleProof.map((p) => (p.startsWith("0x") ? p : `0x${p}`))
          );
          merkleRoots.push(
            o.merkleRoot.startsWith("0x") ? o.merkleRoot : `0x${o.merkleRoot}`
          );
          rootDeadlines.push(BigInt(o.rootDeadlineSec));
          batchSignatures.push(normalizeSig(o.signature));
        } else {
          listingSignatures.push(normalizeSig(o.signature));
          proofs.push([]);
          merkleRoots.push(zeroBytes32);
          rootDeadlines.push(0n);
          batchSignatures.push("0x");
        }
      }

      const allowance: bigint = await tokenContract.allowance(account, marketplaceAddress);
      if (allowance < totalPay) {
        const approveTx = await tokenContract.approve(marketplaceAddress, totalPay);
        await approveTx.wait();
      }

      const tx = await marketplace.buyBatch(
        listings,
        listingSignatures,
        proofs,
        merkleRoots,
        rootDeadlines,
        batchSignatures
      );
      await tx.wait();
      setSuccess(`批量购买已确认（${selected.length} 笔）。`);
      setCartIds(new Set());
      await fetchOrders();
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setError(msg || "批量购买失败");
    } finally {
      setBuyBatchSubmitting(false);
    }
  };

  const handleSubmitBid = async () => {
    if (!selectedOrder) return;
    if (!isConnected || !account || chainId == null || !signer) {
      setError("请先连接钱包。");
      return;
    }
    const priceNum = Number(bidPrice);
    if (!Number.isFinite(priceNum) || priceNum <= 0) {
      setError("出价必须大于 0。");
      return;
    }
    if (!bidDeadlineLocal) {
      setError("请选择出价截止时间。");
      return;
    }
    try {
      setBidSubmittingId(selectedOrder.id);
      setError(null);
      setSuccess(null);

      const marketplaceAddress = getBloomMarketplaceAddress(chainId);
      const deadlineMs = new Date(bidDeadlineLocal).getTime();
      const deadlineSec = Math.floor(deadlineMs / 1000);
      const deadlineIso = new Date(deadlineSec * 1000).toISOString();
      const priceWei = parseUnits(String(priceNum), 18);

      // bidWithSig 会 transferFrom(买家 -> 市场)，必须先授权 BloomToken 给市场合约（与「购买」一致）
      const tokenContract = getBloomTokenContract(signer, chainId);
      const allowance: bigint = await tokenContract.allowance(account, marketplaceAddress);
      if (allowance < priceWei) {
        const approveTx = await tokenContract.approve(marketplaceAddress, priceWei);
        await approveTx.wait();
      }

      // Bid 需要 salt 用于唯一标识 bid（合约层面进入 EIP-712 计算）。
      const bidSaltBig = BigInt(hexlify(randomBytes(32)));
      const signature = await signer.signTypedData(
        {
          name: "BloomMarketplace",
          version: "1",
          chainId,
          verifyingContract: marketplaceAddress,
        },
        {
          Bid: [
            { name: "nft", type: "address" },
            { name: "buyer", type: "address" },
            { name: "tokenId", type: "uint256" },
            { name: "price", type: "uint256" },
            { name: "deadline", type: "uint256" },
            { name: "salt", type: "uint256" },
          ],
        },
        {
          nft: getBloomNFTAddress(chainId),
          buyer: account,
          tokenId: BigInt(selectedOrder.tokenId),
          price: priceWei,
          deadline: BigInt(deadlineSec),
          salt: bidSaltBig,
        }
      );

      const resp = await fetch(API_ENDPOINTS.bidPlaced, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          ordersId: selectedOrder.id,
          buyer: account,
          price: priceNum,
          deadline: deadlineIso,
          salt: bidSaltBig.toString(),
          signature,
        }),
      });
      const data = await resp.json();
      if (!resp.ok || data?.code !== 0) {
        throw new Error(data?.message || "出价失败");
      }
      setSuccess("出价已提交。");
      setBidDialogOpen(false);
      await fetchOrders();
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setError(msg || "出价失败");
    } finally {
      setBidSubmittingId(null);
    }
  };

  // 新版：不再托管代币，因此不需要“取回托管/退款”相关逻辑。

  const handleCancelBid = async (bid: BidItem) => {
    if (!detailOrder || !signer || chainId == null || !account) {
      setError("请先连接钱包。");
      return;
    }
    if (detailOrder.status !== 0) {
      setError("当前挂单不在进行中状态，无法撤回出价。");
      return;
    }
    if (bid.buyer.toLowerCase() !== account.toLowerCase()) {
      setError("只能由出价者本人撤回出价。");
      return;
    }
    const ok = await requestConfirm({
      title: "确认撤回出价",
      description: `将撤回该笔出价并使其在链上失效（不涉及托管退款）。`,
    });
    if (!ok) return;

    setCancelBidId(bid.id);
    setError(null);
    setSuccess(null);
    try {
      const mp = getBloomMarketplaceContract(signer, chainId);
      const bidStruct = {
        nft: getBloomNFTAddress(chainId),
        buyer: bid.buyer,
        tokenId: BigInt(detailOrder.tokenId),
        price: parseUnits(String(bid.price), 18),
        deadline: BigInt(Math.floor(new Date(bid.deadline).getTime() / 1000)),
        salt: BigInt(bid.salt),
      };

      const tx = await mp.cancelBidOrder(bidStruct);
      await tx.wait();
      setSuccess("撤回出价已确认，托管款将退回。");
      await openOrderDetail(detailOrder);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setError(msg || "撤回出价失败");
    } finally {
      setCancelBidId(null);
    }
  };

  const openOrderDetail = async (order: EntryOrder) => {
    setDetailOrder(order);
    setBidListLoading(true);
    setError(null);
    try {
      const resp = await fetch(API_ENDPOINTS.bidList(order.id));
      const data = await resp.json();
      if (!resp.ok || data?.code !== 0) {
        throw new Error(data?.message || "获取出价列表失败");
      }
      setBidList((data?.data as BidItem[]) || []);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setError(msg || "获取出价列表失败");
      setBidList([]);
    } finally {
      setBidListLoading(false);
    }
  };

  useEffect(() => {
    void fetchOrders();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [statusFilter]);

  return (
    <Box>
      <Typography variant="h4" sx={{ mb: 3, fontWeight: 700 }}>
        市场挂单列表
      </Typography>

      {!detailOrder && (
        <Stack direction="row" sx={{ mb: 2 }}>
          <FormControl size="small" sx={{ minWidth: 220 }}>
            <InputLabel id="market-status-filter-label">状态筛选</InputLabel>
            <Select
              labelId="market-status-filter-label"
              label="状态筛选"
              value={statusFilter}
              onChange={(e) => {
                const v = e.target.value as string | number;
                setStatusFilter(typeof v === "string" && v === "" ? "" : Number(v));
              }}
            >
              <MenuItem value="">全部</MenuItem>
              <MenuItem value={0}>准备中</MenuItem>
              <MenuItem value={1}>进行中</MenuItem>
              <MenuItem value={2}>已成交</MenuItem>
              <MenuItem value={3}>已过期</MenuItem>
              <MenuItem value={4}>已取消</MenuItem>
            </Select>
          </FormControl>
        </Stack>
      )}

      {loading && (
        <Box sx={{ display: "flex", justifyContent: "center", py: 4 }}>
          <CircularProgress />
        </Box>
      )}

      {error && <Alert severity="error">{error}</Alert>}
      {success && <Alert severity="success" sx={{ mt: 1 }}>{success}</Alert>}

      {!loading && !error && orders.length === 0 && (
        <Typography color="text.secondary">暂无挂单数据</Typography>
      )}

      {!loading && !error && orders.length > 0 && !detailOrder && (
        <TableContainer component={Paper} variant="outlined">
          <Stack direction="row" justifyContent="flex-end" sx={{ mb: 1 }}>
            <Button
              variant="contained"
              color="secondary"
              disabled={buyBatchSubmitting || cartIds.size < 2}
              onClick={() => void handleBuyBatch()}
            >
              {buyBatchSubmitting ? "批量购买中…" : `购物车批量购买 (${cartIds.size})`}
            </Button>
          </Stack>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell padding="checkbox" width={48}>
                  购物车
                </TableCell>
                <TableCell>NFT</TableCell>
                <TableCell>价格</TableCell>
                <TableCell>截止时间</TableCell>
                <TableCell>持有者(卖家)</TableCell>
                <TableCell>状态</TableCell>
                <TableCell align="right">操作</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {orders.map((o) => (
                <TableRow key={o.id} hover>
                  <TableCell padding="checkbox">
                    <Checkbox
                      size="small"
                      checked={cartIds.has(o.id)}
                      disabled={o.status !== 0}
                      onChange={() => toggleCart(o.id)}
                      inputProps={{ "aria-label": `cart ${o.id}` }}
                    />
                  </TableCell>
                  <TableCell>
                    <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
                      <Box
                        component="img"
                        src={o.imageUrl}
                        alt={`NFT ${o.tokenId}`}
                        sx={{ width: 56, height: 56, objectFit: "cover", borderRadius: 1 }}
                      />
                      <Typography variant="body2">Token #{o.tokenId}</Typography>
                    </Box>
                  </TableCell>
                  <TableCell>{o.price} BT</TableCell>
                  <TableCell>{formatDateTime(o.deadline)}</TableCell>
                  <TableCell>
                    <Typography variant="body2" sx={{ maxWidth: 180 }} noWrap title={o.seller}>
                      {o.seller}
                    </Typography>
                  </TableCell>
                  <TableCell>{o.statusDesc || String(o.status)}</TableCell>
                  <TableCell align="right">
                    <Stack direction="row" spacing={1} justifyContent="flex-end">
                      <Button
                        variant="contained"
                        size="small"
                        disabled={buySubmittingId === o.id || o.status !== 0}
                        onClick={() => void handleBuy(o)}
                      >
                        {buySubmittingId === o.id ? "购买中..." : "购买"}
                      </Button>
                      <Button
                        variant="outlined"
                        size="small"
                        disabled={o.status !== 0}
                        onClick={() => openBidDialog(o)}
                      >
                        出价
                      </Button>
                      <Button size="small" onClick={() => void openOrderDetail(o)}>
                        详情
                      </Button>
                    </Stack>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}

      {detailOrder && (
        <Box>
          <Breadcrumbs sx={{ mb: 2 }}>
            <Link
              component="button"
              underline="hover"
              color="inherit"
              onClick={() => setDetailOrder(null)}
            >
              市场列表
            </Link>
            <Typography color="text.primary">订单详情 #{detailOrder.id}</Typography>
          </Breadcrumbs>
          <Card sx={{ mb: 2 }}>
            <CardContent>
              <Box sx={{ display: "flex", gap: 2 }}>
                <Box
                  component="img"
                  src={detailOrder.imageUrl}
                  alt={`NFT ${detailOrder.tokenId}`}
                  sx={{ width: 140, height: 140, objectFit: "cover", borderRadius: 1 }}
                />
                <Stack spacing={0.8}>
                  <Typography variant="h6">Token #{detailOrder.tokenId}</Typography>
                  <Typography variant="body2">价格：{detailOrder.price} BT</Typography>
                  <Typography variant="body2">截止：{formatDateTime(detailOrder.deadline)}</Typography>
                  <Typography variant="body2">状态：{detailOrder.statusDesc || String(detailOrder.status)}</Typography>
                </Stack>
              </Box>
            </CardContent>
          </Card>
          <Typography variant="h6" sx={{ mb: 1 }}>
            出价列表
          </Typography>
          {bidListLoading ? (
            <Box sx={{ display: "flex", justifyContent: "center", py: 2 }}>
              <CircularProgress size={24} />
            </Box>
          ) : bidList.length === 0 ? (
            <Typography color="text.secondary">暂无出价</Typography>
          ) : (
            <TableContainer component={Paper} variant="outlined">
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>买家</TableCell>
                    <TableCell>出价</TableCell>
                    <TableCell>截止时间</TableCell>
                    <TableCell>状态</TableCell>
                    <TableCell align="right">操作</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {bidList.map((b) => {
                    // 仅「进行中」且由出价者本人可撤回出价（使其在链上失效）。
                    const canCancel =
                      detailOrder &&
                      detailOrder.status === 0 &&
                      b.status === 0 &&
                      account &&
                      b.buyer.toLowerCase() === account.toLowerCase();
                    return (
                    <TableRow key={b.id}>
                      <TableCell>{b.buyer}</TableCell>
                      <TableCell>{b.price} BT</TableCell>
                      <TableCell>{formatDateTime(b.deadline)}</TableCell>
                      <TableCell>{bidStatusText(b.status)}</TableCell>
                      <TableCell align="right">
                        {canCancel && (
                          <Button
                            size="small"
                            variant="contained"
                            color="secondary"
                            disabled={cancelBidId === b.id}
                            onClick={() => void handleCancelBid(b)}
                          >
                            {cancelBidId === b.id ? "处理中..." : "撤回出价"}
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </TableContainer>
          )}
        </Box>
      )}

      <Dialog open={bidDialogOpen} onClose={() => setBidDialogOpen(false)} maxWidth="xs" fullWidth>
        <DialogTitle>提交出价</DialogTitle>
        <DialogContent sx={{ py: 1 }}>
          <Stack spacing={1.5} sx={{ mt: 1 }}>
            <TextField
              label="出价（BT）"
              type="number"
              inputProps={{ min: 0, step: "0.0001" }}
              value={bidPrice}
              fullWidth
              size="small"
              onChange={(e) => setBidPrice(e.target.value)}
            />
            <TextField
              label="截止时间（deadline）"
              type="datetime-local"
              value={bidDeadlineLocal}
              fullWidth
              InputLabelProps={{ shrink: true }}
              size="small"
              onChange={(e) => setBidDeadlineLocal(e.target.value)}
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setBidDialogOpen(false)} disabled={bidSubmittingId != null}>
            取消
          </Button>
          <Button
            variant="contained"
            disabled={bidSubmittingId != null}
            onClick={() => void handleSubmitBid()}
          >
            {bidSubmittingId != null ? "提交中..." : "确认出价"}
          </Button>
        </DialogActions>
      </Dialog>
      {confirmDialog}
    </Box>
  );
}

