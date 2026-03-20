import {
  Box,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Typography,
} from "@mui/material";
import { useWeb3 } from "../web3/provider";
import { useEffect, useState } from "react";
import { API_ENDPOINTS } from "../config/api";

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
}

export function Profile() {
  const { account, isConnected } = useWeb3();
  const [loading, setLoading] = useState(false);
  const [categories, setCategories] = useState<NftCategory[]>([]);
  const [selectedCategoryId, setSelectedCategoryId] = useState<number | null>(
    null
  );
  const [nftList, setNftList] = useState<NftItem[]>([]);
  const [error, setError] = useState<string | null>(null);

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
      return;
    }

    const fetchNftList = async () => {
      try {
        setLoading(true);
        setError(null);
        const resp = await fetch(
          `${API_ENDPOINTS.nftUserListByCategory(
            selectedCategoryId
          )}?owner=${account}`
        );
        const data = await resp.json();
        if (!resp.ok || data.code !== 0) {
          throw new Error(data.message || "获取 NFT 列表失败");
        }
        const list: NftItem[] = data.data || [];
        setNftList(list);
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
          <Typography sx={{ mb: 2 }}>当前地址：{account}</Typography>

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
                    xs: "1fr",
                    sm: "repeat(2, 1fr)",
                    md: "repeat(3, 1fr)",
                  },
                  gap: 2,
                }}
              >
                {nftList.map((nft) => (
                  <Card
                    key={nft.id}
                    sx={{
                      height: "100%",
                      display: "flex",
                      flexDirection: "column",
                    }}
                  >
                    <Box
                      component="img"
                      src={nft.imageUrl}
                      alt={nft.name}
                      sx={{
                        width: "100%",
                        height: 220,
                        objectFit: "cover",
                        borderTopLeftRadius: 2,
                        borderTopRightRadius: 2,
                      }}
                    />
                    <CardContent sx={{ flexGrow: 1 }}>
                      <Typography
                        variant="h6"
                        sx={{ fontWeight: 600, mb: 1 }}
                      >
                        {nft.name}
                      </Typography>
                      <Typography
                        variant="body2"
                        color="text.secondary"
                        sx={{ mb: 1 }}
                        noWrap
                      >
                        {nft.description}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        Token ID: {nft.tokenId}
                      </Typography>
                    </CardContent>
                  </Card>
                ))}
              </Box>
            </>
          )}
        </>
      )}
    </>
  );
}

