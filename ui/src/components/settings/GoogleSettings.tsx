import { Link, Unplug } from "lucide-react";
import type { GoogleAuthStatus } from "@/types";

interface GoogleSettingsProps {
  authStatus: GoogleAuthStatus | null;
  onConnect: () => void;
  onDisconnect: () => void;
}

export function GoogleSettings({
  authStatus,
  onConnect,
  onDisconnect,
}: GoogleSettingsProps) {
  const isAuthenticated = authStatus?.authenticated ?? false;

  return (
    <div className="card p-5">
      <h3 className="font-display font-semibold text-fg mb-5">Google (Gmail + Calendar)</h3>

      <div className="rounded-md border border-edge bg-raised p-4">
        {isAuthenticated ? (
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="h-2 w-2 rounded-full bg-accent animate-pulse" />
              <span className="text-sm text-fg">Connected</span>
            </div>
            <button
              onClick={onDisconnect}
              className="btn-danger !px-2.5 !py-1 !text-xs flex items-center gap-1"
            >
              <Unplug size={11} />
              Disconnect
            </button>
          </div>
        ) : (
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="h-2 w-2 rounded-full bg-fg-3" />
              <span className="text-sm text-fg-3">Not connected</span>
            </div>
            <button
              onClick={onConnect}
              className="btn-accent !px-2.5 !py-1 !text-xs flex items-center gap-1"
            >
              <Link size={11} />
              Connect Google
            </button>
          </div>
        )}
      </div>

      <p className="mt-3 text-xs text-fg-3">
        Connect your Google account to enable Gmail and Calendar tools for the agent.
      </p>
    </div>
  );
}
