import {
  Alert,
  Box,
  Card,
  CardContent,
  Chip,
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
} from "@mui/material";
import { parseUnits, TypedDataEncoder } from "ethers";
import { useWeb3 } from "../web3/provider";
import { useEffect, useState } from "react";
import { API_ENDPOINTS } from "../config/api";
import {
  getBloomNFTContract,
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
  status?: number;
  statusDesc?: string;
}

interface EntryOrderItem {
  id: number;
  nftListId: number;
  seller: string;
  tokenId: number;
  price: number;
  deadline: string;
  nonce: number;
  status: number;
  statusDesc?: string;
}

export function Profile() {
  const { account, isConnected, chainId, signer } = useWeb3();
  const [loading, setLoading] = useState(false);
  const [categories, setCategories] = useState<NftCategory[]>([]);
  const [selectedCategoryId, setSelectedCategoryId] = useState<number | null>(
    null
  );
  const [nftList, setNftList] = useState<NftItem[]>([]);
  const [orderByNftListId, setOrderByNftListId] = useState<Record<number, EntryOrderItem>>({});
  const [error, setError] = useState<string | null>(null);

  // --- Entry order dialog state ---
  const [orderDialogOpen, setOrderDialogOpen] = useState(false);
  const [selectedNft, setSelectedNft] = useState<NftItem | null>(null);
  const [orderSubmitting, setOrderSubmitting] = useState(false);
  const [cancelSubmittingId, setCancelSubmittingId] = useState<number | null>(null);
  const [orderError, setOrderError] = useState<string | null>(null);
  const [orderSuccess, setOrderSuccess] = useState<string | null>(null);

  const [price, setPrice] = useState("1");
  const [deadlineLocal, setDeadlineLocal] = useState("");

  const pad2 = (n: number) => String(n).padStart(2, "0");
  const toDateTimeLocal = (d: Date) => {
    // datetime-local expects local time: YYYY-MM-DDTHH:mm
    return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}T${pad2(
      d.getHours()
    )}:${pad2(d.getMinutes())}`;
  };
  const defaultDeadline = () => {
    const d = new Date();
    d.setDate(d.getDate() + 1); // 默认 +1 天
    return toDateTimeLocal(d);
  };

  const openEntryOrderDialog = (nft: NftItem) => {
    setSelectedNft(nft);
    setPrice("1");
    setDeadlineLocal(defaultDeadline());
    setOrderError(null);
    setOrderSuccess(null);
    setOrderDialogOpen(true);
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

    // EIP-712 签名：Listing(address nft,address seller,uint256 tokenId,uint256 price,uint256 deadline,uint256 nonce)
    const nftAddress = getBloomNFTAddress(chainId);
    const marketplaceAddress = getBloomMarketplaceAddress(chainId);
    const deadlineSeconds = Math.floor(new Date(deadlineIso).getTime() / 1000);
    let nonceNum: number;

    // 挂单前确保 marketplace 已被 NFT 合约授权（一次授权可复用）。
    try {
      const nftContract = getBloomNFTContract(signer, chainId);
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

    // 签名前实时读取链上 nonce，避免用户手动输入导致 bad nonce
    try {
      const marketplaceContract = getBloomMarketplaceContract(signer, chainId);
      const onChainNonce = await marketplaceContract.nonces(account);
      nonceNum = Number(onChainNonce);
      if (!Number.isFinite(nonceNum) || nonceNum < 0 || !Number.isInteger(nonceNum)) {
        setOrderError("读取链上 nonce 失败，请稍后重试。");
        return;
      }
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setOrderError(msg || "读取链上 nonce 失败，请稍后重试。");
      return;
    }

    const types: Record<string, Array<{ name: string; type: string }>> = {
      Listing: [
        { name: "nft", type: "address" },
        { name: "seller", type: "address" },
        { name: "tokenId", type: "uint256" },
        { name: "price", type: "uint256" },
        { name: "deadline", type: "uint256" },
        { name: "nonce", type: "uint256" },
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
      nonce: BigInt(nonceNum),
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
      nonce: nonceNum,
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

    const order = orderByNftListId[nft.id];
    if (!order) {
      setError("未找到对应挂单信息，无法取消上架。");
      return;
    }
    if (order.status !== 1) {
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
      const listing = {
        nft: getBloomNFTAddress(chainId),
        seller: order.seller,
        tokenId: BigInt(order.tokenId),
        price: parseUnits(String(order.price), 18),
        deadline: BigInt(Math.floor(new Date(order.deadline).getTime() / 1000)),
        nonce: BigInt(order.nonce),
      };

      const tx = await marketplace.cancelListing(listing);
      await tx.wait();

      // 等后端监听链上事件后，刷新数据展示最新状态
      if (selectedCategoryId != null) {
        const [nftResp, orderResp] = await Promise.all([
          fetch(`${API_ENDPOINTS.nftUserListByCategory(selectedCategoryId)}?owner=${account}`),
          fetch(API_ENDPOINTS.orderList()),
        ]);
        const nftData = await nftResp.json();
        const orderData = await orderResp.json();
        if (nftResp.ok && nftData?.code === 0) {
          setNftList((nftData.data as NftItem[]) || []);
        }
        if (orderResp.ok && orderData?.code === 0) {
          const orders = (orderData.data as EntryOrderItem[]) || [];
          const map: Record<number, EntryOrderItem> = {};
          for (const o of orders) {
            map[o.nftListId] = o;
          }
          setOrderByNftListId(map);
        }
      }
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setError(msg || "取消上架失败");
    } finally {
      setCancelSubmittingId(null);
    }
  };

  useEffect(() => {
    if (!isConnected || !account) {
      setCategories([]);
      setNftList([]);
      setSelectedCategoryId(null);
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
        if (list.length > 0) {
          setSelectedCategoryId(list[0].id);
        } else {
          setSelectedCategoryId(null);
          setNftList([]);
        }
      } catch (e: unknown) {
        const msg = e instanceof Error ? e.message : undefined;
        setError(msg || "获取 NFT 类目失败");
      } finally {
        setLoading(false);
      }
    };

    fetchCategories();
  }, [isConnected, account]);

  useEffect(() => {
    if (!isConnected || !account || selectedCategoryId == null) {
      setNftList([]);
      setOrderByNftListId({});
      return;
    }

    const fetchNftList = async () => {
      try {
        setLoading(true);
        setError(null);
        const [nftResp, orderResp] = await Promise.all([
          fetch(`${API_ENDPOINTS.nftUserListByCategory(selectedCategoryId)}?owner=${account}`),
          fetch(API_ENDPOINTS.orderList()),
        ]);
        const nftData = await nftResp.json();
        const orderData = await orderResp.json();
        if (!nftResp.ok || nftData.code !== 0) {
          throw new Error(nftData.message || "获取 NFT 列表失败");
        }
        const list: NftItem[] = nftData.data || [];
        setNftList(list);

        if (!orderResp.ok || orderData.code !== 0) {
          throw new Error(orderData.message || "获取挂单列表失败");
        }
        const orders: EntryOrderItem[] = orderData.data || [];
        const map: Record<number, EntryOrderItem> = {};
        for (const o of orders) {
          map[o.nftListId] = o;
        }
        setOrderByNftListId(map);
      } catch (e: unknown) {
        const msg = e instanceof Error ? e.message : undefined;
        setError(msg || "获取 NFT 列表失败");
      } finally {
        setLoading(false);
      }
    };

    fetchNftList();
  }, [isConnected, account, selectedCategoryId]);

  return (
    <>
      <Typography variant="h4" sx={{ mb: 3, fontWeight: 700 }}>
        个人中心
      </Typography>
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
              <Box sx={{ mb: 3, display: "flex", gap: 1, flexWrap: "wrap" }}>
                {categories.map((cat) => (
                  <Chip
                    key={cat.id}
                    label={cat.name}
                    color={cat.id === selectedCategoryId ? "primary" : "default"}
                    onClick={() => setSelectedCategoryId(cat.id)}
                    variant={
                      cat.id === selectedCategoryId ? "filled" : "outlined"
                    }
                  />
                ))}
              </Box>

              <Box
                sx={{
                  display: "grid",
                  gridTemplateColumns: {
                    xs: "repeat(auto-fill, minmax(180px, 1fr))",
                    sm: "repeat(auto-fill, minmax(210px, 1fr))",
                  },
                  gap: 2,
                  alignItems: "start",
                }}
              >
                {nftList.map((nft) => (
                  (() => {
                    const order = orderByNftListId[nft.id];
                    const status = order?.status ?? nft.status;
                    const statusDesc =
                      order?.statusDesc || nft.statusDesc || (status != null ? String(status) : "-");
                    const canCancel = status === 1 && !!order;
                    return (
                  <Card
                    key={nft.id}
                    sx={{
                      width: "100%",
                      maxWidth: 210,
                      minHeight: 320,
                      display: "flex",
                      flexDirection: "column",
                      borderRadius: 2,
                      border: "1px solid",
                      borderColor: "divider",
                      boxShadow: "0 2px 10px rgba(0,0,0,0.04)",
                    }}
                  >
                    <Box
                      sx={{
                        width: "100%",
                        height: 210,
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                        p: 1.5,
                        background:
                          "linear-gradient(180deg, rgba(255,255,255,0.05) 0%, rgba(255,255,255,0.02) 100%)",
                        borderTopLeftRadius: 2,
                        borderTopRightRadius: 2,
                      }}
                    >
                      <Box
                        component="img"
                        src={nft.imageUrl}
                        alt={nft.name}
                        sx={{
                          width: "100%",
                          height: "100%",
                          objectFit: "contain",
                          borderRadius: 1,
                        }}
                      />
                    </Box>
                    <CardContent sx={{ flexGrow: 1, display: "flex", flexDirection: "column", gap: 1, pt: 1.5 }}>
                      <Typography
                        variant="h6"
                        sx={{ fontWeight: 700, mb: 0.5 }}
                      >
                        {nft.name}
                      </Typography>
                      <Divider />
                      <Stack direction="row" justifyContent="space-between" alignItems="center">
                        <Typography variant="caption" color="text.secondary">
                          Token ID: {nft.tokenId}
                        </Typography>
                        <Chip
                          label={statusDesc}
                          size="small"
                          color={status === 1 ? "primary" : status === 2 ? "success" : "default"}
                          variant={status === 1 || status === 2 ? "filled" : "outlined"}
                        />
                      </Stack>

                      <Box sx={{ mt: "auto", display: "flex", gap: 1 }}>
                        <Button
                          variant="contained"
                          size="small"
                          disabled={!isConnected}
                          onClick={() => openEntryOrderDialog(nft)}
                          fullWidth={!canCancel}
                          sx={{ borderRadius: 999, fontWeight: 700 }}
                        >
                          挂单
                        </Button>
                        {canCancel && (
                          <Button
                            variant="outlined"
                            color="warning"
                            size="small"
                            disabled={cancelSubmittingId === nft.id}
                            onClick={() => void handleCancelListing(nft)}
                            sx={{ borderRadius: 999, fontWeight: 700, px: 1.5 }}
                          >
                            {cancelSubmittingId === nft.id ? "取消中..." : "取消上架"}
                          </Button>
                        )}
                      </Box>
                    </CardContent>
                  </Card>
                    );
                  })()
                ))}
              </Box>
            </>
          )}
        </>
      )}

      <Dialog open={orderDialogOpen} onClose={() => setOrderDialogOpen(false)} maxWidth="xs" fullWidth>
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
              size="small"
              onChange={(e) => setDeadlineLocal(e.target.value)}
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
    </>
  );
}

