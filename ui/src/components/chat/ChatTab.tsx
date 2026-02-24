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
  const SESSIONS_PAGE_SIZE = 50;
  const MESSAGES_PAGE_SIZE = 120;

  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [sessionsTotal, setSessionsTotal] = useState(0);
  const [loadingMoreSessions, setLoadingMoreSessions] = useState(false);
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [isTyping, setIsTyping] = useState(false);
  const [loadingOlderMessages, setLoadingOlderMessages] = useState(false);
  const [hasMoreMessages, setHasMoreMessages] = useState(false);
  const [nextBeforeSeq, setNextBeforeSeq] = useState<number | null>(null);

  const mergeSessionLists = useCallback((fresh: SessionSummary[], current: SessionSummary[]) => {
    const out = [...fresh];
    const seen = new Set(fresh.map((s) => s.id));
    for (const s of current) {
      if (!seen.has(s.id)) out.push(s);
    }
    return out;
  }, []);

  const loadSessionLatest = useCallback((sessionId: string) => {
    getSession(sessionId, { limit: MESSAGES_PAGE_SIZE })
      .then((session) => {
        setMessages(session.messages ?? []);
        const pagination = session.pagination;
        setHasMoreMessages(Boolean(pagination?.has_more));
        setNextBeforeSeq(
          typeof pagination?.next_before_seq === "number" ? pagination.next_before_seq : null
        );
      })
      .catch(() => {});
  }, []);

  // Load sessions on mount and auto-refresh every 4s
  useEffect(() => {
    let mounted = true;

    const load = () => {
      getSessions({ limit: SESSIONS_PAGE_SIZE, offset: 0 })
        .then((page) => {
          if (!mounted) return;
          setSessions((current) => mergeSessionLists(page.items ?? [], current));
          setSessionsTotal(page.pagination?.total ?? 0);
        })
        .catch(() => {});
    };

    load();
    const timer = setInterval(load, 4000);
    return () => {
      mounted = false;
      clearInterval(timer);
    };
  }, [mergeSessionLists]);

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
    loadSessionLatest(selectedSessionId);
  }, [selectedSessionId, loadSessionLatest]);

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

      // Re-fetch latest page to capture tool_calls and persist canonical history order.
      if (selectedSessionId) {
        loadSessionLatest(selectedSessionId);
      }
    } else if (ws.lastMessage.type === "typing") {
      setIsTyping(true);
    } else if (ws.lastMessage.type === "done") {
      setIsTyping(false);
    }
  }, [ws.lastMessage, selectedSessionId, loadSessionLatest]);

  const handleSelect = useCallback((id: string) => {
    setSelectedSessionId(id);
  }, []);

  const handleLoadMoreSessions = useCallback(() => {
    if (loadingMoreSessions) return;
    if (sessions.length >= sessionsTotal && sessionsTotal > 0) return;

    setLoadingMoreSessions(true);
    getSessions({ limit: SESSIONS_PAGE_SIZE, offset: sessions.length })
      .then((page) => {
        setSessions((current) => {
          const existing = new Set(current.map((s) => s.id));
          const next = [...current];
          for (const row of page.items ?? []) {
            if (!existing.has(row.id)) {
              next.push(row);
            }
          }
          return next;
        });
        setSessionsTotal(page.pagination?.total ?? sessionsTotal);
      })
      .catch(() => {})
      .finally(() => setLoadingMoreSessions(false));
  }, [loadingMoreSessions, sessions.length, sessionsTotal]);

  const handleLoadOlderMessages = useCallback(() => {
    if (!selectedSessionId || !hasMoreMessages || loadingOlderMessages) return;
    if (typeof nextBeforeSeq !== "number" || nextBeforeSeq <= 0) return;

    setLoadingOlderMessages(true);
    getSession(selectedSessionId, {
      limit: MESSAGES_PAGE_SIZE,
      beforeSeq: nextBeforeSeq,
    })
      .then((session) => {
        const older = session.messages ?? [];
        setMessages((current) => [...older, ...current]);
        const pagination = session.pagination;
        setHasMoreMessages(Boolean(pagination?.has_more));
        setNextBeforeSeq(
          typeof pagination?.next_before_seq === "number" ? pagination.next_before_seq : null
        );
      })
      .catch(() => {})
      .finally(() => setLoadingOlderMessages(false));
  }, [selectedSessionId, hasMoreMessages, loadingOlderMessages, nextBeforeSeq]);

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
        hasMore={sessions.length < sessionsTotal}
        onLoadMore={handleLoadMoreSessions}
        loadingMore={loadingMoreSessions}
      />

      <div className="flex flex-1 flex-col min-w-0">
        <MessageList
          messages={messages}
          hasMore={hasMoreMessages}
          onLoadMore={handleLoadOlderMessages}
          loadingMore={loadingOlderMessages}
        />
        <TypingIndicator isTyping={isTyping} />
        <ChatInput onSend={handleSend} disabled={!ws.connected || isTyping} />
      </div>
    </div>
  );
}
