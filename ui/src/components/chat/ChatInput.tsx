import { useState, useCallback } from "react";

interface ChatInputProps {
  onSend: (text: string) => void;
  disabled?: boolean;
}

export function ChatInput({ onSend, disabled = false }: ChatInputProps) {
  const [value, setValue] = useState("");

  const handleSend = useCallback(() => {
    const trimmed = value.trim();
    if (!trimmed) return;
    onSend(trimmed);
    setValue("");
  }, [value, onSend]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLInputElement>) => {
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        handleSend();
      }
    },
    [handleSend]
  );

  return (
    <div className="p-4">
      <div className="glass-card mx-auto flex max-w-3xl items-center gap-2 rounded-lg border border-violet-500/10 bg-[#0f1117]/80 px-4 py-3 backdrop-blur-sm">
        <input
          type="text"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={handleKeyDown}
          disabled={disabled}
          placeholder="Type a message..."
          className="flex-1 bg-transparent px-2 py-1 text-sm text-slate-200 placeholder-slate-500 outline-none focus:ring-1 focus:ring-violet-500/50 rounded disabled:opacity-50"
        />
        <button
          onClick={handleSend}
          disabled={disabled || !value.trim()}
          className="rounded-lg bg-violet-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-violet-500 disabled:opacity-50 disabled:hover:bg-violet-600"
        >
          Send
        </button>
      </div>
    </div>
  );
}
