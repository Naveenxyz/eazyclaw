import { useEffect, useRef } from "react";
import { MessageSquare } from "lucide-react";
import { MessageBubble } from "@/components/chat/MessageBubble";
import type { Message } from "@/types";

interface MessageListProps {
  messages: Message[];
}

export function MessageList({ messages }: MessageListProps) {
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
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
    <div className="flex-1 overflow-y-auto p-4 bg-base">
      <div className="mx-auto flex max-w-3xl flex-col space-y-3">
        {messages.map((message, index) => (
          <MessageBubble key={index} message={message} />
        ))}
        <div ref={bottomRef} />
      </div>
    </div>
  );
}
