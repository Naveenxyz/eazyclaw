import { useState } from "react";
import Header from "@/components/layout/Header";
import { ChatTab } from "@/components/chat/ChatTab";
import SkillsTab from "@/components/skills/SkillsTab";
import StatusTab from "@/components/status/StatusTab";
import SettingsTab from "@/components/settings/SettingsTab";
import { useWebSocket } from "@/hooks/use-websocket";

const TABS = ["Chat", "Skills", "Status", "Settings"] as const;
type Tab = (typeof TABS)[number];

export default function DashboardPage() {
  const [activeTab, setActiveTab] = useState<Tab>("Chat");
  const ws = useWebSocket();

  const renderTab = () => {
    switch (activeTab) {
      case "Chat":
        return <ChatTab ws={ws} />;
      case "Skills":
        return <SkillsTab />;
      case "Status":
        return <StatusTab />;
      case "Settings":
        return <SettingsTab />;
    }
  };

  return (
    <div className="flex h-screen flex-col bg-zinc-950">
      <Header
        connected={ws.connected}
        activeTab={activeTab}
        onTabChange={(tab) => setActiveTab(tab as Tab)}
      />
      <main className="flex-1 overflow-hidden">{renderTab()}</main>
    </div>
  );
}
