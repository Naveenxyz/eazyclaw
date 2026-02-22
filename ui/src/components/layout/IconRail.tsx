import {
  MessageSquare,
  Brain,
  Puzzle,
  Activity,
  Clock,
  Settings,
} from "lucide-react";

interface IconRailProps {
  activeTab: string;
  onTabChange: (tab: string) => void;
  connected: boolean;
}

const NAV_ITEMS = [
  { id: "chat", icon: MessageSquare, label: "Chat" },
  { id: "memory", icon: Brain, label: "Memory" },
  { id: "skills", icon: Puzzle, label: "Skills" },
  { id: "status", icon: Activity, label: "Status" },
  { id: "cron", icon: Clock, label: "Cron" },
  { id: "settings", icon: Settings, label: "Config" },
] as const;

export function IconRail({ activeTab, onTabChange, connected }: IconRailProps) {
  return (
    <nav className="flex w-[56px] flex-col items-center bg-surface border-r border-edge h-full select-none">
      {/* Monogram */}
      <div className="flex h-[52px] w-full items-center justify-center border-b border-edge shrink-0">
        <span className="font-display text-accent text-[24px] font-extrabold leading-none">
          E
        </span>
      </div>

      {/* Navigation items */}
      <div className="stagger flex flex-col items-center gap-[4px] pt-3 flex-1">
        {NAV_ITEMS.map(({ id, icon: Icon, label }) => {
          const isActive = activeTab === id;

          return (
            <button
              key={id}
              onClick={() => onTabChange(id)}
              className={[
                "group relative flex w-[40px] h-[40px] flex-col items-center justify-center rounded transition-all duration-150",
                isActive
                  ? "bg-accent-dim border-l-[3px] border-l-accent"
                  : "border-l-[3px] border-l-transparent hover:bg-raised",
              ].join(" ")}
              title={label}
            >
              <Icon
                size={18}
                className={[
                  "transition-colors duration-150",
                  isActive
                    ? "text-accent"
                    : "text-fg-3 group-hover:text-fg-2",
                ].join(" ")}
              />
              <span
                className={[
                  "font-mono text-[9px] uppercase leading-none mt-[2px] transition-colors duration-150",
                  isActive
                    ? "text-accent"
                    : "text-fg-3 group-hover:text-fg-2",
                ].join(" ")}
              >
                {label}
              </span>
            </button>
          );
        })}
      </div>

      {/* Bottom: system status */}
      <div className="flex flex-col items-center gap-1.5 pb-4 pt-3 border-t border-edge w-[40px] shrink-0">
        <div
          className={`status-indicator ${connected ? "online" : "offline"}`}
          title={connected ? "System Online" : "System Offline"}
        />
        <span className="font-mono text-[9px] uppercase text-fg-3 leading-none">
          SYS
        </span>
      </div>
    </nav>
  );
}

export default IconRail;
