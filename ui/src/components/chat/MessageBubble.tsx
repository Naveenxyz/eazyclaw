import { ToolCallCard } from "@/components/chat/ToolCallCard";
import { MarkdownContent } from "@/components/chat/MarkdownContent";
import type { Message } from "@/types";

interface MessageBubbleProps {
  message: Message;
}

export function MessageBubble({ message }: MessageBubbleProps) {
  const { role, content, tool_calls } = message;

  // ── User message ──────────────────────────────────────────────
  if (role === "user") {
    return (
      <div className="flex justify-end">
        <div className="max-w-[75%] rounded-lg bg-raised p-3">
          <div className="mb-1.5 flex items-center gap-1.5">
            <span className="inline-block h-1.5 w-1.5 rounded-full bg-accent" />
            <span className="font-mono text-[10px] uppercase tracking-wider text-fg-3">
              You
            </span>
          </div>
          <div className="text-sm text-fg">{content}</div>
        </div>
      </div>
    );
  }

  // ── Assistant message ─────────────────────────────────────────
  if (role === "assistant") {
    return (
      <div className="flex justify-start">
        <div
          className="w-full rounded-lg border border-edge bg-surface p-3"
          style={{ borderLeftWidth: "2px", borderLeftColor: "#7B8CFF" }}
        >
          <div className="mb-1.5 flex items-center gap-1.5">
            <span className="inline-block h-1.5 w-1.5 rounded-full bg-info" />
            <span className="font-mono text-[10px] uppercase tracking-wider text-fg-3">
              Agent
            </span>
          </div>

          {/* Tool calls rendered before content */}
          {tool_calls && tool_calls.length > 0 && (
            <div className="mb-2 flex flex-col gap-2">
              {tool_calls.map((tc, i) => (
                <ToolCallCard key={tc.id || i} toolCall={tc} />
              ))}
            </div>
          )}

          {content && <MarkdownContent content={content} />}
        </div>
      </div>
    );
  }

  // ── Tool message ──────────────────────────────────────────────
  if (role === "tool") {
    return (
      <div className="flex justify-start">
        <div className="max-w-[75%] rounded-lg bg-raised/50 p-3">
          <div className="mb-1.5 flex items-center gap-1.5">
            <span className="inline-block h-1.5 w-1.5 rounded-full bg-fg-3" />
            <span className="font-mono text-[10px] uppercase tracking-wider text-fg-3">
              Tool
            </span>
          </div>
          <pre className="max-h-48 overflow-auto rounded bg-base p-2">
            <code className="text-xs font-mono text-fg-2 whitespace-pre-wrap break-all">
              {content}
            </code>
          </pre>
        </div>
      </div>
    );
  }

  return null;
}
