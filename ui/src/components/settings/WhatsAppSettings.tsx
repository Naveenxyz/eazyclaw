import { useState } from "react";
import { Plus, X, Check, Ban, Smartphone, QrCode, Unplug } from "lucide-react";
import { QRCodeSVG } from "qrcode.react";
import type { WhatsAppAdminState } from "@/types";

interface WhatsAppSettingsProps {
  adminState: WhatsAppAdminState | null;
  onApprove: (userId: string) => void;
  onReject: (userId: string) => void;
  onDisconnect: () => void;
  onAddUser: (userId: string) => void;
  onRemoveUser: (userId: string) => void;
  onPolicyChange: (field: "group_policy" | "dm_policy", value: string) => void;
  pendingActionUserId?: string | null;
  qrData?: string | null;
}

export function WhatsAppSettings({
  adminState,
  onApprove,
  onReject,
  onDisconnect,
  onAddUser,
  onRemoveUser,
  onPolicyChange,
  pendingActionUserId,
  qrData,
}: WhatsAppSettingsProps) {
  const [newUser, setNewUser] = useState("");

  const status = adminState?.status ?? "disconnected";
  const allowedUsers = adminState?.allowed_users ?? [];
  const pendingApprovals = adminState?.pending_approvals ?? [];

  const addUser = () => {
    const trimmed = newUser.trim();
    if (!trimmed || allowedUsers.includes(trimmed)) return;
    onAddUser(trimmed);
    setNewUser("");
  };

  return (
    <div className="card p-5">
      <h3 className="font-display font-semibold text-fg mb-5">WhatsApp</h3>

      {/* Connection Status */}
      <div className="mb-5">
        <label className="section-label text-[10px] mb-1.5 block">Connection</label>
        <div className="rounded-md border border-edge bg-raised p-4">
          {status === "connected" ? (
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <div className="h-2 w-2 rounded-full bg-accent animate-pulse" />
                <Smartphone size={14} className="text-fg-2" />
                <span className="text-sm font-mono text-fg">
                  {adminState?.phone_number || "Connected"}
                </span>
              </div>
              <button
                onClick={onDisconnect}
                className="btn-danger !px-2.5 !py-1 !text-xs flex items-center gap-1"
              >
                <Unplug size={11} />
                Disconnect
              </button>
            </div>
          ) : status === "qr_pending" ? (
            <div className="flex flex-col items-center gap-3">
              <QrCode size={16} className="text-fg-3" />
              <p className="text-xs text-fg-3">Scan QR code with WhatsApp to pair</p>
              {qrData ? (
                <div className="p-3 bg-white rounded-md">
                  <QRCodeSVG value={qrData} size={192} />
                </div>
              ) : (
                <div className="w-48 h-48 rounded-md border border-edge bg-base flex items-center justify-center">
                  <div className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-accent border-t-transparent" />
                </div>
              )}
            </div>
          ) : (
            <div className="flex items-center gap-2">
              <div className="h-2 w-2 rounded-full bg-fg-3" />
              <span className="text-sm text-fg-3">Disconnected</span>
            </div>
          )}
        </div>
      </div>

      {/* Group Policy */}
      <div className="mb-5">
        <label className="section-label text-[10px] mb-1.5 block">Group Policy</label>
        <select
          value={adminState?.group_policy ?? "allowlist"}
          onChange={(e) => onPolicyChange("group_policy", e.target.value)}
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
          value={adminState?.dm_policy ?? "allow"}
          onChange={(e) => onPolicyChange("dm_policy", e.target.value)}
          className="w-full bg-raised border border-edge rounded-md px-3 py-2 text-sm font-mono text-fg input-focus"
        >
          <option value="allow">allow</option>
          <option value="deny">deny</option>
        </select>
      </div>

      {/* Allowed Users */}
      <div className="mb-5">
        <label className="section-label text-[10px] mb-1.5 block">Allowed Users</label>
        {allowedUsers.length > 0 && (
          <div className="mb-2 flex flex-wrap gap-1.5">
            {allowedUsers.map((user) => (
              <span
                key={user}
                className="inline-flex items-center gap-1.5 badge-accent font-mono text-sm"
              >
                {user}
                <button
                  onClick={() => onRemoveUser(user)}
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
            placeholder="Phone JID (e.g. 919876543210@s.whatsapp.net)..."
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
        <div>
          <label className="section-label text-[10px] mb-2 block">Pending DM Approvals</label>
          <div className="flex flex-col gap-2">
            {pendingApprovals.map((item) => (
              <div
                key={item.user_id}
                className="rounded-md border border-edge bg-raised p-3"
              >
                <div className="flex items-center justify-between gap-3">
                  <div className="min-w-0">
                    <div className="truncate text-sm font-mono text-fg">
                      {item.display_name || item.phone_number || item.user_id}
                    </div>
                    <div className="truncate text-xs text-fg-3 mt-0.5">
                      {item.phone_number} &middot; {item.message_count} message
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
    </div>
  );
}
