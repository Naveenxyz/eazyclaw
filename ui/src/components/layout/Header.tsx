interface HeaderProps {
  connected: boolean;
  activeTab: string;
  onTabChange: (tab: string) => void;
}

const TABS = ["Chat", "Skills", "Status", "Settings"];

export default function Header({ connected, activeTab, onTabChange }: HeaderProps) {
  return (
    <header className="border-b border-zinc-800 bg-zinc-900">
      <div className="flex items-center justify-between px-4 py-3">
        <span className="text-lg font-bold text-violet-500">EazyClaw</span>
        <div className="flex items-center gap-2">
          <span
            className={`inline-block h-2.5 w-2.5 rounded-full ${
              connected ? "bg-green-500" : "bg-red-500"
            }`}
          />
          <span className="text-sm text-zinc-400">
            {connected ? "Connected" : "Disconnected"}
          </span>
        </div>
      </div>

      <nav className="flex gap-1 px-4 pb-2">
        {TABS.map((tab) => (
          <button
            key={tab}
            onClick={() => onTabChange(tab)}
            className={`rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
              activeTab === tab
                ? "bg-violet-600 text-white"
                : "text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200"
            }`}
          >
            {tab}
          </button>
        ))}
      </nav>
    </header>
  );
}
