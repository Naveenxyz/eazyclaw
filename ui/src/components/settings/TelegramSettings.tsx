import { useState } from "react";
import type { TelegramChannelConfig, TelegramChatConfig } from "@/types";

interface TelegramSettingsProps {
  config: TelegramChannelConfig;
  onChange: (config: TelegramChannelConfig) => void;
}

export function TelegramSettings({ config, onChange }: TelegramSettingsProps) {
  const [newUser, setNewUser] = useState("");
  const [newChatId, setNewChatId] = useState("");

  const allowedChats = config.allowed_chats ?? {};

  const updateConfig = (partial: Partial<TelegramChannelConfig>) => {
    onChange({ ...config, ...partial });
  };

  const addUser = () => {
    const trimmed = newUser.trim();
    if (!trimmed) return;
    if (config.allowed_users.includes(trimmed)) return;
    updateConfig({ allowed_users: [...config.allowed_users, trimmed] });
    setNewUser("");
  };

  const removeUser = (user: string) => {
    updateConfig({
      allowed_users: config.allowed_users.filter((u) => u !== user),
    });
  };

  const addChat = () => {
    const trimmed = newChatId.trim();
    if (!trimmed) return;
    if (allowedChats[trimmed]) return;
    const newChat: TelegramChatConfig = {
      allow: true,
      require_mention: false,
    };
    updateConfig({
      allowed_chats: { ...allowedChats, [trimmed]: newChat },
    });
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
      allowed_chats: {
        ...allowedChats,
        [chatId]: { ...chat, ...partial },
      },
    });
  };

  return (
    <div className="rounded-lg border border-zinc-800 bg-zinc-900 p-5">
      <h2 className="mb-4 text-lg font-semibold text-violet-400">Telegram</h2>

      {/* Group Policy */}
      <div className="mb-4">
        <label className="mb-1 block text-xs font-medium uppercase tracking-wider text-zinc-500">
          Group Policy
        </label>
        <select
          value={config.group_policy}
          onChange={(e) => updateConfig({ group_policy: e.target.value as TelegramChannelConfig["group_policy"] })}
          className="w-full rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2 text-sm text-zinc-100"
        >
          <option value="allowlist">allowlist</option>
          <option value="open">open</option>
        </select>
      </div>

      {/* DM Policy */}
      <div className="mb-4">
        <label className="mb-1 block text-xs font-medium uppercase tracking-wider text-zinc-500">
          DM Policy
        </label>
        <select
          value={config.dm.policy}
          onChange={(e) =>
            updateConfig({ dm: { ...config.dm, policy: e.target.value as TelegramChannelConfig["dm"]["policy"] } })
          }
          className="w-full rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2 text-sm text-zinc-100"
        >
          <option value="allow">allow</option>
          <option value="deny">deny</option>
        </select>
      </div>

      {/* Allowed Users */}
      <div className="mb-4">
        <label className="mb-1 block text-xs font-medium uppercase tracking-wider text-zinc-500">
          Allowed Users
        </label>
        <div className="mb-2 flex flex-wrap gap-2">
          {config.allowed_users.map((user) => (
            <span
              key={user}
              className="inline-flex items-center gap-1 rounded-full bg-zinc-800 px-3 py-1 text-xs text-zinc-300"
            >
              {user}
              <button
                onClick={() => removeUser(user)}
                className="ml-1 text-zinc-500 hover:text-red-400"
              >
                x
              </button>
            </span>
          ))}
        </div>
        <div className="flex gap-2">
          <input
            type="text"
            value={newUser}
            onChange={(e) => setNewUser(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && addUser()}
            placeholder="Add user..."
            className="flex-1 rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 placeholder-zinc-600"
          />
          <button
            onClick={addUser}
            className="rounded-md bg-violet-600 px-3 py-2 text-sm font-medium text-white hover:bg-violet-500"
          >
            Add
          </button>
        </div>
      </div>

      {/* Allowed Chats */}
      <div>
        <label className="mb-2 block text-xs font-medium uppercase tracking-wider text-zinc-500">
          Allowed Chats
        </label>

        {Object.entries(allowedChats).map(([chatId, chat]) => (
          <div
            key={chatId}
            className="mb-2 flex items-center gap-3 rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2"
          >
            <span className="flex-1 text-sm text-zinc-300">{chatId}</span>
            <label className="flex items-center gap-1">
              <input
                type="checkbox"
                checked={chat.allow}
                onChange={(e) =>
                  updateChat(chatId, { allow: e.target.checked })
                }
                className="rounded border-zinc-700"
              />
              <span className="text-xs text-zinc-400">Allow</span>
            </label>
            <label className="flex items-center gap-1">
              <input
                type="checkbox"
                checked={chat.require_mention}
                onChange={(e) =>
                  updateChat(chatId, { require_mention: e.target.checked })
                }
                className="rounded border-zinc-700"
              />
              <span className="text-xs text-zinc-400">Require Mention</span>
            </label>
            <button
              onClick={() => removeChat(chatId)}
              className="text-xs text-red-400 hover:text-red-300"
            >
              Remove
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
            className="flex-1 rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 placeholder-zinc-600"
          />
          <button
            onClick={addChat}
            className="rounded-md bg-violet-600 px-3 py-2 text-sm font-medium text-white hover:bg-violet-500"
          >
            Add Chat
          </button>
        </div>
      </div>
    </div>
  );
}
