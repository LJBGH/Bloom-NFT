import { Alert, Box, Button, Divider, Stack, TextField, Typography } from "@mui/material";
import { formatUnits } from "ethers";
import { useCallback, useEffect, useState } from "react";
import {
  getBloomMarketplaceAddress,
  getBloomMarketplaceContract,
  getBloomNFTContract,
  getBloomTokenContract,
} from "../web3/contracts";
import { useWeb3 } from "../web3/provider";

export function TokenOwnerTool() {
  const { account, chainId, signer, provider } = useWeb3();
  const [tokenId, setTokenId] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [owner, setOwner] = useState<string | null>(null);

  const [feeLoading, setFeeLoading] = useState(false);
  const [feeError, setFeeError] = useState<string | null>(null);
  const [feeSuccess, setFeeSuccess] = useState<string | null>(null);
  const [marketplaceOwner, setMarketplaceOwner] = useState<string | null>(null);
  const [mpBtBalance, setMpBtBalance] = useState<string | null>(null);
  const [bidEscrowTotal, setBidEscrowTotal] = useState<string | null>(null);
  const [withdrawableFees, setWithdrawableFees] = useState<string | null>(null);
  /** 可提手续费 wei，用于判断是否大于 0 */
  const [withdrawableWei, setWithdrawableWei] = useState<bigint | null>(null);

  const handleQuery = async () => {
    if (chainId == null) {
      setError("无法获取当前网络(chainId)。请先连接钱包。");
      return;
    }
    const tokenIdNum = Number(tokenId);
    if (!Number.isFinite(tokenIdNum) || tokenIdNum < 0 || !Number.isInteger(tokenIdNum)) {
      setError("请输入合法的 tokenId（非负整数）。");
      return;
    }
    const reader = signer ?? provider;
    if (!reader) {
      setError("无法读取链上数据，请先连接钱包。");
      return;
    }

    try {
      setLoading(true);
      setError(null);
      setOwner(null);
      const nftContract = getBloomNFTContract(reader, chainId);
      const result: string = await nftContract.ownerOf(BigInt(tokenIdNum));
      setOwner(result);
    } catch (e: unknown) {
      const err = e as { reason?: string; message?: string } | undefined;
      setError(err?.reason || err?.message || "查询失败，请确认 tokenId 是否存在。");
    } finally {
      setLoading(false);
    }
  };

  const loadMarketplaceFees = useCallback(async () => {
    if (chainId == null) {
      setMarketplaceOwner(null);
      setMpBtBalance(null);
      setBidEscrowTotal(null);
      setWithdrawableFees(null);
      setWithdrawableWei(null);
      return;
    }
    const reader = signer ?? provider;
    if (!reader) {
      setMarketplaceOwner(null);
      setMpBtBalance(null);
      setBidEscrowTotal(null);
      setWithdrawableFees(null);
      setWithdrawableWei(null);
      return;
    }
    try {
      setFeeLoading(true);
      setFeeError(null);
      const mp = getBloomMarketplaceContract(reader, chainId);
      const mpAddr = getBloomMarketplaceAddress(chainId);
      const token = getBloomTokenContract(reader, chainId);
      const [own, bal, escrow] = await Promise.all([
        mp.owner() as Promise<string>,
        token.balanceOf(mpAddr) as Promise<bigint>,
        mp.totalBidEscrow() as Promise<bigint>,
      ]);
      setMarketplaceOwner(own);
      setMpBtBalance(formatUnits(bal, 18));
      setBidEscrowTotal(formatUnits(escrow, 18));
      const w = bal - escrow;
      const ww = w > 0n ? w : 0n;
      setWithdrawableWei(ww);
      setWithdrawableFees(formatUnits(ww, 18));
    } catch (e: unknown) {
      const err = e as { message?: string };
      setFeeError(err?.message || "读取市场合约数据失败");
      setMarketplaceOwner(null);
      setMpBtBalance(null);
      setBidEscrowTotal(null);
      setWithdrawableFees(null);
      setWithdrawableWei(null);
    } finally {
      setFeeLoading(false);
    }
  }, [chainId, signer, provider]);

  useEffect(() => {
    void loadMarketplaceFees();
  }, [loadMarketplaceFees]);

  const handleWithdrawFees = async () => {
    if (chainId == null || !signer) {
      setFeeError("请先连接钱包。");
      return;
    }
    const mp = getBloomMarketplaceContract(signer, chainId);
    try {
      setFeeError(null);
      setFeeSuccess(null);
      setFeeLoading(true);
      const tx = await mp.withdrawFees();
      await tx.wait();
      setFeeSuccess("手续费已提取到合约 owner 地址。");
      await loadMarketplaceFees();
    } catch (e: unknown) {
      const err = e as { reason?: string; message?: string };
      setFeeError(err?.reason || err?.message || "提取失败");
    } finally {
      setFeeLoading(false);
    }
  };

  const isMarketplaceOwner =
    account &&
    marketplaceOwner &&
    account.toLowerCase() === marketplaceOwner.toLowerCase();

  return (
    <Box>
      <Typography variant="h4" sx={{ mb: 3, fontWeight: 700 }}>
        工具
      </Typography>

      <Typography variant="h6" sx={{ mb: 2, fontWeight: 600 }}>
        市场手续费（BT）
      </Typography>
      <Stack spacing={2} maxWidth={560} sx={{ mb: 4 }}>
        {feeError && (
          <Alert severity="error" onClose={() => setFeeError(null)}>
            {feeError}
          </Alert>
        )}
        {feeSuccess && (
          <Alert severity="success" onClose={() => setFeeSuccess(null)}>
            {feeSuccess}
          </Alert>
        )}
        <Typography variant="body2" color="text.secondary">
          从 BloomMarketplace 合约提取累计交易手续费（BT）。合约内余额需扣除买家出价托管部分后，剩余方可提取；仅合约 owner 可调用链上{" "}
          <code>withdrawFees</code>。
        </Typography>
        {feeLoading && !mpBtBalance ? (
          <Typography color="text.secondary">加载中…</Typography>
        ) : (
          <>
            <Typography variant="body2">
              合约 owner：<wbr />
              {marketplaceOwner ?? "—"}
            </Typography>
            <Typography variant="body2">
              市场合约 BT 余额：{mpBtBalance ?? "—"} BT
            </Typography>
            <Typography variant="body2">
              其中托管（出价锁定）：{bidEscrowTotal ?? "—"} BT
            </Typography>
            <Typography variant="body2" sx={{ fontWeight: 600 }}>
              可提取手续费：{withdrawableFees ?? "—"} BT
            </Typography>
          </>
        )}
        <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
          <Button variant="outlined" size="small" onClick={() => void loadMarketplaceFees()} disabled={feeLoading}>
            刷新
          </Button>
          <Button
            variant="contained"
            color="secondary"
            onClick={() => void handleWithdrawFees()}
            disabled={feeLoading || !isMarketplaceOwner || !withdrawableWei || withdrawableWei === 0n}
          >
            {feeLoading ? "处理中…" : "提取全部可提手续费"}
          </Button>
        </Stack>
        {!isMarketplaceOwner && account && marketplaceOwner && (
          <Alert severity="info">当前钱包不是市场合约 owner，无法提取。</Alert>
        )}
      </Stack>

      <Divider sx={{ my: 3 }} />

      <Typography variant="h6" sx={{ mb: 2, fontWeight: 600 }}>
        Token Owner 查询
      </Typography>
      <Stack spacing={2} maxWidth={480}>
        {error && (
          <Alert severity="error" onClose={() => setError(null)}>
            {error}
          </Alert>
        )}
        {owner && <Alert severity="success">所属地址：{owner}</Alert>}

        <TextField
          label="Token ID"
          type="number"
          inputProps={{ min: 0, step: 1 }}
          value={tokenId}
          onChange={(e) => setTokenId(e.target.value)}
          fullWidth
        />

        <Button variant="contained" onClick={handleQuery} disabled={loading}>
          {loading ? "查询中..." : "查询所属地址"}
        </Button>
      </Stack>
    </Box>
  );
}

