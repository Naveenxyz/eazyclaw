import type { SessionSummary } from "@/types";

interface SessionListProps {
  sessions: SessionSummary[];
  selectedId: string;
  onSelect: (id: string) => void;
}

function formatRelativeTime(dateString: string): string {
  const now = new Date();
  const date = new Date(dateString);
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);
  const diffMin = Math.floor(diffSec / 60);
  const diffHr = Math.floor(diffMin / 60);
  const diffDay = Math.floor(diffHr / 24);

  if (diffSec < 60) return "just now";
  if (diffMin < 60) return `${diffMin}m ago`;
  if (diffHr < 24) return `${diffHr}h ago`;
  return `${diffDay}d ago`;
}

function truncateId(id: string, maxLen = 12): string {
  return id.length > maxLen ? id.slice(0, maxLen) + "..." : id;
}

export function SessionList({ sessions, selectedId, onSelect }: SessionListProps) {
  return (
    <div className="flex w-[260px] flex-col border-r border-violet-500/10 bg-[#0f1117]">
      <div className="border-b border-violet-500/10 px-4 py-3">
        <h2 className="text-sm font-mono font-semibold uppercase tracking-wider text-slate-400">
          Sessions
        </h2>
      </div>
      <div className="flex-1 overflow-y-auto">
        {sessions.map((session) => {
          const isActive = session.id === selectedId;
          return (
            <button
              key={session.id}
              onClick={() => onSelect(session.id)}
              className={`w-full px-4 py-3 text-left transition-colors ${
                isActive
                  ? "border-l-2 border-violet-500 bg-violet-500/10 text-slate-200"
                  : "border-l-2 border-transparent text-slate-300 hover:bg-white/5"
              }`}
            >
              <div className="truncate text-sm font-medium font-mono">
                {truncateId(session.id)}
              </div>
              <div className="mt-1 flex items-center justify-between text-xs text-slate-500">
                <span>{session.message_count} messages</span>
                <span>{formatRelativeTime(session.updated)}</span>
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}
