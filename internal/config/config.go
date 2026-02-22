package config

import (
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v3"
)

// Config holds all configuration for EazyClaw.
type Config struct {
	DataDir      string          `yaml:"data_dir" envconfig:"DATA_DIR"`
	WorkspaceDir string          `yaml:"workspace_dir" envconfig:"WORKSPACE_DIR"`
	Providers    ProvidersConfig `yaml:"providers"`
	Channels     ChannelsConfig  `yaml:"channels"`
	Tools        ToolsConfig     `yaml:"tools"`
	SkillsDir    string          `yaml:"skills_dir" envconfig:"SKILLS_DIR"`
	Heartbeat    HeartbeatConfig `yaml:"heartbeat"`
	Memory       MemoryConfig    `yaml:"memory"`
}

// HeartbeatConfig holds settings for the periodic heartbeat runner.
type HeartbeatConfig struct {
	Enabled  bool          `yaml:"enabled" envconfig:"HEARTBEAT_ENABLED"`
	Interval time.Duration `yaml:"interval" envconfig:"HEARTBEAT_INTERVAL"`
}

// MemoryConfig controls OpenClaw-style file memory behavior.
type MemoryConfig struct {
	Enabled             *bool                  `yaml:"enabled"`
	ContextMaxChars     int                    `yaml:"context_max_chars"`
	DailyFilesInContext int                    `yaml:"daily_files_in_context"`
	Compaction          MemoryCompactionConfig `yaml:"compaction"`
	Background          MemoryBackgroundConfig `yaml:"background"`
}

// MemoryCompactionConfig tunes compaction + pre-compaction memory flushing.
type MemoryCompactionConfig struct {
	Enabled             *bool `yaml:"enabled"`
	TriggerMessages     int   `yaml:"trigger_messages"`
	KeepRecentMessages  int   `yaml:"keep_recent_messages"`
	SummaryMaxChars     int   `yaml:"summary_max_chars"`
	ContextWindowTokens int   `yaml:"context_window_tokens"`
	ReserveTokensFloor  int   `yaml:"reserve_tokens_floor"`
	SoftThresholdTokens int   `yaml:"soft_threshold_tokens"`
	PreFlushMemoryWrite *bool `yaml:"pre_flush_memory_write"`
}

// MemoryBackgroundConfig controls periodic background memory digests.
type MemoryBackgroundConfig struct {
	Enabled  *bool         `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
	MaxRuns  int           `yaml:"max_runs"`
}

// ProvidersConfig holds LLM provider settings.
type ProvidersConfig struct {
	DefaultModel string        `yaml:"default_model" envconfig:"DEFAULT_MODEL"`
	Anthropic    ProviderEntry `yaml:"anthropic"`
	OpenAI       ProviderEntry `yaml:"openai"`
	Gemini       ProviderEntry `yaml:"gemini"`
	Moonshot     ProviderEntry `yaml:"moonshot"`
	KimiCoding   ProviderEntry `yaml:"kimi_coding"`
	Zhipu        ProviderEntry `yaml:"zhipu"`
}

// ProviderEntry holds settings for a single provider.
type ProviderEntry struct {
	Model   string `yaml:"model"`
	BaseURL string `yaml:"base_url"`
	Auth    string `yaml:"auth"` // "api_key" | "oauth"
}

// ChannelsConfig holds messaging channel settings.
type ChannelsConfig struct {
	Telegram TelegramChannelConfig `yaml:"telegram"`
	Discord  DiscordChannelConfig  `yaml:"discord"`
	Web      WebChannelEntry       `yaml:"web"`
}

// DiscordGuildChannelConfig holds per-channel config within a guild.
type DiscordGuildChannelConfig struct {
	Allow          *bool `yaml:"allow,omitempty" json:"allow,omitempty"`
	RequireMention *bool `yaml:"require_mention,omitempty" json:"require_mention,omitempty"`
}

// DiscordGuildConfig holds per-guild settings.
type DiscordGuildConfig struct {
	RequireMention bool                                 `yaml:"require_mention" json:"require_mention"`
	Channels       map[string]DiscordGuildChannelConfig `yaml:"channels,omitempty" json:"channels,omitempty"`
}

// DiscordDMConfig holds DM policy for Discord.
type DiscordDMConfig struct {
	Policy string `yaml:"policy" json:"policy"` // "allow" | "deny" | "pairing"
}

// DiscordChannelConfig holds rich Discord channel settings.
type DiscordChannelConfig struct {
	AllowedUsers []string                      `yaml:"allowed_users" json:"allowed_users"`
	GroupPolicy  string                        `yaml:"group_policy" json:"group_policy"` // "allowlist" | "open"
	DM           DiscordDMConfig               `yaml:"dm" json:"dm"`
	Guilds       map[string]DiscordGuildConfig `yaml:"guilds,omitempty" json:"guilds,omitempty"`
}

// TelegramChatConfig holds per-chat settings for Telegram.
type TelegramChatConfig struct {
	Allow          bool `yaml:"allow" json:"allow"`
	RequireMention bool `yaml:"require_mention" json:"require_mention"`
}

// TelegramDMConfig holds DM policy for Telegram.
type TelegramDMConfig struct {
	Policy string `yaml:"policy" json:"policy"` // "allow" | "deny"
}

// TelegramChannelConfig holds rich Telegram channel settings.
type TelegramChannelConfig struct {
	AllowedUsers []string                      `yaml:"allowed_users" json:"allowed_users"`
	GroupPolicy  string                        `yaml:"group_policy" json:"group_policy"` // "allowlist" | "open"
	DM           TelegramDMConfig              `yaml:"dm" json:"dm"`
	AllowedChats map[string]TelegramChatConfig `yaml:"allowed_chats,omitempty" json:"allowed_chats,omitempty"`
}

// WebChannelEntry holds settings for the web dashboard channel.
type WebChannelEntry struct {
	Enabled  bool   `yaml:"enabled" envconfig:"WEB_ENABLED"`
	Port     int    `yaml:"port" envconfig:"WEB_PORT"`
	Password string `yaml:"-" envconfig:"WEB_PASSWORD"` // env-only, no yaml
}

// ToolsConfig holds tool settings.
type ToolsConfig struct {
	Shell   ShellConfig   `yaml:"shell"`
	Browser BrowserConfig `yaml:"browser"`
}

// ShellConfig holds shell tool settings.
type ShellConfig struct {
	DenyPatterns  []string      `yaml:"deny_patterns"`
	Timeout       time.Duration `yaml:"timeout"`
	WorkspaceOnly bool          `yaml:"workspace_only"`
}

// BrowserConfig holds browser tool settings.
type BrowserConfig struct {
	Headless bool `yaml:"headless"`
}

// Load reads config from a YAML file and overlays environment variables.
func Load(path string) (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	if err := envconfig.Process("", cfg); err != nil {
		return nil, err
	}

	setDefaults(cfg)
	return cfg, nil
}

func setDefaults(cfg *Config) {
	if cfg.DataDir == "" {
		cfg.DataDir = "/data/eazyclaw"
	}
	if cfg.WorkspaceDir == "" {
		cfg.WorkspaceDir = cfg.DataDir + "/workspace"
	}
	if cfg.SkillsDir == "" {
		cfg.SkillsDir = cfg.DataDir + "/skills"
	}
	if cfg.Providers.DefaultModel == "" {
		cfg.Providers.DefaultModel = "claude-sonnet-4-6"
	}
	if cfg.Tools.Shell.Timeout == 0 {
		cfg.Tools.Shell.Timeout = 60 * time.Second
	}
	if cfg.Channels.Web.Port == 0 {
		cfg.Channels.Web.Port = 8080
	}
	if cfg.Channels.Discord.GroupPolicy == "" {
		cfg.Channels.Discord.GroupPolicy = "allowlist"
	}
	if cfg.Channels.Discord.DM.Policy == "" {
		cfg.Channels.Discord.DM.Policy = "allow"
	}
	if cfg.Channels.Telegram.GroupPolicy == "" {
		cfg.Channels.Telegram.GroupPolicy = "allowlist"
	}
	if cfg.Channels.Telegram.DM.Policy == "" {
		cfg.Channels.Telegram.DM.Policy = "allow"
	}
	if cfg.Heartbeat.Interval == 0 {
		cfg.Heartbeat.Interval = 5 * time.Minute
	}
	if cfg.Memory.Enabled == nil {
		cfg.Memory.Enabled = boolPtr(true)
	}
	if cfg.Memory.ContextMaxChars == 0 {
		cfg.Memory.ContextMaxChars = 12000
	}
	if cfg.Memory.DailyFilesInContext == 0 {
		cfg.Memory.DailyFilesInContext = 2
	}
	if cfg.Memory.Compaction.Enabled == nil {
		cfg.Memory.Compaction.Enabled = boolPtr(true)
	}
	if cfg.Memory.Compaction.TriggerMessages == 0 {
		cfg.Memory.Compaction.TriggerMessages = 80
	}
	if cfg.Memory.Compaction.KeepRecentMessages == 0 {
		cfg.Memory.Compaction.KeepRecentMessages = 30
	}
	if cfg.Memory.Compaction.SummaryMaxChars == 0 {
		cfg.Memory.Compaction.SummaryMaxChars = 3500
	}
	if cfg.Memory.Compaction.ContextWindowTokens == 0 {
		cfg.Memory.Compaction.ContextWindowTokens = 200000
	}
	if cfg.Memory.Compaction.ReserveTokensFloor == 0 {
		cfg.Memory.Compaction.ReserveTokensFloor = 20000
	}
	if cfg.Memory.Compaction.SoftThresholdTokens == 0 {
		cfg.Memory.Compaction.SoftThresholdTokens = 4000
	}
	if cfg.Memory.Compaction.PreFlushMemoryWrite == nil {
		cfg.Memory.Compaction.PreFlushMemoryWrite = boolPtr(true)
	}
	if cfg.Memory.Background.Enabled == nil {
		cfg.Memory.Background.Enabled = boolPtr(true)
	}
	if cfg.Memory.Background.Interval == 0 {
		cfg.Memory.Background.Interval = 10 * time.Minute
	}
	if cfg.Memory.Background.MaxRuns == 0 {
		cfg.Memory.Background.MaxRuns = 20
	}
}

func boolPtr(v bool) *bool {
	return &v
}
