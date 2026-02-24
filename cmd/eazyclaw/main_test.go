package main

import (
	"strings"
	"testing"

	"github.com/eazyclaw/eazyclaw/internal/config"
	"github.com/eazyclaw/eazyclaw/internal/skill"
	"github.com/eazyclaw/eazyclaw/internal/state"
)

func TestApplyStoreRuntimeChannelState_OverlaysPoliciesAndAllowlists(t *testing.T) {
	store, err := state.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.SetAllowedUsers("discord", []string{"d1"}); err != nil {
		t.Fatalf("set discord users: %v", err)
	}
	if err := store.SetPolicy("discord", "group_policy", "allowlist"); err != nil {
		t.Fatalf("set discord group policy: %v", err)
	}
	if err := store.SetPolicy("discord", "dm_policy", "deny"); err != nil {
		t.Fatalf("set discord dm policy: %v", err)
	}
	if err := store.SetAllowedUsers("telegram", []string{"t1"}); err != nil {
		t.Fatalf("set telegram users: %v", err)
	}
	if err := store.SetPolicy("telegram", "group_policy", "open"); err != nil {
		t.Fatalf("set telegram group policy: %v", err)
	}
	if err := store.SetPolicy("telegram", "dm_policy", "deny"); err != nil {
		t.Fatalf("set telegram dm policy: %v", err)
	}
	if err := store.SetAllowedUsers("whatsapp", []string{"w1"}); err != nil {
		t.Fatalf("set whatsapp users: %v", err)
	}
	if err := store.SetPolicy("whatsapp", "group_policy", "open"); err != nil {
		t.Fatalf("set whatsapp group policy: %v", err)
	}
	if err := store.SetPolicy("whatsapp", "dm_policy", "deny"); err != nil {
		t.Fatalf("set whatsapp dm policy: %v", err)
	}

	ch := config.ChannelsConfig{}
	applyStoreRuntimeChannelState(&ch, store)

	if len(ch.Discord.AllowedUsers) != 1 || ch.Discord.AllowedUsers[0] != "d1" {
		t.Fatalf("unexpected discord users: %#v", ch.Discord.AllowedUsers)
	}
	if ch.Discord.GroupPolicy != "allowlist" || ch.Discord.DM.Policy != "deny" {
		t.Fatalf("unexpected discord policies: group=%q dm=%q", ch.Discord.GroupPolicy, ch.Discord.DM.Policy)
	}

	if len(ch.Telegram.AllowedUsers) != 1 || ch.Telegram.AllowedUsers[0] != "t1" {
		t.Fatalf("unexpected telegram users: %#v", ch.Telegram.AllowedUsers)
	}
	if ch.Telegram.GroupPolicy != "open" || ch.Telegram.DM.Policy != "deny" {
		t.Fatalf("unexpected telegram policies: group=%q dm=%q", ch.Telegram.GroupPolicy, ch.Telegram.DM.Policy)
	}

	if len(ch.WhatsApp.AllowedUsers) != 1 || ch.WhatsApp.AllowedUsers[0] != "w1" {
		t.Fatalf("unexpected whatsapp users: %#v", ch.WhatsApp.AllowedUsers)
	}
	if ch.WhatsApp.GroupPolicy != "open" || ch.WhatsApp.DM.Policy != "deny" {
		t.Fatalf("unexpected whatsapp policies: group=%q dm=%q", ch.WhatsApp.GroupPolicy, ch.WhatsApp.DM.Policy)
	}
}

func TestBuildSkillPromptEntries_FormatsAndBoundsOutput(t *testing.T) {
	skills := []skill.Skill{
		{
			Name:         "weather",
			Path:         "/tmp/skills/weather/skill.md",
			Description:  "Forecast and weather lookups",
			Instructions: strings.Repeat("x", 1500),
		},
	}

	entries := buildSkillPromptEntries(skills, 30000)
	if len(entries) != 1 {
		t.Fatalf("expected one entry, got %d", len(entries))
	}
	entry := entries[0]
	if !strings.Contains(entry, "### weather") {
		t.Fatalf("missing skill name header: %q", entry)
	}
	if !strings.Contains(entry, "Skill file: /tmp/skills/weather/skill.md") {
		t.Fatalf("missing skill path in entry: %q", entry)
	}
	if !strings.Contains(entry, "...[truncated]") {
		t.Fatalf("expected truncated instruction preview marker")
	}
}

func TestBuildSkillPromptEntries_RespectsTotalPromptBudget(t *testing.T) {
	skills := []skill.Skill{
		{Name: "s1", Instructions: strings.Repeat("a", 400)},
		{Name: "s2", Instructions: strings.Repeat("b", 400)},
		{Name: "s3", Instructions: strings.Repeat("c", 400)},
	}

	entries := buildSkillPromptEntries(skills, 550)
	if len(entries) == 0 {
		t.Fatalf("expected at least one skill entry within budget")
	}
	if len(entries) >= len(skills) {
		t.Fatalf("expected budget-limited truncation, got %d entries for %d skills", len(entries), len(skills))
	}
}
