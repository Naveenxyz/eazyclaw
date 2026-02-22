import { useState } from "react";
import { Plus, X, ChevronDown, ChevronRight, Check, Ban } from "lucide-react";
import type {
  DiscordAdminState,
  DiscordChannelConfig,
  DiscordGuildConfig,
  DiscordGuildChannelConfig,
} from "@/types";

interface DiscordSettingsProps {
  config: DiscordChannelConfig;
  adminState: DiscordAdminState | null;
  onChange: (config: DiscordChannelConfig) => void;
  onApprove: (userId: string) => void;
  onReject: (userId: string) => void;
  pendingActionUserId?: string | null;
}

export function DiscordSettings({
  config,
  adminState,
  onChange,
  onApprove,
  onReject,
  pendingActionUserId,
}: DiscordSettingsProps) {
  const [newUser, setNewUser] = useState("");
  const [newGuildId, setNewGuildId] = useState("");
  const [newChannelIds, setNewChannelIds] = useState<Record<string, string>>({});
  const [guildsExpanded, setGuildsExpanded] = useState(true);

  const guilds = config.guilds ?? {};

  const updateConfig = (partial: Partial<DiscordChannelConfig>) => {
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

  const addGuild = () => {
    const trimmed = newGuildId.trim();
    if (!trimmed || guilds[trimmed]) return;
    const newGuild: DiscordGuildConfig = { require_mention: false, channels: {} };
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
    updateConfig({ guilds: { ...guilds, [guildId]: { ...guild, ...partial } } });
  };

  const addChannel = (guildId: string) => {
    const channelId = (newChannelIds[guildId] || "").trim();
    if (!channelId) return;
    const guild = guilds[guildId];
    if (!guild) return;
    const channels = guild.channels ?? {};
    if (channels[channelId]) return;
    const newChannel: DiscordGuildChannelConfig = { allow: true, require_mention: false };
    updateGuild(guildId, { channels: { ...channels, [channelId]: newChannel } });
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
      channels: { ...channels, [channelId]: { ...channels[channelId], ...partial } },
    });
  };

  const pendingApprovals = adminState?.pending_approvals ?? [];

  return (
    <div className="card p-5">
      <h3 className="font-display font-semibold text-fg mb-5">Discord</h3>

      {/* Group Policy */}
      <div className="mb-5">
        <label className="section-label text-[10px] mb-1.5 block">Group Policy</label>
        <select
          value={config.group_policy}
          onChange={(e) =>
            updateConfig({ group_policy: e.target.value as DiscordChannelConfig["group_policy"] })
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
              dm: { ...config.dm, policy: e.target.value as DiscordChannelConfig["dm"]["policy"] },
            })
          }
          className="w-full bg-raised border border-edge rounded-md px-3 py-2 text-sm font-mono text-fg input-focus"
        >
          <option value="allow">allow</option>
          <option value="deny">deny</option>
          <option value="pairing">pairing</option>
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

      {/* Guilds */}
      <div>
        <button
          onClick={() => setGuildsExpanded((prev) => !prev)}
          className="flex items-center gap-1.5 mb-3 group"
        >
          {guildsExpanded ? (
            <ChevronDown size={12} className="text-fg-3" />
          ) : (
            <ChevronRight size={12} className="text-fg-3" />
          )}
          <span className="section-label text-[10px]">Guilds</span>
          <span className="badge-neutral ml-1">{Object.keys(guilds).length}</span>
        </button>

        {guildsExpanded && (
          <>
            {Object.entries(guilds).map(([guildId, guild]) => (
              <div key={guildId} className="mb-4 rounded-md border border-edge bg-raised p-4">
                <div className="mb-3 flex items-center justify-between">
                  <span className="text-sm font-mono font-medium text-fg">
                    Guild: {guildId}
                  </span>
                  <button
                    onClick={() => removeGuild(guildId)}
                    className="text-xs text-error hover:opacity-80 transition-opacity flex items-center gap-1"
                  >
                    <X size={11} />
                    Remove
                  </button>
                </div>

                {/* Require Mention Toggle */}
                <label className="mb-3 flex items-center gap-2 cursor-pointer">
                  <div
                    className={`toggle ${guild.require_mention ? "active" : ""}`}
                    onClick={(e) => {
                      e.preventDefault();
                      updateGuild(guildId, { require_mention: !guild.require_mention });
                    }}
                  />
                  <span className="text-sm text-fg-2">Require Mention</span>
                </label>

                {/* Channels */}
                <div className="ml-2">
                  <label className="section-label text-[10px] mb-2 block">Channels</label>

                  {Object.entries(guild.channels ?? {}).map(([channelId, channel]) => (
                    <div
                      key={channelId}
                      className="mb-2 flex items-center gap-3 rounded-md border border-edge bg-surface px-3 py-2"
                    >
                      <span className="flex-1 text-xs font-mono text-fg-2">{channelId}</span>
                      <label className="flex items-center gap-1.5 cursor-pointer">
                        <div
                          className={`toggle ${channel.allow !== false ? "active" : ""}`}
                          onClick={() =>
                            updateChannel(guildId, channelId, { allow: !(channel.allow ?? true) })
                          }
                        />
                        <span className="text-xs text-fg-3">Allow</span>
                      </label>
                      <label className="flex items-center gap-1.5 cursor-pointer">
                        <div
                          className={`toggle ${channel.require_mention ? "active" : ""}`}
                          onClick={() =>
                            updateChannel(guildId, channelId, {
                              require_mention: !channel.require_mention,
                            })
                          }
                        />
                        <span className="text-xs text-fg-3">Mention</span>
                      </label>
                      <button
                        onClick={() => removeChannel(guildId, channelId)}
                        className="text-fg-3 hover:text-error transition-colors"
                      >
                        <X size={12} />
                      </button>
                    </div>
                  ))}

                  <div className="mt-2 flex gap-2">
                    <input
                      type="text"
                      value={newChannelIds[guildId] || ""}
                      onChange={(e) =>
                        setNewChannelIds((prev) => ({ ...prev, [guildId]: e.target.value }))
                      }
                      onKeyDown={(e) => e.key === "Enter" && addChannel(guildId)}
                      placeholder="Channel ID..."
                      className="flex-1 bg-raised border border-edge rounded-md px-3 py-1.5 text-xs font-mono text-fg placeholder-fg-3 input-focus"
                    />
                    <button
                      onClick={() => addChannel(guildId)}
                      className="btn !text-xs !px-3 !py-1.5 flex items-center gap-1"
                    >
                      <Plus size={11} />
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
                className="flex-1 bg-raised border border-edge rounded-md px-3 py-2 text-sm font-mono text-fg placeholder-fg-3 input-focus"
              />
              <button onClick={addGuild} className="btn flex items-center gap-1.5">
                <Plus size={12} />
                Add Guild
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
