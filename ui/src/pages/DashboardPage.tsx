import { useState } from "react";
import { ChatTab } from "@/components/chat/ChatTab";
import MemoryTab from "@/components/memory/MemoryTab";
import SkillsTab from "@/components/skills/SkillsTab";
import StatusTab from "@/components/status/StatusTab";
import CronTab from "@/components/cron/CronTab";
import SettingsTab from "@/components/settings/SettingsTab";
import { IconRail } from "@/components/layout/IconRail";
import { useWebSocket } from "@/hooks/use-websocket";

const TABS = ["chat", "memory", "skills", "status", "cron", "settings"] as const;
type Tab = (typeof TABS)[number];

export default function DashboardPage() {
  const [activeTab, setActiveTab] = useState<Tab>("chat");
  const ws = useWebSocket();

  const renderTab = () => {
    switch (activeTab) {
      case "chat":
        return <ChatTab ws={ws} />;
      case "memory":
        return <MemoryTab />;
      case "skills":
        return <SkillsTab />;
      case "status":
        return <StatusTab />;
      case "cron":
        return <CronTab />;
      case "settings":
        return <SettingsTab />;
    }
  };

  return (
    <div className="grid grid-cols-[56px_1fr] min-h-screen bg-base relative overflow-hidden">
      {/* Pulse bar — ambient top-of-screen accent line */}
      <div className="pulse-bar absolute top-0 left-0 right-0 z-50" />

      {/* Ambient glow orbs */}
      <div
        className="pointer-events-none fixed z-0"
        aria-hidden="true"
        style={{
          width: 480,
          height: 480,
          top: "-8%",
          right: "-6%",
          background:
            "radial-gradient(circle, rgba(0, 229, 153, 0.03) 0%, transparent 70%)",
          animation: "drift 20s ease-in-out infinite",
        }}
      />
      <div
        className="pointer-events-none fixed z-0"
        aria-hidden="true"
        style={{
          width: 360,
          height: 360,
          bottom: "5%",
          left: "12%",
          background:
            "radial-gradient(circle, rgba(0, 229, 153, 0.03) 0%, transparent 70%)",
          animation: "drift 26s ease-in-out infinite reverse",
        }}
      />

      {/* Left sidebar */}
      <IconRail
        activeTab={activeTab}
        onTabChange={(tab) => setActiveTab(tab as Tab)}
        connected={ws.connected}
      />

      {/* Main content area */}
      <main className="overflow-y-auto relative z-10">{renderTab()}</main>
    </div>
  );
}
