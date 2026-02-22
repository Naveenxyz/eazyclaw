import { useState, useEffect, useCallback } from "react";
import { getSessions, getSession } from "@/lib/api";
import { SessionList } from "@/components/chat/SessionList";
import { MessageList } from "@/components/chat/MessageList";
import { ChatInput } from "@/components/chat/ChatInput";
import { TypingIndicator } from "@/components/chat/TypingIndicator";
import type { Message, SessionSummary, WSMessage } from "@/types";

interface ChatTabProps {
  ws: {
    connected: boolean;
    send: (text: string) => void;
    lastMessage: WSMessage | null;
  };
}

export function ChatTab({ ws }: ChatTabProps) {
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [isTyping, setIsTyping] = useState(false);

  // Load sessions on mount and auto-refresh every 4s
  useEffect(() => {
    let mounted = true;

    const load = () => {
      getSessions()
        .then((list) => {
          if (!mounted) return;
          setSessions(list);
        })
        .catch(() => {});
    };

    load();
    const timer = setInterval(load, 4000);
    return () => {
      mounted = false;
      clearInterval(timer);
    };
  }, []);

  // Auto-select first session when none is selected
  useEffect(() => {
    if (selectedSessionId) return;
    const first = sessions[0];
    if (!first) return;
    setSelectedSessionId(first.id);
  }, [sessions, selectedSessionId]);

  // Fetch messages when selected session changes
  useEffect(() => {
    if (!selectedSessionId) return;
    getSession(selectedSessionId)
      .then((session) => {
        setMessages(session.messages);
      })
      .catch(() => {});
  }, [selectedSessionId]);

  // Handle incoming WebSocket messages
  useEffect(() => {
    if (!ws.lastMessage) return;

    if (ws.lastMessage.type === "message") {
      const newMsg: Message = {
        role: (ws.lastMessage.role as Message["role"]) || "assistant",
        content: ws.lastMessage.content || "",
      };
      setMessages((prev) => [...prev, newMsg]);
      setIsTyping(false);

      // Re-fetch full session to capture tool_calls
      if (selectedSessionId) {
        getSession(selectedSessionId)
          .then((session) => {
            setMessages(session.messages);
          })
          .catch(() => {});
      }
    } else if (ws.lastMessage.type === "typing") {
      setIsTyping(true);
    } else if (ws.lastMessage.type === "done") {
      setIsTyping(false);
    }
  }, [ws.lastMessage, selectedSessionId]);

  const handleSelect = useCallback((id: string) => {
    setSelectedSessionId(id);
  }, []);

  const handleSend = useCallback(
    (text: string) => {
      const userMessage: Message = {
        role: "user",
        content: text,
      };
      setMessages((prev) => [...prev, userMessage]);
      ws.send(text);
      setIsTyping(true);
    },
    [ws],
  );

  return (
    <div className="flex h-full flex-row bg-base">
      <SessionList
        sessions={sessions}
        selectedId={selectedSessionId}
        onSelect={handleSelect}
      />

      <div className="flex flex-1 flex-col min-w-0">
        <MessageList messages={messages} />
        <TypingIndicator isTyping={isTyping} />
        <ChatInput onSend={handleSend} disabled={!ws.connected || isTyping} />
      </div>
    </div>
  );
}
