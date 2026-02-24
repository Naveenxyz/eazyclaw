export interface ToolCall {
  id: string;
  name: string;
  arguments: string | Record<string, unknown>;
}

export interface Message {
  role: "user" | "assistant" | "tool";
  content: string;
  tool_calls?: ToolCall[];
  tool_call_id?: string;
}

export interface Session {
  id: string;
  messages: Message[];
  created: string;
  updated: string;
  pagination?: SessionDetailPagination;
}

export interface SessionSummary {
  id: string;
  message_count: number;
  created: string;
  updated: string;
}

export interface SessionsPagination {
  limit: number;
  offset: number;
  total: number;
  has_more: boolean;
}

export interface SessionsPage {
  items: SessionSummary[];
  pagination: SessionsPagination;
}

export interface SessionDetailPagination {
  limit: number;
  total: number;
  has_more: boolean;
  next_before_seq?: number;
}

export interface SkillTool {
  name: string;
  description: string;
  command: string;
}

export interface SkillDependency {
  manager: string;
  package: string;
}

export interface Skill {
  name: string;
  description: string;
  tools: SkillTool[];
  dependencies: SkillDependency[];
}

export interface StatusResponse {
  providers: string[];
  channels: string[];
}

export interface DiscordGuildChannelConfig {
  allow?: boolean;
  require_mention?: boolean;
}

export interface DiscordGuildConfig {
  require_mention: boolean;
  channels?: Record<string, DiscordGuildChannelConfig>;
}

export interface DiscordDMConfig {
  policy: "allow" | "deny" | "pairing";
}

export interface DiscordChannelConfig {
  allowed_users: string[];
  group_policy: "allowlist" | "open";
  dm: DiscordDMConfig;
  guilds?: Record<string, DiscordGuildConfig>;
}

export interface DiscordPendingApproval {
  user_id: string;
  username: string;
  preview: string;
  message_count: number;
  first_seen_at: string;
  last_seen_at: string;
}

export interface DiscordAdminState {
  group_policy: "allowlist" | "open";
  dm_policy: "allow" | "deny" | "pairing";
  allowed_users: string[];
  pending_approvals: DiscordPendingApproval[];
}

export interface TelegramPendingApproval {
  user_id: string;
  username: string;
  preview: string;
  message_count: number;
  first_seen_at: string;
  last_seen_at: string;
}

export interface TelegramAdminState {
  group_policy: "allowlist" | "open";
  dm_policy: "allow" | "deny";
  allowed_users: string[];
  pending_approvals: TelegramPendingApproval[];
}

export interface TelegramChatConfig {
  allow: boolean;
  require_mention: boolean;
}

export interface TelegramDMConfig {
  policy: "allow" | "deny";
}

export interface TelegramChannelConfig {
  allowed_users: string[];
  group_policy: "allowlist" | "open";
  dm: TelegramDMConfig;
  allowed_chats?: Record<string, TelegramChatConfig>;
}

export interface ChannelConfig {
  discord: DiscordChannelConfig;
  telegram: TelegramChannelConfig;
  web: {
    enabled: boolean;
    port: number;
    has_password: boolean;
  };
}

export interface WSMessage {
  type: "message" | "typing" | "done";
  role?: string;
  content?: string;
}

export interface MemoryNode {
  name: string;
  path: string;
  type: 'file' | 'dir';
  children?: MemoryNode[];
}

// --- Cron ---

export interface CronJob {
  id: string;
  schedule: string;
  task: string;
  enabled: boolean;
  last_run: string;
  next_run: string;
}

// --- Heartbeat ---

export interface HeartbeatStatus {
  enabled: boolean;
  interval: string;
  last_run: string;
  running: boolean;
}

// --- WhatsApp ---

export interface WhatsAppDMConfig {
  policy: "allow" | "deny";
}

export interface WhatsAppChannelConfig {
  allowed_users: string[];
  group_policy: "allowlist" | "open";
  dm: WhatsAppDMConfig;
}

export interface WhatsAppPendingApproval {
  user_id: string;
  display_name: string;
  phone_number: string;
  preview: string;
  message_count: number;
  first_seen_at: string;
  last_seen_at: string;
}

export interface WhatsAppAdminState {
  group_policy: "allowlist" | "open";
  dm_policy: "allow" | "deny";
  allowed_users: string[];
  pending_approvals: WhatsAppPendingApproval[];
  status: "disconnected" | "qr_pending" | "connected";
  phone_number: string;
  qr_code: string;
}

// --- Google ---

export interface GoogleAuthStatus {
  authenticated: boolean;
}
