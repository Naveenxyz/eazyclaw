import { useState, useEffect, useCallback } from "react";
import { AlertTriangle } from "lucide-react";
import {
  getConfig,
  getDiscordAdminState,
  getTelegramAdminState,
  getWhatsAppAdminState,
  getGoogleAuthStatus,
  getGoogleAuthURL,
  putConfig,
  updateDiscordApproval,
  updateTelegramApproval,
  updateWhatsAppApproval,
  updateWhatsAppSettings,
  disconnectWhatsApp,
  disconnectGoogle,
} from "@/lib/api";
import { DiscordSettings } from "@/components/settings/DiscordSettings";
import { TelegramSettings } from "@/components/settings/TelegramSettings";
import { WhatsAppSettings } from "@/components/settings/WhatsAppSettings";
import { GoogleSettings } from "@/components/settings/GoogleSettings";
import type {
  ChannelConfig,
  DiscordAdminState,
  TelegramAdminState,
  WhatsAppAdminState,
  GoogleAuthStatus,
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
  const [whatsappAdmin, setWhatsappAdmin] = useState<WhatsAppAdminState | null>(null);
  const [googleAuth, setGoogleAuth] = useState<GoogleAuthStatus | null>(null);
  const [pendingDiscordApprovalUserId, setPendingDiscordApprovalUserId] = useState<string | null>(null);
  const [pendingTelegramApprovalUserId, setPendingTelegramApprovalUserId] = useState<string | null>(null);
  const [pendingWhatsAppApprovalUserId, setPendingWhatsAppApprovalUserId] = useState<string | null>(null);

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
    Promise.all([
      getConfig(),
      getDiscordAdminState(),
      getTelegramAdminState(),
      getWhatsAppAdminState().catch(() => null),
      getGoogleAuthStatus().catch(() => null),
    ])
      .then(([cfg, dcAdmin, tgAdmin, waAdmin, gAuth]) => {
        setConfig(normalizeConfig(cfg));
        setDiscordAdmin(dcAdmin);
        setTelegramAdmin(tgAdmin);
        if (waAdmin) setWhatsappAdmin(waAdmin);
        if (gAuth) setGoogleAuth(gAuth);
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
      Promise.all([
        getDiscordAdminState(),
        getTelegramAdminState(),
        getWhatsAppAdminState().catch(() => null),
        getGoogleAuthStatus().catch(() => null),
      ])
        .then(([dc, tg, wa, gAuth]) => {
          setDiscordAdmin(dc);
          setTelegramAdmin(tg);
          if (wa) setWhatsappAdmin(wa);
          if (gAuth) setGoogleAuth(gAuth);
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

  const handleWhatsAppApprovalAction = useCallback(
    async (action: "approve" | "reject", userId: string) => {
      setPendingWhatsAppApprovalUserId(userId);
      try {
        const state = await updateWhatsAppApproval(action, userId);
        setWhatsappAdmin(state);
      } catch (err) {
        setSaveStatus(err instanceof Error ? err.message : "Failed to update approval.");
      } finally {
        setPendingWhatsAppApprovalUserId(null);
      }
    },
    []
  );

  const handleWhatsAppDisconnect = useCallback(async () => {
    try {
      await disconnectWhatsApp();
      setWhatsappAdmin((prev) =>
        prev ? { ...prev, status: "disconnected", phone_number: "", qr_code: "" } : prev
      );
    } catch (err) {
      setSaveStatus(err instanceof Error ? err.message : "Failed to disconnect WhatsApp.");
    }
  }, []);

  const handleWhatsAppAddUser = useCallback(
    async (userId: string) => {
      const current = whatsappAdmin?.allowed_users ?? [];
      if (current.includes(userId)) return;
      const updated = [...current, userId];
      try {
        await updateWhatsAppSettings({ allowed_users: updated });
        setWhatsappAdmin((prev) => (prev ? { ...prev, allowed_users: updated } : prev));
      } catch (err) {
        setSaveStatus(err instanceof Error ? err.message : "Failed to add user.");
      }
    },
    [whatsappAdmin]
  );

  const handleWhatsAppRemoveUser = useCallback(
    async (userId: string) => {
      const current = whatsappAdmin?.allowed_users ?? [];
      const updated = current.filter((u) => u !== userId);
      try {
        await updateWhatsAppSettings({ allowed_users: updated });
        setWhatsappAdmin((prev) => (prev ? { ...prev, allowed_users: updated } : prev));
      } catch (err) {
        setSaveStatus(err instanceof Error ? err.message : "Failed to remove user.");
      }
    },
    [whatsappAdmin]
  );

  const handleWhatsAppPolicyChange = useCallback(
    async (field: "group_policy" | "dm_policy", value: string) => {
      try {
        await updateWhatsAppSettings({ [field]: value });
        setWhatsappAdmin((prev) => (prev ? { ...prev, [field]: value } : prev));
      } catch (err) {
        setSaveStatus(err instanceof Error ? err.message : "Failed to update policy.");
      }
    },
    []
  );

  const handleGoogleConnect = useCallback(async () => {
    try {
      const { url } = await getGoogleAuthURL();
      window.open(url, "_blank");
    } catch (err) {
      setSaveStatus(err instanceof Error ? err.message : "Failed to get Google auth URL.");
    }
  }, []);

  const handleGoogleDisconnect = useCallback(async () => {
    try {
      await disconnectGoogle();
      setGoogleAuth({ authenticated: false });
    } catch (err) {
      setSaveStatus(err instanceof Error ? err.message : "Failed to disconnect Google.");
    }
  }, []);

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

        <WhatsAppSettings
          adminState={whatsappAdmin}
          onApprove={(userId) => handleWhatsAppApprovalAction("approve", userId)}
          onReject={(userId) => handleWhatsAppApprovalAction("reject", userId)}
          onDisconnect={handleWhatsAppDisconnect}
          onAddUser={handleWhatsAppAddUser}
          onRemoveUser={handleWhatsAppRemoveUser}
          onPolicyChange={handleWhatsAppPolicyChange}
          pendingActionUserId={pendingWhatsAppApprovalUserId}
        />

        <GoogleSettings
          authStatus={googleAuth}
          onConnect={handleGoogleConnect}
          onDisconnect={handleGoogleDisconnect}
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
