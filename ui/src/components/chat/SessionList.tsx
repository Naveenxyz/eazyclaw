import { MessageSquare } from "lucide-react";
import type { SessionSummary } from "@/types";

interface SessionListProps {
  sessions: SessionSummary[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  hasMore: boolean;
  onLoadMore: () => void;
  loadingMore: boolean;
}

function formatRelativeTime(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diffMs = now - then;
  const diffSec = Math.floor(diffMs / 1000);
  const diffMin = Math.floor(diffSec / 60);
  const diffHr = Math.floor(diffMin / 60);
  const diffDay = Math.floor(diffHr / 24);

  if (diffSec < 60) return "just now";
  if (diffMin < 60) return `${diffMin}m ago`;
  if (diffHr < 24) return `${diffHr}h ago`;
  return `${diffDay}d ago`;
}

export function SessionList({
  sessions,
  selectedId,
  onSelect,
  hasMore,
  onLoadMore,
  loadingMore,
}: SessionListProps) {
  return (
    <div className="flex w-60 flex-col border-r border-edge bg-surface h-full">
      {/* Header */}
      <div className="border-b border-edge px-4 py-3">
        <h2 className="section-label">Sessions</h2>
      </div>

      {/* Session list */}
      <div className="flex-1 overflow-y-auto">
        {sessions.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-16 px-4">
            <MessageSquare size={28} className="text-fg-3 opacity-40" />
            <p className="mt-3 text-xs text-fg-3 font-display font-bold">
              No sessions yet
            </p>
          </div>
        ) : (
          sessions.map((session) => {
            const isActive = session.id === selectedId;
            return (
              <button
                key={session.id}
                onClick={() => onSelect(session.id)}
                className={`w-full px-4 py-3 text-left transition-colors duration-150 cursor-pointer ${
                  isActive
                    ? "border-l-2 border-accent bg-accent-dim text-fg"
                    : "border-l-2 border-transparent text-fg-2 hover:bg-raised"
                }`}
              >
                <div className="truncate text-sm font-mono text-fg">
                  {session.id.slice(0, 10)}
                </div>
                <div className="mt-1.5 flex items-center justify-between">
                  <span className="badge-neutral badge text-[10px]">
                    {session.message_count} msgs
                  </span>
                  <span className="text-xs text-fg-3">
                    {formatRelativeTime(session.updated)}
                  </span>
                </div>
              </button>
            );
          })
        )}

        {hasMore && (
          <div className="p-3 border-t border-edge">
            <button
              onClick={onLoadMore}
              disabled={loadingMore}
              className="w-full rounded-md border border-edge bg-raised px-3 py-2 text-xs font-medium text-fg-2 hover:text-fg hover:bg-surface transition-colors disabled:opacity-60 disabled:cursor-not-allowed cursor-pointer"
            >
              {loadingMore ? "Loading..." : "Load older sessions"}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
