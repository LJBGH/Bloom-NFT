import { useState } from "react";
import { Layout } from "./components/Layout";
import { Airdrop } from "./pages/Airdrop";
import { Mint } from "./pages/Mint";
import { Profile } from "./pages/Profile";

function App() {
  const [activeTab, setActiveTab] = useState<"airdrop" | "mint" | "profile">("airdrop");

  let content: JSX.Element;
  if (activeTab === "mint") {
    content = <Mint />;
  } else if (activeTab === "profile") {
    content = <Profile />;
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
