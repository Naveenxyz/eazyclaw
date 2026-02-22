import { useState } from "react";
import IconRail from "@/components/layout/IconRail";
import { ChatTab } from "@/components/chat/ChatTab";
import MemoryTab from "@/components/memory/MemoryTab";
import SkillsTab from "@/components/skills/SkillsTab";
import StatusTab from "@/components/status/StatusTab";
import SettingsTab from "@/components/settings/SettingsTab";
import { useWebSocket } from "@/hooks/use-websocket";

const TABS = ["Chat", "Memory", "Skills", "Status", "Settings"] as const;
type Tab = (typeof TABS)[number];

export default function DashboardPage() {
  const [activeTab, setActiveTab] = useState<Tab>("Chat");
  const ws = useWebSocket();

  const renderTab = () => {
    switch (activeTab) {
      case "Chat":
        return <ChatTab ws={ws} />;
      case "Memory":
        return <MemoryTab />;
      case "Skills":
        return <SkillsTab />;
      case "Status":
        return <StatusTab />;
      case "Settings":
        return <SettingsTab />;
    }
  };

  return (
    <div className="grid grid-cols-[56px_1fr] h-screen bg-[#08090d]">
      <IconRail
        activeTab={activeTab}
        onTabChange={(tab) => setActiveTab(tab as Tab)}
        isConnected={ws.connected}
      />
      <main className="overflow-hidden">{renderTab()}</main>
    </div>
  );
}
