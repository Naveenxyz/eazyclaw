interface TypingIndicatorProps {
  isTyping: boolean;
}

export function TypingIndicator({ isTyping }: TypingIndicatorProps) {
  if (!isTyping) return null;

  return (
    <div className="px-4 pb-2">
      <div className="mx-auto flex max-w-3xl items-center gap-3">
        <div className="flex items-center gap-1">
          <span
            className="inline-block h-1.5 w-1.5 animate-pulse rounded-full bg-accent"
            style={{
              animationDelay: "0ms",
              boxShadow: "0 0 6px rgba(0, 229, 153, 0.4)",
            }}
          />
          <span
            className="inline-block h-1.5 w-1.5 animate-pulse rounded-full bg-accent"
            style={{
              animationDelay: "150ms",
              boxShadow: "0 0 6px rgba(0, 229, 153, 0.4)",
            }}
          />
          <span
            className="inline-block h-1.5 w-1.5 animate-pulse rounded-full bg-accent"
            style={{
              animationDelay: "300ms",
              boxShadow: "0 0 6px rgba(0, 229, 153, 0.4)",
            }}
          />
        </div>
        <span className="text-xs font-mono text-fg-3">Processing...</span>
      </div>
    </div>
  );
}
