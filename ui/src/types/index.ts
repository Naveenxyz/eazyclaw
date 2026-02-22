export interface ToolCall {
  id: string;
  name: string;
  arguments: string;
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
}

export interface SessionSummary {
  id: string;
  message_count: number;
  created: string;
  updated: string;
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
