import { createContext, useContext, useEffect, useMemo, useState } from "react";
import { BrowserProvider, JsonRpcSigner } from "ethers";
import type { Eip1193Provider } from "ethers";

type Web3ContextValue = {
  account: string | null;
  isConnected: boolean;
  chainId: number | null;
  provider: BrowserProvider | null;
  signer: JsonRpcSigner | null;
  connectWallet: () => Promise<void>;
  disconnect: () => void;
};

const Web3Context = createContext<Web3ContextValue | undefined>(undefined);

function getEthereumFromWindow(): Eip1193Provider | null {
  if (typeof window === "undefined") return null;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const anyWindow = window as any;
  return anyWindow.ethereum ?? null;
}

export function Web3Provider({ children }: { children: React.ReactNode }) {
  const [provider, setProvider] = useState<BrowserProvider | null>(null);
  const [signer, setSigner] = useState<JsonRpcSigner | null>(null);
  const [account, setAccount] = useState<string | null>(null);
  const [chainId, setChainId] = useState<number | null>(null);

  useEffect(() => {
    const eth = getEthereumFromWindow();
    if (!eth) return;

    const browserProvider = new BrowserProvider(eth);
    setProvider(browserProvider);

    // Try to hydrate from already connected accounts
    browserProvider
      .listAccounts()
      .then(async (accounts) => {
        if (accounts.length > 0) {
          const s = await browserProvider.getSigner();
          setSigner(s);
          setAccount(await s.getAddress());
        }
      })
      .catch(() => {
        // ignore
      });

    browserProvider
      .getNetwork()
      .then((n) => setChainId(Number(n.chainId)))
      .catch(() => {
        // ignore
      });

    // Subscribe to account / chain changes
    const handleAccountsChanged = (accounts: string[]) => {
      if (accounts.length === 0) {
        setAccount(null);
        setSigner(null);
      } else {
        setAccount(accounts[0]);
        browserProvider.getSigner().then(setSigner).catch(() => {
          setSigner(null);
        });
      }
    };

    const handleChainChanged = (newChainId: string) => {
      setChainId(parseInt(newChainId, 16));
    };

    eth.on?.("accountsChanged", handleAccountsChanged);
    eth.on?.("chainChanged", handleChainChanged);

    return () => {
      eth.removeListener?.("accountsChanged", handleAccountsChanged);
      eth.removeListener?.("chainChanged", handleChainChanged);
    };
  }, []);

  const connectWallet = async () => {
    const eth = getEthereumFromWindow();
    if (!eth) {
      alert("No Ethereum provider found. Please install MetaMask.");
      return;
    }

    const browserProvider = provider ?? new BrowserProvider(eth);
    if (!provider) {
      setProvider(browserProvider);
    }

    const accounts = await eth.request?.({ method: "eth_requestAccounts" });
    const selected = Array.isArray(accounts) && accounts.length > 0 ? accounts[0] : null;
    if (selected) {
      const s = await browserProvider.getSigner();
      setSigner(s);
      setAccount(selected);
      const network = await browserProvider.getNetwork();
      setChainId(Number(network.chainId));
    }
  };

  const value = useMemo<Web3ContextValue>(
    () => ({
      account,
      isConnected: !!account,
      chainId,
      provider,
      signer,
      connectWallet,
      disconnect: () => {
        setAccount(null);
        setSigner(null);
      },
    }),
    [account, chainId, provider, signer]
  );

  return <Web3Context.Provider value={value}>{children}</Web3Context.Provider>;
}

export function useWeb3(): Web3ContextValue {
  const ctx = useContext(Web3Context);
  if (!ctx) {
    throw new Error("useWeb3 must be used within a Web3Provider");
  }
  return ctx;
}

