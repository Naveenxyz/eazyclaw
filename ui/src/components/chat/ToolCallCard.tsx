import { useState } from "react";
import { ChevronDown } from "lucide-react";
import type { ToolCall } from "@/types";

interface ToolCallCardProps {
  toolCall: ToolCall;
}

function formatArguments(raw: string | Record<string, unknown>): string {
  if (typeof raw === "object" && raw !== null) {
    return JSON.stringify(raw, null, 2);
  }
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return String(raw);
  }
}

export function ToolCallCard({ toolCall }: ToolCallCardProps) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="rounded-md border border-edge bg-raised overflow-hidden">
      {/* Clickable header */}
      <button
        onClick={() => setExpanded((prev) => !prev)}
        className="flex w-full items-center justify-between px-3 py-2 text-left transition-colors duration-150 hover:bg-elevated cursor-pointer"
      >
        <span className="badge-accent badge">
          {toolCall.name}
        </span>
        <ChevronDown
          size={14}
          className={`text-fg-3 transition-transform duration-200 ${
            expanded ? "rotate-180" : ""
          }`}
        />
      </button>

      {/* Expandable body */}
      <div
        className={`overflow-hidden transition-all duration-200 ease-in-out ${
          expanded ? "max-h-[200px]" : "max-h-0"
        }`}
      >
        <div className="border-t border-edge px-3 py-2">
          <pre className="max-h-40 overflow-auto rounded bg-base p-2">
            <code className="text-xs font-mono text-fg-2 whitespace-pre-wrap">
              {formatArguments(toolCall.arguments)}
            </code>
          </pre>
        </div>
      </div>
    </div>
  );
}
