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
    <div className="flex w-[260px] flex-col border-r border-zinc-800 bg-zinc-900">
      <div className="border-b border-zinc-800 px-4 py-3">
        <h2 className="text-sm font-semibold uppercase tracking-wider text-zinc-400">
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
                  ? "bg-violet-600/20 text-violet-200"
                  : "text-zinc-300 hover:bg-zinc-800"
              }`}
            >
              <div className="truncate text-sm font-medium">
                {truncateId(session.id)}
              </div>
              <div className="mt-1 flex items-center justify-between text-xs text-zinc-500">
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
