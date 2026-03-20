import { AppBar, Box, Button, Container, Stack, Toolbar, Typography } from "@mui/material";
import { WalletButton } from "./WalletButton";

type Props = {
  children: React.ReactNode;
  activeTab: "airdrop" | "mint" | "profile";
  onTabChange: (tab: "airdrop" | "mint" | "profile") => void;
};

export function Layout({ children, activeTab, onTabChange }: Props) {
  return (
    <Box sx={{ minHeight: "100vh", bgcolor: "background.default" }}>
      <AppBar position="static" color="transparent" elevation={0}>
        <Toolbar>
          <Typography variant="h6" sx={{ fontWeight: 700, mr: 4 }}>
            Bloom NFT DApp
          </Typography>
          <Stack direction="row" spacing={1} sx={{ flexGrow: 1 }}>
            <Button
              color={activeTab === "mint" ? "primary" : "inherit"}
              variant={activeTab === "mint" ? "contained" : "text"}
              onClick={() => onTabChange("mint")}
            >
              铸造
            </Button>
            <Button
              color={activeTab === "profile" ? "primary" : "inherit"}
              variant={activeTab === "profile" ? "contained" : "text"}
              onClick={() => onTabChange("profile")}
            >
              个人
            </Button>
            <Button
              color={activeTab === "airdrop" ? "primary" : "inherit"}
              variant={activeTab === "airdrop" ? "contained" : "text"}
              onClick={() => onTabChange("airdrop")}
            >
              空投
            </Button>
          </Stack>
          <WalletButton />
        </Toolbar>
      </AppBar>
      <Container maxWidth="lg" sx={{ pt: 4, pb: 6 }}>
        {children}
      </Container>
    </Box>
  );
}

