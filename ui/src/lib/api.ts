import type {
  StatusResponse,
  SessionsPage,
  Session,
  Skill,
  ChannelConfig,
  DiscordAdminState,
  TelegramAdminState,
  WhatsAppAdminState,
  GoogleAuthStatus,
  MemoryNode,
  CronJob,
  HeartbeatStatus,
} from "@/types";

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: "include",
    ...options,
  });
  if (res.status === 401) {
    window.location.href = "/login";
    throw new Error("Unauthorized");
  }
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || res.statusText);
  }
  return res.json();
}

export async function login(password: string): Promise<{ status: string }> {
  return request("/api/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ password }),
  });
}

export async function getStatus(): Promise<StatusResponse> {
  return request("/api/status");
}

export async function getSessions(params?: { limit?: number; offset?: number }): Promise<SessionsPage> {
  const query = new URLSearchParams();
  if (typeof params?.limit === "number") query.set("limit", String(params.limit));
  if (typeof params?.offset === "number") query.set("offset", String(params.offset));
  const suffix = query.toString() ? `?${query.toString()}` : "";
  return request(`/api/sessions${suffix}`);
}

export async function getSession(
  id: string,
  params?: { limit?: number; beforeSeq?: number }
): Promise<Session> {
  const query = new URLSearchParams();
  if (typeof params?.limit === "number") query.set("limit", String(params.limit));
  if (typeof params?.beforeSeq === "number") query.set("before_seq", String(params.beforeSeq));
  const suffix = query.toString() ? `?${query.toString()}` : "";
  return request(`/api/sessions/${encodeURIComponent(id)}${suffix}`);
}

export async function getSkills(): Promise<Skill[]> {
  return request("/api/skills");
}

export async function getConfig(): Promise<ChannelConfig> {
  return request("/api/config");
}

export async function putConfig(data: Partial<ChannelConfig>): Promise<{ status: string; message: string }> {
  return request("/api/config", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
}

export async function getDiscordAdminState(): Promise<DiscordAdminState> {
  return request("/api/discord");
}

export async function updateDiscordApproval(
  action: "approve" | "reject",
  userId: string
): Promise<DiscordAdminState> {
  return request("/api/discord/approvals", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action, user_id: userId }),
  });
}

export async function getTelegramAdminState(): Promise<TelegramAdminState> {
  return request("/api/telegram");
}

export async function updateTelegramApproval(
  action: "approve" | "reject",
  userId: string
): Promise<TelegramAdminState> {
  return request("/api/telegram/approvals", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action, user_id: userId }),
  });
}

export async function getMemoryTree(): Promise<MemoryNode> {
  return request("/api/memory");
}

export async function getMemoryFile(path: string): Promise<{ path: string; content: string }> {
  return request(`/api/memory/${path}`);
}

export async function putMemoryFile(path: string, content: string): Promise<void> {
  return request(`/api/memory/${path}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ content }),
  });
}

// --- Cron API ---

export async function getCronJobs(): Promise<CronJob[]> {
  return request("/api/cron");
}

export async function addCronJob(
  schedule: string,
  task: string,
  deliveryChannel?: string,
  deliveryChatID?: string
): Promise<{ id: string; status: string }> {
  return request("/api/cron", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      schedule,
      task,
      ...(deliveryChannel !== undefined ? { delivery_channel: deliveryChannel } : {}),
      ...(deliveryChatID !== undefined ? { delivery_chat_id: deliveryChatID } : {}),
    }),
  });
}

export async function updateCronJob(
  id: string,
  data: {
    schedule?: string;
    task?: string;
    enabled?: boolean;
    delivery_channel?: string;
    delivery_chat_id?: string;
  }
): Promise<{ id: string; status: string }> {
  return request(`/api/cron/${encodeURIComponent(id)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
}

export async function deleteCronJob(id: string): Promise<{ id: string; status: string }> {
  return request(`/api/cron/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

export async function toggleCronJob(id: string): Promise<{ id: string; enabled: boolean }> {
  return request(`/api/cron/${encodeURIComponent(id)}/toggle`, {
    method: "POST",
  });
}

// --- Heartbeat API ---

export async function getHeartbeatStatus(): Promise<HeartbeatStatus> {
  return request("/api/heartbeat");
}

// --- WhatsApp API ---

export async function getWhatsAppAdminState(): Promise<WhatsAppAdminState> {
  return request("/api/whatsapp/status");
}

export async function getWhatsAppQR(): Promise<string> {
  const data = await request<{ qr_code: string }>("/api/whatsapp/qr");
  return data.qr_code;
}

export async function disconnectWhatsApp(): Promise<{ status: string }> {
  return request("/api/whatsapp/disconnect", { method: "POST" });
}

export async function updateWhatsAppSettings(data: {
  allowed_users?: string[];
  group_policy?: string;
  dm_policy?: string;
}): Promise<{ status: string }> {
  return request("/api/whatsapp/settings", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
}

export async function updateWhatsAppApproval(
  action: "approve" | "reject",
  userId: string
): Promise<WhatsAppAdminState> {
  const endpoint = action === "approve" ? "/api/whatsapp/approve" : "/api/whatsapp/reject";
  return request(endpoint, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ user_id: userId }),
  });
}

// --- Google API ---

export async function getGoogleAuthStatus(): Promise<GoogleAuthStatus> {
  return request("/api/google/status");
}

export async function getGoogleAuthURL(): Promise<{ url: string }> {
  return request("/api/google/auth/url");
}

export async function disconnectGoogle(): Promise<{ status: string }> {
  return request("/api/google/disconnect", { method: "POST" });
}
