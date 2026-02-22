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
  const [selectedSessionId, setSelectedSessionId] = useState<string>("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [isTyping, setIsTyping] = useState(false);

  useEffect(() => {
    getSessions().then(setSessions).catch(() => {});
  }, []);

  useEffect(() => {
    if (!selectedSessionId) return;
    getSession(selectedSessionId).then((session) => {
      setMessages(session.messages);
    }).catch(() => {});
  }, [selectedSessionId]);

  useEffect(() => {
    if (!ws.lastMessage) return;

    if (ws.lastMessage.type === "message") {
      const newMsg: Message = {
        role: (ws.lastMessage.role as Message["role"]) || "assistant",
        content: ws.lastMessage.content || "",
      };
      setMessages((prev) => [...prev, newMsg]);
      setIsTyping(false);
      // Fetch full session to get tool_calls
      if (selectedSessionId) {
        getSession(selectedSessionId).then((session) => {
          setMessages(session.messages);
        }).catch(() => {});
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
    [ws]
  );

  return (
    <div className="flex h-full flex-row">
      <SessionList
        sessions={sessions}
        selectedId={selectedSessionId}
        onSelect={handleSelect}
      />
      <div className="flex flex-1 flex-col">
        <MessageList messages={messages} />
        <TypingIndicator visible={isTyping} />
        <ChatInput onSend={handleSend} disabled={isTyping} />
      </div>
    </div>
  );
}
