import { useState } from "react";
import { Plus, X, Check, Ban } from "lucide-react";
import type {
  TelegramAdminState,
  TelegramChannelConfig,
  TelegramChatConfig,
} from "@/types";

interface TelegramSettingsProps {
  config: TelegramChannelConfig;
  adminState: TelegramAdminState | null;
  onChange: (config: TelegramChannelConfig) => void;
  onApprove: (userId: string) => void;
  onReject: (userId: string) => void;
  pendingActionUserId?: string | null;
}

export function TelegramSettings({
  config,
  adminState,
  onChange,
  onApprove,
  onReject,
  pendingActionUserId,
}: TelegramSettingsProps) {
  const [newUser, setNewUser] = useState("");
  const [newChatId, setNewChatId] = useState("");

  const allowedChats = config.allowed_chats ?? {};

  const updateConfig = (partial: Partial<TelegramChannelConfig>) => {
    onChange({ ...config, ...partial });
  };

  const addUser = () => {
    const trimmed = newUser.trim();
    if (!trimmed || config.allowed_users.includes(trimmed)) return;
    updateConfig({ allowed_users: [...config.allowed_users, trimmed] });
    setNewUser("");
  };

  const removeUser = (user: string) => {
    updateConfig({ allowed_users: config.allowed_users.filter((u) => u !== user) });
  };

  const addChat = () => {
    const trimmed = newChatId.trim();
    if (!trimmed || allowedChats[trimmed]) return;
    const newChat: TelegramChatConfig = { allow: true, require_mention: false };
    updateConfig({ allowed_chats: { ...allowedChats, [trimmed]: newChat } });
    setNewChatId("");
  };

  const removeChat = (chatId: string) => {
    const { [chatId]: _, ...rest } = allowedChats;
    updateConfig({ allowed_chats: rest });
  };

  const updateChat = (chatId: string, partial: Partial<TelegramChatConfig>) => {
    const chat = allowedChats[chatId];
    if (!chat) return;
    updateConfig({
      allowed_chats: { ...allowedChats, [chatId]: { ...chat, ...partial } },
    });
  };

  const pendingApprovals = adminState?.pending_approvals ?? [];

  return (
    <div className="card p-5">
      <h3 className="font-display font-semibold text-fg mb-5">Telegram</h3>

      {/* Group Policy */}
      <div className="mb-5">
        <label className="section-label text-[10px] mb-1.5 block">Group Policy</label>
        <select
          value={config.group_policy}
          onChange={(e) =>
            updateConfig({
              group_policy: e.target.value as TelegramChannelConfig["group_policy"],
            })
          }
          className="w-full bg-raised border border-edge rounded-md px-3 py-2 text-sm font-mono text-fg input-focus"
        >
          <option value="allowlist">allowlist</option>
          <option value="open">open</option>
        </select>
      </div>

      {/* DM Policy */}
      <div className="mb-5">
        <label className="section-label text-[10px] mb-1.5 block">DM Policy</label>
        <select
          value={config.dm.policy}
          onChange={(e) =>
            updateConfig({
              dm: { ...config.dm, policy: e.target.value as TelegramChannelConfig["dm"]["policy"] },
            })
          }
          className="w-full bg-raised border border-edge rounded-md px-3 py-2 text-sm font-mono text-fg input-focus"
        >
          <option value="allow">allow</option>
          <option value="deny">deny</option>
        </select>
      </div>

      {/* Allowed Users */}
      <div className="mb-5">
        <label className="section-label text-[10px] mb-1.5 block">Allowed Users</label>
        {config.allowed_users.length > 0 && (
          <div className="mb-2 flex flex-wrap gap-1.5">
            {config.allowed_users.map((user) => (
              <span
                key={user}
                className="inline-flex items-center gap-1.5 badge-accent font-mono text-sm"
              >
                {user}
                <button
                  onClick={() => removeUser(user)}
                  className="text-fg-3 hover:text-error transition-colors"
                  aria-label={`Remove ${user}`}
                >
                  <X size={10} />
                </button>
              </span>
            ))}
          </div>
        )}
        <div className="flex gap-2">
          <input
            type="text"
            value={newUser}
            onChange={(e) => setNewUser(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && addUser()}
            placeholder="User ID..."
            className="flex-1 bg-raised border border-edge rounded-md px-3 py-2 text-sm font-mono text-fg placeholder-fg-3 input-focus"
          />
          <button onClick={addUser} className="btn flex items-center gap-1.5">
            <Plus size={12} />
            Add
          </button>
        </div>
      </div>

      {/* Pending Approvals */}
      {pendingApprovals.length > 0 && (
        <div className="mb-5">
          <label className="section-label text-[10px] mb-2 block">Pending DM Approvals</label>
          <div className="flex flex-col gap-2">
            {pendingApprovals.map((item) => (
              <div
                key={item.user_id}
                className="rounded-md border border-edge bg-raised p-3"
              >
                <div className="flex items-center justify-between gap-3">
                  <div className="min-w-0">
                    <div className="truncate text-sm font-mono text-fg">{item.user_id}</div>
                    <div className="truncate text-xs text-fg-3 mt-0.5">
                      {item.username || "unknown"} &middot; {item.message_count} message
                      {item.message_count !== 1 ? "s" : ""}
                    </div>
                  </div>
                  <div className="flex gap-1.5 shrink-0">
                    <button
                      onClick={() => onApprove(item.user_id)}
                      disabled={pendingActionUserId === item.user_id}
                      className="btn-accent !px-2.5 !py-1 !text-xs flex items-center gap-1"
                    >
                      <Check size={11} />
                      Approve
                    </button>
                    <button
                      onClick={() => onReject(item.user_id)}
                      disabled={pendingActionUserId === item.user_id}
                      className="btn-danger !px-2.5 !py-1 !text-xs flex items-center gap-1"
                    >
                      <Ban size={11} />
                      Reject
                    </button>
                  </div>
                </div>
                {item.preview && (
                  <p className="mt-2 text-xs text-fg-2 line-clamp-2">{item.preview}</p>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Allowed Chats */}
      <div>
        <label className="section-label text-[10px] mb-2 block">Allowed Chats</label>

        {Object.entries(allowedChats).map(([chatId, chat]) => (
          <div
            key={chatId}
            className="mb-2 flex items-center gap-3 rounded-md border border-edge bg-raised px-3 py-2"
          >
            <span className="flex-1 text-sm font-mono text-fg-2">{chatId}</span>
            <label className="flex items-center gap-1.5 cursor-pointer">
              <div
                className={`toggle ${chat.allow ? "active" : ""}`}
                onClick={() => updateChat(chatId, { allow: !chat.allow })}
              />
              <span className="text-xs text-fg-3">Allow</span>
            </label>
            <label className="flex items-center gap-1.5 cursor-pointer">
              <div
                className={`toggle ${chat.require_mention ? "active" : ""}`}
                onClick={() => updateChat(chatId, { require_mention: !chat.require_mention })}
              />
              <span className="text-xs text-fg-3">Mention</span>
            </label>
            <button
              onClick={() => removeChat(chatId)}
              className="text-fg-3 hover:text-error transition-colors"
              aria-label={`Remove chat ${chatId}`}
            >
              <X size={12} />
            </button>
          </div>
        ))}

        <div className="mt-2 flex gap-2">
          <input
            type="text"
            value={newChatId}
            onChange={(e) => setNewChatId(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && addChat()}
            placeholder="Chat ID..."
            className="flex-1 bg-raised border border-edge rounded-md px-3 py-2 text-sm font-mono text-fg placeholder-fg-3 input-focus"
          />
          <button onClick={addChat} className="btn flex items-center gap-1.5">
            <Plus size={12} />
            Add Chat
          </button>
        </div>
      </div>
    </div>
  );
}
