import { useState } from "react";
import type { ToolCall } from "@/types";

interface ToolCallCardProps {
  toolCall: ToolCall;
}

export function ToolCallCard({ toolCall }: ToolCallCardProps) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="rounded-md border border-zinc-800 bg-zinc-900">
      <button
        onClick={() => setExpanded((prev) => !prev)}
        className="flex w-full items-center justify-between px-3 py-2 text-left"
      >
        <span className="rounded bg-violet-500/20 px-2 py-0.5 text-xs font-medium text-violet-300">
          {toolCall.name}
        </span>
        <svg
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 20 20"
          fill="currentColor"
          className={`h-4 w-4 text-zinc-500 transition-transform ${
            expanded ? "rotate-180" : ""
          }`}
        >
          <path
            fillRule="evenodd"
            d="M5.22 8.22a.75.75 0 0 1 1.06 0L10 11.94l3.72-3.72a.75.75 0 1 1 1.06 1.06l-4.25 4.25a.75.75 0 0 1-1.06 0L5.22 9.28a.75.75 0 0 1 0-1.06Z"
            clipRule="evenodd"
          />
        </svg>
      </button>
      {expanded && (
        <div className="border-t border-zinc-800 px-3 py-2">
          <pre className="overflow-x-auto text-xs">
            <code className="font-mono text-zinc-400">
              {JSON.stringify(toolCall.arguments, null, 2)}
            </code>
          </pre>
        </div>
      )}
    </div>
  );
}
