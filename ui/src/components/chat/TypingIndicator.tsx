interface TypingIndicatorProps {
  visible: boolean;
}

export function TypingIndicator({ visible }: TypingIndicatorProps) {
  if (!visible) return null;

  return (
    <div className="px-4 py-2">
      <div className="mx-auto flex max-w-3xl items-center gap-1.5">
        <span className="inline-block h-2 w-2 animate-pulse rounded-full bg-violet-500 shadow-[0_0_8px_2px] shadow-violet-500/50 [animation-delay:0ms]" />
        <span className="inline-block h-2 w-2 animate-pulse rounded-full bg-violet-500 shadow-[0_0_8px_2px] shadow-violet-500/50 [animation-delay:150ms]" />
        <span className="inline-block h-2 w-2 animate-pulse rounded-full bg-violet-500 shadow-[0_0_8px_2px] shadow-violet-500/50 [animation-delay:300ms]" />
      </div>
    </div>
  );
}
