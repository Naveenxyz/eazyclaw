import { useState } from "react";
import type {
  DiscordChannelConfig,
  DiscordGuildConfig,
  DiscordGuildChannelConfig,
} from "@/types";

interface DiscordSettingsProps {
  config: DiscordChannelConfig;
  onChange: (config: DiscordChannelConfig) => void;
}

export function DiscordSettings({ config, onChange }: DiscordSettingsProps) {
  const [newUser, setNewUser] = useState("");
  const [newGuildId, setNewGuildId] = useState("");
  const [newChannelIds, setNewChannelIds] = useState<Record<string, string>>({});

  const guilds = config.guilds ?? {};

  const updateConfig = (partial: Partial<DiscordChannelConfig>) => {
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

  const addGuild = () => {
    const trimmed = newGuildId.trim();
    if (!trimmed) return;
    if (guilds[trimmed]) return;
    const newGuild: DiscordGuildConfig = {
      require_mention: false,
      channels: {},
    };
    updateConfig({ guilds: { ...guilds, [trimmed]: newGuild } });
    setNewGuildId("");
  };

  const removeGuild = (guildId: string) => {
    const { [guildId]: _, ...rest } = guilds;
    updateConfig({ guilds: rest });
    const { [guildId]: __, ...restChannelIds } = newChannelIds;
    setNewChannelIds(restChannelIds);
  };

  const updateGuild = (guildId: string, partial: Partial<DiscordGuildConfig>) => {
    const guild = guilds[guildId];
    if (!guild) return;
    updateConfig({
      guilds: {
        ...guilds,
        [guildId]: { ...guild, ...partial },
      },
    });
  };

  const addChannel = (guildId: string) => {
    const channelId = (newChannelIds[guildId] || "").trim();
    if (!channelId) return;
    const guild = guilds[guildId];
    if (!guild) return;
    const channels = guild.channels ?? {};
    if (channels[channelId]) return;
    const newChannel: DiscordGuildChannelConfig = {
      allow: true,
      require_mention: false,
    };
    updateGuild(guildId, {
      channels: { ...channels, [channelId]: newChannel },
    });
    setNewChannelIds((prev) => ({ ...prev, [guildId]: "" }));
  };

  const removeChannel = (guildId: string, channelId: string) => {
    const guild = guilds[guildId];
    if (!guild) return;
    const channels = guild.channels ?? {};
    const { [channelId]: _, ...rest } = channels;
    updateGuild(guildId, { channels: rest });
  };

  const updateChannel = (
    guildId: string,
    channelId: string,
    partial: Partial<DiscordGuildChannelConfig>
  ) => {
    const guild = guilds[guildId];
    if (!guild) return;
    const channels = guild.channels ?? {};
    updateGuild(guildId, {
      channels: {
        ...channels,
        [channelId]: { ...channels[channelId], ...partial },
      },
    });
  };

  return (
    <div className="rounded-lg border border-zinc-800 bg-zinc-900 p-5">
      <h2 className="mb-4 text-lg font-semibold text-violet-400">Discord</h2>

      {/* Group Policy */}
      <div className="mb-4">
        <label className="mb-1 block text-xs font-medium uppercase tracking-wider text-zinc-500">
          Group Policy
        </label>
        <select
          value={config.group_policy}
          onChange={(e) => updateConfig({ group_policy: e.target.value as DiscordChannelConfig["group_policy"] })}
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
            updateConfig({ dm: { ...config.dm, policy: e.target.value as DiscordChannelConfig["dm"]["policy"] } })
          }
          className="w-full rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2 text-sm text-zinc-100"
        >
          <option value="allow">allow</option>
          <option value="deny">deny</option>
          <option value="pairing">pairing</option>
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

      {/* Guilds */}
      <div>
        <label className="mb-2 block text-xs font-medium uppercase tracking-wider text-zinc-500">
          Guilds
        </label>

        {Object.entries(guilds).map(([guildId, guild]) => (
          <div
            key={guildId}
            className="mb-4 rounded-md border border-zinc-800 bg-zinc-950 p-4"
          >
            <div className="mb-3 flex items-center justify-between">
              <span className="text-sm font-medium text-zinc-200">
                Guild: {guildId}
              </span>
              <button
                onClick={() => removeGuild(guildId)}
                className="text-xs text-red-400 hover:text-red-300"
              >
                Remove
              </button>
            </div>

            {/* Require Mention Toggle */}
            <label className="mb-3 flex items-center gap-2">
              <input
                type="checkbox"
                checked={guild.require_mention}
                onChange={(e) =>
                  updateGuild(guildId, { require_mention: e.target.checked })
                }
                className="rounded border-zinc-700"
              />
              <span className="text-sm text-zinc-300">Require Mention</span>
            </label>

            {/* Channels */}
            <div className="ml-2">
              <label className="mb-2 block text-xs font-medium uppercase tracking-wider text-zinc-600">
                Channels
              </label>

              {Object.entries(guild.channels ?? {}).map(([channelId, channel]) => (
                <div
                  key={channelId}
                  className="mb-2 flex items-center gap-3 rounded-md border border-zinc-800 bg-zinc-900 px-3 py-2"
                >
                  <span className="flex-1 text-xs text-zinc-300">
                    {channelId}
                  </span>
                  <label className="flex items-center gap-1">
                    <input
                      type="checkbox"
                      checked={channel.allow ?? true}
                      onChange={(e) =>
                        updateChannel(guildId, channelId, {
                          allow: e.target.checked,
                        })
                      }
                      className="rounded border-zinc-700"
                    />
                    <span className="text-xs text-zinc-400">Allow</span>
                  </label>
                  <label className="flex items-center gap-1">
                    <input
                      type="checkbox"
                      checked={channel.require_mention ?? false}
                      onChange={(e) =>
                        updateChannel(guildId, channelId, {
                          require_mention: e.target.checked,
                        })
                      }
                      className="rounded border-zinc-700"
                    />
                    <span className="text-xs text-zinc-400">
                      Require Mention
                    </span>
                  </label>
                  <button
                    onClick={() => removeChannel(guildId, channelId)}
                    className="text-xs text-red-400 hover:text-red-300"
                  >
                    Remove
                  </button>
                </div>
              ))}

              <div className="mt-2 flex gap-2">
                <input
                  type="text"
                  value={newChannelIds[guildId] || ""}
                  onChange={(e) =>
                    setNewChannelIds((prev) => ({
                      ...prev,
                      [guildId]: e.target.value,
                    }))
                  }
                  onKeyDown={(e) => e.key === "Enter" && addChannel(guildId)}
                  placeholder="Channel ID..."
                  className="flex-1 rounded-md border border-zinc-800 bg-zinc-950 px-3 py-1.5 text-xs text-zinc-100 placeholder-zinc-600"
                />
                <button
                  onClick={() => addChannel(guildId)}
                  className="rounded-md bg-violet-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-violet-500"
                >
                  Add
                </button>
              </div>
            </div>
          </div>
        ))}

        <div className="flex gap-2">
          <input
            type="text"
            value={newGuildId}
            onChange={(e) => setNewGuildId(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && addGuild()}
            placeholder="Guild ID..."
            className="flex-1 rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 placeholder-zinc-600"
          />
          <button
            onClick={addGuild}
            className="rounded-md bg-violet-600 px-3 py-2 text-sm font-medium text-white hover:bg-violet-500"
          >
            Add Guild
          </button>
        </div>
      </div>
    </div>
  );
}
