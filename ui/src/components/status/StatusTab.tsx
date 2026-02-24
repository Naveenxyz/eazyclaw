import { useState, useEffect, useCallback } from "react";
import { getStatus, getSessions, getHeartbeatStatus } from "@/lib/api";
import { StatusCard } from "@/components/status/StatusCard";
import type { HeartbeatStatus } from "@/types";

const GO_ZERO_TIME = "0001-01-01T00:00:00Z";

function formatRelativeTime(iso: string): string {
  if (!iso || iso === GO_ZERO_TIME) return "Never";
  const now = Date.now();
  const then = new Date(iso).getTime();
  const diff = now - then;
  const abs = Math.abs(diff);
  const future = diff < 0;

  if (abs < 60_000) return future ? "in <1m" : "<1m ago";
  if (abs < 3_600_000) {
    const m = Math.floor(abs / 60_000);
    return future ? `in ${m}m` : `${m}m ago`;
  }
  if (abs < 86_400_000) {
    const h = Math.floor(abs / 3_600_000);
    return future ? `in ${h}h` : `${h}h ago`;
  }
  const d = Math.floor(abs / 86_400_000);
  return future ? `in ${d}d` : `${d}d ago`;
}

function formatInterval(interval: string): string {
  if (!interval) return "—";
  // Go durations like "30m0s", "1h0m0s"
  const match = interval.match(/^(?:(\d+)h)?(?:(\d+)m)?/);
  if (!match) return interval;
  const h = match[1] ? parseInt(match[1]) : 0;
  const m = match[2] ? parseInt(match[2]) : 0;
  if (h > 0 && m > 0) return `Every ${h}h ${m}m`;
  if (h > 0) return `Every ${h}h`;
  if (m > 0) return `Every ${m}m`;
  return interval;
}

function EcgLine() {
  return (
    <svg
      viewBox="0 0 120 32"
      className="w-full h-8 mt-2"
      preserveAspectRatio="none"
    >
      <polyline
        className="ecg-line"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        points="0,16 20,16 28,16 32,4 36,28 40,16 48,16 60,16 68,16 72,4 76,28 80,16 88,16 100,16 108,16 112,4 116,28 120,16"
      />
    </svg>
  );
}

export default function StatusTab() {
  const [providers, setProviders] = useState<string[]>([]);
  const [channels, setChannels] = useState<string[]>([]);
  const [sessionCount, setSessionCount] = useState<number>(0);
  const [heartbeat, setHeartbeat] = useState<HeartbeatStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchHeartbeat = useCallback(() => {
    getHeartbeatStatus()
      .then((data) => setHeartbeat(data))
      .catch(() => {});
  }, []);

  useEffect(() => {
    Promise.all([getStatus(), getSessions({ limit: 1, offset: 0 }), getHeartbeatStatus()])
      .then(([statusData, sessionsPage, hb]) => {
        setProviders(statusData.providers || []);
        setChannels(statusData.channels || []);
        setSessionCount(sessionsPage.pagination?.total ?? 0);
        setHeartbeat(hb);
        setError(null);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load status");
      })
      .finally(() => {
        setLoading(false);
      });
  }, []);

  // Heartbeat auto-refresh every 5s
  useEffect(() => {
    const interval = setInterval(fetchHeartbeat, 5000);
    return () => clearInterval(interval);
  }, [fetchHeartbeat]);

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center bg-base">
        <p className="text-xs font-mono text-fg-3">Loading status...</p>
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

  return (
    <div className="p-6 bg-base min-h-full">
      {/* Section 1: System Health */}
      <section className="mb-8">
        <h2 className="section-label mb-4">System</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 stagger">
          {/* Heartbeat card */}
          <div className="card p-5">
            <div className="flex items-center justify-between mb-3">
              <span className="font-display font-semibold text-fg">Heartbeat</span>
              {heartbeat?.running ? (
                <div className="flex items-center gap-2">
                  <div className="status-indicator online" />
                  <span className="text-xs font-medium text-accent">Active</span>
                </div>
              ) : (
                <div className="flex items-center gap-2">
                  <div className="status-indicator offline" />
                  <span className="text-xs font-medium text-fg-3">Inactive</span>
                </div>
              )}
            </div>

            {heartbeat && (
              <>
                <div className="flex items-center gap-4 text-xs font-mono text-fg-2">
                  <span>{formatInterval(heartbeat.interval)}</span>
                  <span>Last: {formatRelativeTime(heartbeat.last_run)}</span>
                </div>
                {heartbeat.running && (
                  <div className="text-accent">
                    <EcgLine />
                  </div>
                )}
              </>
            )}
          </div>

          {/* Session count card */}
          <div className="card p-5">
            <span
              className="font-display text-3xl font-bold text-fg block"
              style={{ fontVariantNumeric: "tabular-nums" }}
            >
              {sessionCount}
            </span>
            <span className="text-sm text-fg-2 mt-1 block">Active Sessions</span>
          </div>
        </div>
      </section>

      {/* Section 2: Providers */}
      {providers.length > 0 && (
        <section className="mb-8">
          <h2 className="section-label mb-4">Providers</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3 stagger">
            {providers.map((name) => (
              <StatusCard key={name} name={name} detail="active" />
            ))}
          </div>
        </section>
      )}

      {/* Section 3: Channels */}
      {channels.length > 0 && (
        <section className="mb-8">
          <h2 className="section-label mb-4">Channels</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3 stagger">
            {channels.map((name) => (
              <StatusCard key={name} name={name} detail="active" />
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
