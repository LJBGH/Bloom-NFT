import { Alert, Box, Button, Stack, TextField, Typography } from "@mui/material";
import { useState } from "react";
import { getBloomNFTContract } from "../web3/contracts";
import { useWeb3 } from "../web3/provider";

export function TokenOwnerTool() {
  const { chainId, signer, provider } = useWeb3();
  const [tokenId, setTokenId] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [owner, setOwner] = useState<string | null>(null);

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

  return (
    <Box>
      <Typography variant="h4" sx={{ mb: 3, fontWeight: 700 }}>
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

