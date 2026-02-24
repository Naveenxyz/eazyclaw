import { useLayoutEffect, useRef } from "react";
import { MessageSquare } from "lucide-react";
import { MessageBubble } from "@/components/chat/MessageBubble";
import type { Message } from "@/types";

interface MessageListProps {
  messages: Message[];
  hasMore: boolean;
  onLoadMore: () => void;
  loadingMore: boolean;
}

export function MessageList({ messages, hasMore, onLoadMore, loadingMore }: MessageListProps) {
  const listRef = useRef<HTMLDivElement>(null);
  const prevRef = useRef<{
    len: number;
    firstSig: string;
    lastSig: string;
    scrollHeight: number;
    scrollTop: number;
  } | null>(null);

  useLayoutEffect(() => {
    const el = listRef.current;
    if (!el) return;

    const first = messages[0];
    const last = messages[messages.length - 1];
    const firstSig = first ? `${first.role}:${first.content}` : "";
    const lastSig = last ? `${last.role}:${last.content}` : "";
    const prev = prevRef.current;

    if (!prev) {
      el.scrollTop = el.scrollHeight;
      prevRef.current = {
        len: messages.length,
        firstSig,
        lastSig,
        scrollHeight: el.scrollHeight,
        scrollTop: el.scrollTop,
      };
      return;
    }

    const grew = messages.length > prev.len;
    const prepended = grew && firstSig !== prev.firstSig && lastSig === prev.lastSig;

    if (prepended) {
      const delta = el.scrollHeight - prev.scrollHeight;
      el.scrollTop = prev.scrollTop + delta;
    } else {
      const nearBottom = prev.scrollHeight - (prev.scrollTop + el.clientHeight) < 80;
      if (nearBottom || messages.length < prev.len || prev.len === 0) {
        el.scrollTop = el.scrollHeight;
      }
    }

    prevRef.current = {
      len: messages.length,
      firstSig,
      lastSig,
      scrollHeight: el.scrollHeight,
      scrollTop: el.scrollTop,
    };
  }, [messages]);

  if (messages.length === 0) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center bg-base">
        <MessageSquare size={32} className="text-fg-3 opacity-30" />
        <p className="mt-3 font-display font-bold text-fg-3 text-sm">
          Start a conversation
        </p>
        <p className="mt-1 text-fg-3 text-xs opacity-50">
          Type a message below to begin
        </p>
      </div>
    );
  }

  return (
    <div ref={listRef} className="flex-1 overflow-y-auto p-4 bg-base">
      <div className="mx-auto flex max-w-3xl flex-col space-y-3">
        {hasMore && (
          <button
            onClick={onLoadMore}
            disabled={loadingMore}
            className="self-center rounded-md border border-edge bg-raised px-3 py-1.5 text-xs font-medium text-fg-2 hover:text-fg hover:bg-surface transition-colors disabled:opacity-60 disabled:cursor-not-allowed cursor-pointer"
          >
            {loadingMore ? "Loading..." : "Load older messages"}
          </button>
        )}
        {messages.map((message, index) => (
          <MessageBubble key={index} message={message} />
        ))}
      </div>
    </div>
  );
}
