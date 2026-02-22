import { useState, useEffect } from "react";
import { getStatus, getSessions } from "@/lib/api";
import { StatusCard } from "@/components/status/StatusCard";

export default function StatusTab() {
  const [providers, setProviders] = useState<string[]>([]);
  const [channels, setChannels] = useState<string[]>([]);
  const [sessionCount, setSessionCount] = useState<number>(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([getStatus(), getSessions()])
      .then(([statusData, sessions]) => {
        setProviders(statusData.providers || []);
        setChannels(statusData.channels || []);
        setSessionCount(Array.isArray(sessions) ? sessions.length : 0);
        setError(null);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load status");
      })
      .finally(() => {
        setLoading(false);
      });
  }, []);

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <p className="text-sm text-zinc-500">Loading status...</p>
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

  return (
    <div className="p-6">
      {providers.length > 0 && (
        <section className="mb-8">
          <h2 className="mb-4 text-sm font-semibold uppercase tracking-wider text-zinc-400">
            Providers
          </h2>
          <div
            className="grid gap-4"
            style={{
              gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))",
            }}
          >
            {providers.map((name) => (
              <StatusCard key={name} name={name} detail="active" />
            ))}
          </div>
        </section>
      )}

      {channels.length > 0 && (
        <section className="mb-8">
          <h2 className="mb-4 text-sm font-semibold uppercase tracking-wider text-zinc-400">
            Channels
          </h2>
          <div
            className="grid gap-4"
            style={{
              gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))",
            }}
          >
            {channels.map((name) => (
              <StatusCard key={name} name={name} detail="active" />
            ))}
          </div>
        </section>
      )}

      <section>
        <h2 className="mb-4 text-sm font-semibold uppercase tracking-wider text-zinc-400">
          Sessions
        </h2>
        <div className="rounded-lg border border-zinc-800 bg-zinc-900 p-4">
          <span className="text-2xl font-bold text-zinc-100">
            {sessionCount}
          </span>
          <span className="ml-2 text-sm text-zinc-400">active sessions</span>
        </div>
      </section>
    </div>
  );
}
