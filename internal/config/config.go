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
	Guilds       map[string]DiscordGuildConfig  `yaml:"guilds,omitempty" json:"guilds,omitempty"`
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
		cfg.DataDir = "/data"
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
}
