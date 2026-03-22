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
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import { TypedDataEncoder, parseUnits } from "ethers";
import { useEffect, useState } from "react";
import { API_ENDPOINTS } from "../config/api";
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
  nonce: number;
  status: number;
  statusDesc?: string;
  signature: string;
  createTime: string;
  updateTime: string;
  imageUrl: string;
  /** 链上 listingHash，未上链同步前可能为空 */
  listingHash?: string;
}

interface BidItem {
  id: number;
  ordersId: number;
  buyer: string;
  price: number;
  deadline: string;
  nonce: number;
  status: number;
  txHash?: string;
  signature?: string;
}

function bidStatusText(status: number) {
  switch (status) {
    case 0:
      return "准备中";
    case 1:
      return "进行中";
    case 2:
      return "已成交";
    case 3:
      return "已过期";
    case 4:
      return "已取消";
    case 5:
      return "已失效";
    case 6:
      return "未中标";
    case 7:
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
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [orders, setOrders] = useState<EntryOrder[]>([]);
  const [buySubmittingId, setBuySubmittingId] = useState<number | null>(null);
  const [bidSubmittingId, setBidSubmittingId] = useState<number | null>(null);
  const [bidDialogOpen, setBidDialogOpen] = useState(false);
  const [selectedOrder, setSelectedOrder] = useState<EntryOrder | null>(null);
  const [bidPrice, setBidPrice] = useState("1");
  const [bidDeadlineLocal, setBidDeadlineLocal] = useState("");
  const [detailOrder, setDetailOrder] = useState<EntryOrder | null>(null);
  const [bidList, setBidList] = useState<BidItem[]>([]);
  const [bidListLoading, setBidListLoading] = useState(false);
  const [refundBidId, setRefundBidId] = useState<number | null>(null);

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
      const resp = await fetch(API_ENDPOINTS.orderList());
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
    if (order.status !== 1) {
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
      const amountWei = parseUnits(String(order.price), 18);
      const allowance: bigint = await tokenContract.allowance(account, marketplaceAddress);
      if (allowance < amountWei) {
        const approveTx = await tokenContract.approve(marketplaceAddress, amountWei);
        await approveTx.wait();
      }

      const listing = {
        nft: getBloomNFTAddress(chainId),
        seller: order.seller,
        tokenId: BigInt(order.tokenId),
        price: amountWei,
        deadline: BigInt(Math.floor(new Date(order.deadline).getTime() / 1000)),
        nonce: BigInt(order.nonce),
      };

      const tx = await marketplace.buy(listing, order.signature);
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

      const marketplace = getBloomMarketplaceContract(signer, chainId);
      const onChainNonce = await marketplace.nonces(account);
      const nonceNum = Number(onChainNonce);
      const listing = {
        nft: getBloomNFTAddress(chainId),
        seller: selectedOrder.seller,
        tokenId: BigInt(selectedOrder.tokenId),
        price: parseUnits(String(selectedOrder.price), 18),
        deadline: BigInt(Math.floor(new Date(selectedOrder.deadline).getTime() / 1000)),
        nonce: BigInt(selectedOrder.nonce),
      };
      const listingHash = TypedDataEncoder.hash(
        {
          name: "BloomMarketplace",
          version: "1",
          chainId,
          verifyingContract: marketplaceAddress,
        },
        {
          Listing: [
            { name: "nft", type: "address" },
            { name: "seller", type: "address" },
            { name: "tokenId", type: "uint256" },
            { name: "price", type: "uint256" },
            { name: "deadline", type: "uint256" },
            { name: "nonce", type: "uint256" },
          ],
        },
        listing
      );
      const signature = await signer.signTypedData(
        {
          name: "BloomMarketplace",
          version: "1",
          chainId,
          verifyingContract: marketplaceAddress,
        },
        {
          Bid: [
            { name: "listingHash", type: "bytes32" },
            { name: "buyer", type: "address" },
            { name: "price", type: "uint256" },
            { name: "deadline", type: "uint256" },
            { name: "nonce", type: "uint256" },
          ],
        },
        {
          listingHash,
          buyer: account,
          price: priceWei,
          deadline: BigInt(deadlineSec),
          nonce: BigInt(nonceNum),
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
          nonce: nonceNum,
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

  const handleRefundLosingBid = async (bid: BidItem) => {
    if (!detailOrder || !signer || chainId == null || !account) {
      setError("请先连接钱包。");
      return;
    }
    if (!detailOrder.listingHash) {
      setError("缺少 listingHash（挂单可能尚未完成链上同步），请稍后重试或刷新列表。");
      return;
    }
    if (!bid.signature) {
      setError("缺少出价签名数据，无法退款。");
      return;
    }
    setRefundBidId(bid.id);
    setError(null);
    setSuccess(null);
    try {
      const mp = getBloomMarketplaceContract(signer, chainId);
      const lh = detailOrder.listingHash.startsWith("0x")
        ? detailOrder.listingHash
        : `0x${detailOrder.listingHash}`;
      const bidStruct = {
        listingHash: lh,
        buyer: bid.buyer,
        price: parseUnits(String(bid.price), 18),
        deadline: BigInt(Math.floor(new Date(bid.deadline).getTime() / 1000)),
        nonce: BigInt(bid.nonce),
      };
      const sig = bid.signature.startsWith("0x") ? bid.signature : `0x${bid.signature}`;
      const tx = await mp.refundLosingBid(bidStruct, sig);
      await tx.wait();
      setSuccess("托管款已退回至钱包，数据库状态将在监听器同步后更新。");
      await openOrderDetail(detailOrder);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setError(msg || "领取退款失败");
    } finally {
      setRefundBidId(null);
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
  }, []);

  return (
    <Box>
      <Typography variant="h4" sx={{ mb: 3, fontWeight: 700 }}>
        市场挂单列表
      </Typography>

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
          <Table>
            <TableHead>
              <TableRow>
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
                        disabled={buySubmittingId === o.id || o.status !== 1}
                        onClick={() => void handleBuy(o)}
                      >
                        {buySubmittingId === o.id ? "购买中..." : "购买"}
                      </Button>
                      <Button
                        variant="outlined"
                        size="small"
                        disabled={o.status !== 1}
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
                    const canRefund =
                      b.status === 6 &&
                      account &&
                      b.buyer.toLowerCase() === account.toLowerCase();
                    return (
                    <TableRow key={b.id}>
                      <TableCell>{b.buyer}</TableCell>
                      <TableCell>{b.price} BT</TableCell>
                      <TableCell>{formatDateTime(b.deadline)}</TableCell>
                      <TableCell>{bidStatusText(b.status)}</TableCell>
                      <TableCell align="right">
                        {canRefund && (
                          <Button
                            size="small"
                            variant="contained"
                            color="secondary"
                            disabled={refundBidId === b.id}
                            onClick={() => void handleRefundLosingBid(b)}
                          >
                            {refundBidId === b.id ? "退款中..." : "领取托管退款"}
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
    </Box>
  );
}

