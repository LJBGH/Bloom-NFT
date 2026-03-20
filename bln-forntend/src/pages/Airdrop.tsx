import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Paper,
  Stack,
  Typography,
} from "@mui/material";
import { formatUnits } from "ethers";
import { useEffect, useMemo, useState } from "react";
import { useWeb3 } from "../web3/provider";
import {
  getBloomTokenAirdropContract,
  getBloomTokenContract,
  getBloomTokenAddress,
  getBloomTokenAirdropAddress,
} from "../web3/contracts";

export function Airdrop() {
  const { account, chainId, signer, provider, isConnected } = useWeb3();

  const [loading, setLoading] = useState(false);
  const [loadingInit, setLoadingInit] = useState(false);
  const [claimed, setClaimed] = useState<boolean>(false);
  const [tokenSymbol, setTokenSymbol] = useState<string>("");
  const [tokenDecimals, setTokenDecimals] = useState<number>(18);
  const [claimAmount, setClaimAmount] = useState<string>("");
  const [airdropContractBalance, setAirdropContractBalance] = useState<string>("");
  const [tokensPerClaimRaw, setTokensPerClaimRaw] = useState<bigint>(0n);
  const [airdropBalanceRaw, setAirdropBalanceRaw] = useState<bigint>(0n);
  const [error, setError] = useState<string | null>(null);

  const tokenAddress = useMemo(
    () => (chainId ? getBloomTokenAddress(chainId) : ""),
    [chainId]
  );

  const airdropAddress = useMemo(
    () => (chainId ? getBloomTokenAirdropAddress(chainId) : ""),
    [chainId]
  );

  const canClaim =
    !loadingInit &&
    isConnected &&
    !!account &&
    !claimed &&
    tokensPerClaimRaw > 0n &&
    airdropBalanceRaw >= tokensPerClaimRaw;

  useEffect(() => {
    let mounted = true;
    const load = async () => {
      if (!chainId) return;

      try {
        setLoadingInit(true);
        setError(null);

        const readTarget = signer ?? provider;
        if (!readTarget) return;

        const token = getBloomTokenContract(readTarget, chainId);
        const airdrop = getBloomTokenAirdropContract(readTarget, chainId);

        const [symbol, decimals, amount] = await Promise.all([
          token.symbol(),
          token.decimals(),
          airdrop.TOKENS_PER_CLAIM(),
        ]);

        if (!mounted) {
          return;
        }

        setTokenSymbol(String(symbol));
        setTokenDecimals(Number(decimals));
        setClaimAmount(formatUnits(amount, Number(decimals)));
        setTokensPerClaimRaw(amount);

        if (airdropAddress) {
          const contractBalance = await token.balanceOf(airdropAddress);
          if (!mounted) return;
          setAirdropBalanceRaw(contractBalance);
          setAirdropContractBalance(formatUnits(contractBalance, Number(decimals)));
        }

        if (isConnected && account) {
          const claimedRes = await airdrop.wasClaimed(account);
          if (!mounted) {
            return;
          }
          setClaimed(Boolean(claimedRes));
        } else {
          setClaimed(false);
        }
      } catch (e: unknown) {
        const msg = e instanceof Error ? e.message : undefined;
        if (mounted) {
          setError(msg || "初始化 Token 数据失败");
        }
      } finally {
        if (mounted) {
          setLoadingInit(false);
        }
      }
    };

    load();
    return () => {
      mounted = false;
    };
  }, [chainId, account, isConnected, signer, provider, airdropAddress]);

  const addTokenToWallet = async () => {
    if (!tokenAddress || !tokenSymbol) {
      setError("Token 信息未加载完成");
      return;
    }

    const win = window as unknown as {
      ethereum?: {
        request?: (args: {
          method: string;
          params?: unknown;
        }) => Promise<unknown>;
      };
    };

    const eth = win.ethereum;
    if (!eth?.request) {
      setError("未找到钱包提供的 ethereum 接口");
      return;
    }

    try {
      setError(null);

      const wasAdded = await eth.request({
        method: "wallet_watchAsset",
        params: {
          type: "ERC20",
          options: {
            address: tokenAddress,
            symbol: tokenSymbol,
            decimals: tokenDecimals,
          },
        },
      });

      if (!wasAdded) {
        setError("钱包拒绝添加代币或添加失败");
      }
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setError(msg || "添加代币到钱包失败");
    }
  };

  const handleClaim = async () => {
    if (!isConnected || !account || !chainId || !signer) {
      setError("请先连接钱包后再领取 Token");
      return;
    }
    if (claimed) {
      setError("该地址已经领取过 Token（wasClaimed=true）");
      return;
    }

    try {
      setLoading(true);
      setError(null);

      const airdrop = getBloomTokenAirdropContract(signer, chainId);

      const tx = await airdrop.withdrawTokens();
      await tx.wait();

      const nextClaimed = await airdrop.wasClaimed(account);
      setClaimed(Boolean(nextClaimed));
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : undefined;
      setError(msg || "领取 Token 失败");
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <Typography variant="h4" sx={{ mb: 3, fontWeight: 700 }}>
        BT 空投
      </Typography>
      <Typography variant="body1" sx={{ mb: 4 }}>
        从 `BloomTokenAirdrop` 合约领取 Token，并把代币添加到钱包。
      </Typography>

      <Box sx={{ display: "flex", flexDirection: { xs: "column", md: "row" }, gap: 3, mb: 4 }}>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Paper sx={{ p: 3 }}>
            <Typography variant="h6" sx={{ fontWeight: 700, mb: 2 }}>
              领取 Token
            </Typography>

            {loadingInit ? (
              <Box sx={{ display: "flex", justifyContent: "center", py: 4 }}>
                <CircularProgress />
              </Box>
            ) : (
              <Stack spacing={2}>
                <Typography variant="body2" color="text.secondary">
                  当前钱包：{account ? account : "未连接"}
                </Typography>

                <Typography variant="body2">
                  代币：{tokenSymbol || "--"}
                  {claimAmount ? `，每次领取：${claimAmount}` : ""}
                </Typography>

                <Alert severity={claimed ? "success" : "info"}>
                  {claimed ? "已领取" : "未领取"}
                </Alert>

                {isConnected && account && (
                  <Typography variant="body2" color="text.secondary">
                    剩余空投：{airdropContractBalance || "0"}{" "}
                    {tokenSymbol || "BT"}
                  </Typography>
                )}

                {error && <Alert severity="error">{error}</Alert>}

                <Button
                  variant="contained"
                  disabled={!canClaim || loading}
                  onClick={handleClaim}
                >
                  {loading ? "领取中..." : "领取Token"}
                </Button>
              </Stack>
            )}
          </Paper>
        </Box>

        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Paper sx={{ p: 3 }}>
            <Typography variant="h6" sx={{ fontWeight: 700, mb: 2 }}>
              添加代币到钱包
            </Typography>

            <Stack spacing={2}>
              <Typography variant="body2" color="text.secondary">
                Token 合约地址：{tokenAddress}
              </Typography>
              <Typography variant="body2">
                {tokenSymbol || "--"}（decimals: {tokenDecimals}）
              </Typography>

              {error && <Alert severity="error">{error}</Alert>}

              <Button
                variant="outlined"
                disabled={!tokenSymbol || loading}
                onClick={addTokenToWallet}
              >
                添加代币
              </Button>
            </Stack>
          </Paper>
        </Box>
      </Box>
    </>
  );
}

