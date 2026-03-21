import { useState, type ReactNode } from "react";
import { Layout } from "./components/Layout";
import { Airdrop } from "./pages/Airdrop";
import { Mint } from "./pages/Mint";
import { Profile } from "./pages/Profile";
import { Market } from "./pages/Market";
import { TokenOwnerTool } from "./pages/TokenOwnerTool";

function App() {
  const [activeTab, setActiveTab] = useState<"airdrop" | "market" | "mint" | "profile" | "tools">("airdrop");

  let content: ReactNode;
  if (activeTab === "mint") {
    content = <Mint />;
  } else if (activeTab === "profile") {
    content = <Profile />;
  } else if (activeTab === "market") {
    content = <Market />;
  } else if (activeTab === "tools") {
    content = <TokenOwnerTool />;
  } else {
    content = <Airdrop />;
  }

  return (
    <Layout activeTab={activeTab} onTabChange={setActiveTab}>
      {content}
    </Layout>
  );
}

export default App;
