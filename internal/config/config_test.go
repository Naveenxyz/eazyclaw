package config

import "testing"

func TestSetDefaultsInitializesAllowedUserSlices(t *testing.T) {
	cfg := &Config{}
	setDefaults(cfg)

	if cfg.Channels.Discord.AllowedUsers == nil {
		t.Fatalf("discord allowed users should be initialized")
	}
	if cfg.Channels.Telegram.AllowedUsers == nil {
		t.Fatalf("telegram allowed users should be initialized")
	}
}
