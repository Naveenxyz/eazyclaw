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
    <div className="glass-card rounded-lg border border-violet-500/10 bg-[#0f1117] p-5">
      <h2 className="mb-4 text-lg font-semibold text-slate-200">Discord</h2>

      {/* Group Policy */}
      <div className="mb-4">
        <label className="mb-1 block text-sm font-medium text-slate-400">
          Group Policy
        </label>
        <select
          value={config.group_policy}
          onChange={(e) => updateConfig({ group_policy: e.target.value as DiscordChannelConfig["group_policy"] })}
          className="w-full rounded-lg border border-violet-500/10 bg-[#0f1117] px-3 py-2 text-sm text-slate-200 focus:border-violet-500/30 focus:ring-1 focus:ring-violet-500/20"
        >
          <option value="allowlist">allowlist</option>
          <option value="open">open</option>
        </select>
      </div>

      {/* DM Policy */}
      <div className="mb-4">
        <label className="mb-1 block text-sm font-medium text-slate-400">
          DM Policy
        </label>
        <select
          value={config.dm.policy}
          onChange={(e) =>
            updateConfig({ dm: { ...config.dm, policy: e.target.value as DiscordChannelConfig["dm"]["policy"] } })
          }
          className="w-full rounded-lg border border-violet-500/10 bg-[#0f1117] px-3 py-2 text-sm text-slate-200 focus:border-violet-500/30 focus:ring-1 focus:ring-violet-500/20"
        >
          <option value="allow">allow</option>
          <option value="deny">deny</option>
          <option value="pairing">pairing</option>
        </select>
      </div>

      {/* Allowed Users */}
      <div className="mb-4">
        <label className="mb-1 block text-sm font-medium text-slate-400">
          Allowed Users
        </label>
        <div className="mb-2 flex flex-wrap gap-2">
          {config.allowed_users.map((user) => (
            <span
              key={user}
              className="inline-flex items-center gap-1 rounded-full bg-violet-500/10 border border-violet-500/20 px-3 py-1 text-xs text-slate-300"
            >
              {user}
              <button
                onClick={() => removeUser(user)}
                className="ml-1 text-slate-500 hover:text-red-400"
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
            className="flex-1 rounded-lg border border-violet-500/10 bg-[#0f1117] px-3 py-2 text-sm text-slate-200 placeholder-slate-600 focus:border-violet-500/30 focus:ring-1 focus:ring-violet-500/20"
          />
          <button
            onClick={addUser}
            className="rounded-lg bg-violet-600 px-4 py-2 text-sm font-medium text-white hover:bg-violet-500"
          >
            Add
          </button>
        </div>
      </div>

      {/* Guilds */}
      <div>
        <label className="mb-2 block text-sm font-medium text-slate-400">
          Guilds
        </label>

        {Object.entries(guilds).map(([guildId, guild]) => (
          <div
            key={guildId}
            className="mb-4 rounded-lg border border-violet-500/10 bg-[#151720] p-4"
          >
            <div className="mb-3 flex items-center justify-between">
              <span className="text-sm font-mono font-medium text-slate-200">
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
                className="rounded border-violet-500/20"
              />
              <span className="text-sm text-slate-300">Require Mention</span>
            </label>

            {/* Channels */}
            <div className="ml-2">
              <label className="mb-2 block text-xs font-medium text-slate-500">
                Channels
              </label>

              {Object.entries(guild.channels ?? {}).map(([channelId, channel]) => (
                <div
                  key={channelId}
                  className="mb-2 flex items-center gap-3 rounded-lg border border-white/5 bg-[#0f1117] px-3 py-2"
                >
                  <span className="flex-1 text-xs font-mono text-slate-300">
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
                      className="rounded border-violet-500/20"
                    />
                    <span className="text-xs text-slate-400">Allow</span>
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
                      className="rounded border-violet-500/20"
                    />
                    <span className="text-xs text-slate-400">
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
                  className="flex-1 rounded-lg border border-violet-500/10 bg-[#0f1117] px-3 py-1.5 text-xs text-slate-200 placeholder-slate-600 focus:border-violet-500/30 focus:ring-1 focus:ring-violet-500/20"
                />
                <button
                  onClick={() => addChannel(guildId)}
                  className="rounded-lg bg-violet-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-violet-500"
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
            className="flex-1 rounded-lg border border-violet-500/10 bg-[#0f1117] px-3 py-2 text-sm text-slate-200 placeholder-slate-600 focus:border-violet-500/30 focus:ring-1 focus:ring-violet-500/20"
          />
          <button
            onClick={addGuild}
            className="rounded-lg bg-violet-600 px-4 py-2 text-sm font-medium text-white hover:bg-violet-500"
          >
            Add Guild
          </button>
        </div>
      </div>
    </div>
  );
}
