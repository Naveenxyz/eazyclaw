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
      <div className="flex h-full items-center justify-center bg-[#08090d]">
        <p className="text-sm font-mono text-slate-500">Loading status...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex h-full items-center justify-center bg-[#08090d]">
        <p className="text-sm text-red-400">{error}</p>
      </div>
    );
  }

  return (
    <div className="p-6 bg-[#08090d]">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
        {providers.length > 0 && (
          <section>
            <h2 className="mb-4 text-sm font-mono font-semibold uppercase tracking-wider text-slate-400">
              Providers
            </h2>
            <div className="flex flex-col gap-4">
              {providers.map((name) => (
                <StatusCard key={name} name={name} detail="active" />
              ))}
            </div>
          </section>
        )}

        {channels.length > 0 && (
          <section>
            <h2 className="mb-4 text-sm font-mono font-semibold uppercase tracking-wider text-slate-400">
              Channels
            </h2>
            <div className="flex flex-col gap-4">
              {channels.map((name) => (
                <StatusCard key={name} name={name} detail="active" />
              ))}
            </div>
          </section>
        )}
      </div>

      <section className="mt-8">
        <h2 className="mb-4 text-sm font-mono font-semibold uppercase tracking-wider text-slate-400">
          Sessions
        </h2>
        <div className="glass-card rounded-lg border border-violet-500/10 bg-[#0f1117] p-4">
          <span className="text-2xl font-mono font-bold text-slate-200">
            {sessionCount}
          </span>
          <span className="ml-2 text-sm text-slate-400">active sessions</span>
        </div>
      </section>
    </div>
  );
}
