import { useState, useEffect, useCallback } from "react";
import { getConfig, putConfig } from "@/lib/api";
import { DiscordSettings } from "@/components/settings/DiscordSettings";
import { TelegramSettings } from "@/components/settings/TelegramSettings";
import type { ChannelConfig, DiscordChannelConfig, TelegramChannelConfig } from "@/types";

export default function SettingsTab() {
  const [config, setConfig] = useState<ChannelConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saveStatus, setSaveStatus] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    getConfig()
      .then((data) => {
        setConfig(data);
        setError(null);
      })
      .catch((err) => {
        setError(
          err instanceof Error ? err.message : "Failed to load configuration"
        );
      })
      .finally(() => {
        setLoading(false);
      });
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

  const handleSave = async () => {
    if (!config) return;
    setSaving(true);
    setSaveStatus(null);
    try {
      await putConfig(config);
      setSaveStatus("Configuration saved successfully.");
    } catch (err) {
      setSaveStatus(
        err instanceof Error ? err.message : "Failed to save configuration."
      );
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <p className="text-sm text-zinc-500">Loading configuration...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex h-full items-center justify-center">
        <p className="text-sm text-red-400">{error}</p>
      </div>
    );
  }

  if (!config) return null;

  return (
    <div className="mx-auto max-w-4xl p-6">
      <div className="flex flex-col gap-6">
        <DiscordSettings
          config={config.discord}
          onChange={handleDiscordChange}
        />
        <TelegramSettings
          config={config.telegram}
          onChange={handleTelegramChange}
        />

        <div className="rounded-lg border border-yellow-800/50 bg-yellow-950/30 p-4">
          <p className="text-sm text-yellow-400">
            Changes require container restart to take effect.
          </p>
        </div>

        {saveStatus && (
          <div
            className={`rounded-lg border p-4 text-sm ${
              saveStatus.includes("successfully")
                ? "border-emerald-800/50 bg-emerald-950/30 text-emerald-400"
                : "border-red-800/50 bg-red-950/30 text-red-400"
            }`}
          >
            {saveStatus}
          </div>
        )}

        <button
          onClick={handleSave}
          disabled={saving}
          className="self-start rounded-lg bg-violet-600 px-6 py-2 text-sm font-medium text-white transition-colors hover:bg-violet-500 disabled:opacity-50"
        >
          {saving ? "Saving..." : "Save Configuration"}
        </button>
      </div>
    </div>
  );
}
