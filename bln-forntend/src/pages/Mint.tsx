import {
  Alert,
  Box,
  Button,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import { useState } from "react";
import { useWeb3 } from "../web3/provider";
import { getBloomNFTAddress, getBloomNFTContract } from "../web3/contracts";
import { API_ENDPOINTS } from "../config/api";

export function Mint() {
  const { account, chainId, signer, isConnected } = useWeb3();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const handleMint = async () => {
    if (!signer || !account) {
      setError("请先连接钱包。");
      return;
    }
    const trimmedName = name.trim();
    const trimmedDesc = description.trim();
    if (!trimmedName) {
      setError("请输入 NFT 名称（name）。");
      return;
    }
    if (!trimmedDesc) {
      setError("请输入 NFT 描述（description）。");
      return;
    }
    if (!file) {
      setError("请上传文件（用于生成元数据 image）。");
      return;
    }

    try {
      setLoading(true);
      setError(null);
      setSuccess(null);

      // 1) 先调用后端：上传文件到 Pinata，生成 metadata tokenUrl（ipfs://...）
      const formData = new FormData();
      formData.append("name", trimmedName);
      formData.append("description", trimmedDesc);
      formData.append("file", file);

      const res = await fetch(API_ENDPOINTS.nftMint, {
        method: "POST",
        body: formData,
      });

      const json = await res.json();
      if (!res.ok || json?.code !== 0) {
        setError(json?.message || "后端生成元数据失败");
        return;
      }

      const tokenUrl: string = json?.data?.tokenUrl;
      if (!tokenUrl) {
        setError("后端返回的 tokenUrl 为空");
        return;
      }

      // 2) 再调用合约：mint NFT（tokenURI = tokenUrl）
      const currentNetwork = await signer.provider?.getNetwork();
      const currentChainId = currentNetwork ? Number(currentNetwork.chainId) : chainId;
      if (currentChainId == null) {
        setError("无法获取当前网络(chainId)。请重新连接钱包。");
        return;
      }

      const nftAddress = getBloomNFTAddress(currentChainId);
      // 先检查地址是否真的有合约代码，避免 ABI/地址不匹配时出现 value="0x"
      const code = await signer.provider?.getCode(nftAddress);
      if (!code || code === "0x") {
        setError(
          `BloomNFT 地址没有合约代码：${nftAddress}。请确认钱包网络(chainId)以及 contract-addresses.json 是否匹配。`
        );
        return;
      }

      const contract = getBloomNFTContract(signer, currentChainId);
      const price = await contract.price();

      const tx = await contract.mint(account, tokenUrl, { value: price });
      const receipt = await tx.wait();

      // 3) 从合约事件中解析 tokenId 和 owner，并回写到后端
      try {
        // BloomNFT 中有 Mint(sender, tokenId, url) 事件
        const mintEvent = receipt?.logs
          .map((log: unknown) => {
            try {
              return contract.interface.parseLog(
                log as Parameters<typeof contract.interface.parseLog>[0]
              );
            } catch {
              return null;
            }
          })
          .find((parsed: unknown) => {
            if (!parsed) return false;
            if (typeof parsed === "object" && "name" in parsed) {
              return (parsed as { name?: string }).name === "Mint";
            }
            return false;
          });

        if (mintEvent) {
          const tokenId = mintEvent.args.tokenId as bigint;
          const sender = mintEvent.args.sender as string;

          const updateForm = new FormData();
          updateForm.append("tokenUrl", tokenUrl);
          updateForm.append("tokenId", tokenId.toString());
          updateForm.append("owner", sender);

          const updateRes = await fetch(API_ENDPOINTS.nftUpdate, {
            method: "POST",
            body: updateForm,
          });
          const updateJson = await updateRes.json();
          if (!updateRes.ok || updateJson?.code !== 0) {
            console.error("更新后端 NFT 失败：", updateJson);
          }
        } else {
          console.warn("未在交易日志中解析到 Mint 事件");
        }
      } catch (parseErr) {
        console.error("解析 Mint 事件或回写后端时出错：", parseErr);
      }

      setSuccess("Mint 成功！");
      setName("");
      setDescription("");
      setFile(null);
    } catch (e: unknown) {
      console.error(e);
      const err = e as { reason?: string; message?: string } | undefined;
      setError(err?.reason || err?.message || "Mint 失败，请稍后重试。");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Box>
      <Typography variant="h4" sx={{ mb: 3, fontWeight: 700 }}>
        铸造 NFT
      </Typography>
      <Stack spacing={2} maxWidth={480}>
        {!isConnected && (
          <Alert severity="info">请先右上角连接钱包，然后再 Mint NFT。</Alert>
        )}
        {error && (
          <Alert severity="error" onClose={() => setError(null)}>
            {error}
          </Alert>
        )}
        {success && (
          <Alert severity="success" onClose={() => setSuccess(null)}>
            {success}
          </Alert>
        )}

        <TextField
          label="NFT 名称"
          placeholder="例如：My NFT #1"
          value={name}
          onChange={(e) => setName(e.target.value)}
          fullWidth
        />

        <TextField
          label="NFT 描述"
          placeholder="请输入描述"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          fullWidth
          multiline
          minRows={3}
        />

        <Stack spacing={1}>
          <Typography variant="body2">上传文件（image）</Typography>
          <Button variant="outlined" component="label" disabled={loading}>
            选择文件
            <input
              type="file"
              hidden
              accept="image/*"
              onChange={(e) => {
                const f = e.target.files?.[0] ?? null;
                setFile(f);
              }}
            />
          </Button>
          {file && (
            <Typography variant="body2" color="text.secondary">
              已选择：{file.name}
            </Typography>
          )}
        </Stack>

        <Button
          variant="contained"
          onClick={handleMint}
          disabled={!isConnected || loading}
        >
          {loading ? "Minting..." : "Mint NFT"}
        </Button>
      </Stack>
    </Box>
  );
}

