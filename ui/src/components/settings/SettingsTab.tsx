import { useState, useEffect, useCallback } from "react";
import { AlertTriangle } from "lucide-react";
import {
  getConfig,
  getDiscordAdminState,
  getTelegramAdminState,
  putConfig,
  updateDiscordApproval,
  updateTelegramApproval,
} from "@/lib/api";
import { DiscordSettings } from "@/components/settings/DiscordSettings";
import { TelegramSettings } from "@/components/settings/TelegramSettings";
import type {
  ChannelConfig,
  DiscordAdminState,
  TelegramAdminState,
  DiscordChannelConfig,
  TelegramChannelConfig,
} from "@/types";

export default function SettingsTab() {
  const [config, setConfig] = useState<ChannelConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saveStatus, setSaveStatus] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [discordAdmin, setDiscordAdmin] = useState<DiscordAdminState | null>(null);
  const [telegramAdmin, setTelegramAdmin] = useState<TelegramAdminState | null>(null);
  const [pendingDiscordApprovalUserId, setPendingDiscordApprovalUserId] = useState<string | null>(null);
  const [pendingTelegramApprovalUserId, setPendingTelegramApprovalUserId] = useState<string | null>(null);

  const normalizeConfig = (cfg: ChannelConfig): ChannelConfig => ({
    ...cfg,
    discord: {
      ...cfg.discord,
      allowed_users: cfg.discord.allowed_users ?? [],
    },
    telegram: {
      ...cfg.telegram,
      allowed_users: cfg.telegram.allowed_users ?? [],
    },
  });

  useEffect(() => {
    Promise.all([getConfig(), getDiscordAdminState(), getTelegramAdminState()])
      .then(([cfg, dcAdmin, tgAdmin]) => {
        setConfig(normalizeConfig(cfg));
        setDiscordAdmin(dcAdmin);
        setTelegramAdmin(tgAdmin);
        setError(null);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load configuration");
      })
      .finally(() => {
        setLoading(false);
      });
  }, []);

  useEffect(() => {
    const timer = setInterval(() => {
      Promise.all([getDiscordAdminState(), getTelegramAdminState()])
        .then(([dc, tg]) => {
          setDiscordAdmin(dc);
          setTelegramAdmin(tg);
        })
        .catch(() => {});
    }, 4000);
    return () => clearInterval(timer);
  }, []);

  const handleDiscordChange = useCallback(
    (discord: DiscordChannelConfig) => {
      setConfig((prev) => (prev ? { ...prev, discord } : prev));
    },
    []
  );

  const handleTelegramChange = useCallback(
    (telegram: TelegramChannelConfig) => {
      setConfig((prev) => (prev ? { ...prev, telegram } : prev));
    },
    []
  );

  const handleDiscordApprovalAction = useCallback(
    async (action: "approve" | "reject", userId: string) => {
      setPendingDiscordApprovalUserId(userId);
      try {
        const state = await updateDiscordApproval(action, userId);
        setDiscordAdmin(state);
        if (action === "approve") {
          setConfig((prev) => {
            if (!prev) return prev;
            if (prev.discord.allowed_users.includes(userId)) return prev;
            return {
              ...prev,
              discord: {
                ...prev.discord,
                allowed_users: [...prev.discord.allowed_users, userId],
              },
            };
          });
        }
      } catch (err) {
        setSaveStatus(err instanceof Error ? err.message : "Failed to update approval.");
      } finally {
        setPendingDiscordApprovalUserId(null);
      }
    },
    []
  );

  const handleTelegramApprovalAction = useCallback(
    async (action: "approve" | "reject", userId: string) => {
      setPendingTelegramApprovalUserId(userId);
      try {
        const state = await updateTelegramApproval(action, userId);
        setTelegramAdmin(state);
        if (action === "approve") {
          setConfig((prev) => {
            if (!prev) return prev;
            if (prev.telegram.allowed_users.includes(userId)) return prev;
            return {
              ...prev,
              telegram: {
                ...prev.telegram,
                allowed_users: [...prev.telegram.allowed_users, userId],
              },
            };
          });
        }
      } catch (err) {
        setSaveStatus(err instanceof Error ? err.message : "Failed to update approval.");
      } finally {
        setPendingTelegramApprovalUserId(null);
      }
    },
    []
  );

  const handleSave = async () => {
    if (!config) return;
    setSaving(true);
    setSaveStatus(null);
    try {
      await putConfig(config);
      setSaveStatus("Configuration saved successfully.");
    } catch (err) {
      setSaveStatus(err instanceof Error ? err.message : "Failed to save configuration.");
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center bg-base">
        <div className="flex items-center gap-2">
          <div className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-accent border-t-transparent" />
          <p className="text-xs font-mono text-fg-3">Loading configuration...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex h-full items-center justify-center bg-base">
        <p className="text-sm text-error">{error}</p>
      </div>
    );
  }

  if (!config) return null;

  return (
    <div className="mx-auto max-w-4xl p-6 bg-base min-h-full">
      <h2 className="section-label mb-6">Configuration</h2>

      <div className="flex flex-col gap-6">
        <DiscordSettings
          config={config.discord}
          adminState={discordAdmin}
          onChange={handleDiscordChange}
          onApprove={(userId) => handleDiscordApprovalAction("approve", userId)}
          onReject={(userId) => handleDiscordApprovalAction("reject", userId)}
          pendingActionUserId={pendingDiscordApprovalUserId}
        />

        <TelegramSettings
          config={config.telegram}
          adminState={telegramAdmin}
          onChange={handleTelegramChange}
          onApprove={(userId) => handleTelegramApprovalAction("approve", userId)}
          onReject={(userId) => handleTelegramApprovalAction("reject", userId)}
          pendingActionUserId={pendingTelegramApprovalUserId}
        />

        {/* Restart warning banner */}
        <div className="card flex items-start gap-3 p-4 border-warning/20 bg-warning/[0.04]">
          <AlertTriangle size={16} className="text-warning shrink-0 mt-0.5" />
          <p className="text-sm text-warning">
            Changes require container restart to take effect.
          </p>
        </div>

        {/* Save status */}
        {saveStatus && (
          <div
            className={`card p-4 text-sm ${
              saveStatus.includes("successfully")
                ? "text-accent border-accent/20 bg-accent-dim"
                : "text-error border-error/20 bg-error/[0.04]"
            }`}
          >
            {saveStatus}
          </div>
        )}

        {/* Save button */}
        <button
          onClick={handleSave}
          disabled={saving}
          className="btn-accent self-start"
        >
          {saving ? "Saving..." : "Save Configuration"}
        </button>
      </div>
    </div>
  );
}
