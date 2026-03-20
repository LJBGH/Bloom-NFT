import { Button, Typography } from "@mui/material";
import { useWeb3 } from "../web3/provider";

function shortenAddress(addr: string) {
  return `${addr.slice(0, 6)}...${addr.slice(-4)}`;
}

export function WalletButton() {
  const { account, isConnected, connectWallet, disconnect } = useWeb3();

  if (!isConnected) {
    return (
      <Button color="inherit" variant="outlined" onClick={connectWallet}>
        Connect Wallet
      </Button>
    );
  }

  return (
    <Button color="inherit" variant="outlined" onClick={disconnect}>
      <Typography variant="body2" component="span">
        {shortenAddress(account!)}
      </Typography>
    </Button>
  );
}

