import { ToolCallCard } from "@/components/chat/ToolCallCard";
import { MarkdownContent } from "@/components/chat/MarkdownContent";
import type { Message } from "@/types";

interface MessageBubbleProps {
  message: Message;
}

export function MessageBubble({ message }: MessageBubbleProps) {
  const { role, content, tool_calls } = message;

  if (role === "user") {
    return (
      <div className="flex justify-end">
        <div className="max-w-[75%] rounded-lg border border-violet-900/50 bg-violet-950/50 px-4 py-3">
          <div className="mb-1 text-xs font-semibold uppercase tracking-wider text-violet-400">
            You
          </div>
          <div className="text-sm text-zinc-200">{content}</div>
        </div>
      </div>
    );
  }

  if (role === "assistant") {
    return (
      <div className="flex justify-start">
        <div className="max-w-[75%] rounded-lg bg-zinc-800/50 px-4 py-3">
          <div className="mb-1 text-xs font-semibold uppercase tracking-wider text-zinc-400">
            Assistant
          </div>
          {tool_calls && tool_calls.length > 0 && (
            <div className="mb-2 flex flex-col gap-2">
              {tool_calls.map((tc, i) => (
                <ToolCallCard key={i} toolCall={tc} />
              ))}
            </div>
          )}
          {content && (
            <div className="text-sm text-zinc-200">
              <MarkdownContent content={content} />
            </div>
          )}
        </div>
      </div>
    );
  }

  if (role === "tool") {
    return (
      <div className="flex justify-start">
        <div className="max-w-[75%] rounded-lg border-l-2 border-violet-500 bg-zinc-900 px-4 py-3">
          <div className="mb-1 text-xs font-semibold uppercase tracking-wider text-violet-400">
            Tool Result
          </div>
          <pre className="overflow-x-auto text-sm">
            <code className="font-mono text-zinc-300">{content}</code>
          </pre>
        </div>
      </div>
    );
  }

  return null;
}
