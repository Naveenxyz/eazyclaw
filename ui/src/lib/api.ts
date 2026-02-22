import type { StatusResponse, SessionSummary, Session, Skill, ChannelConfig } from "@/types";

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

export async function getSessions(): Promise<SessionSummary[]> {
  return request("/api/sessions");
}

export async function getSession(id: string): Promise<Session> {
  return request(`/api/sessions/${encodeURIComponent(id)}`);
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
