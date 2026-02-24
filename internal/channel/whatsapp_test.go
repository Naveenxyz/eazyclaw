package channel

import (
	"testing"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/config"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func TestWhatsAppGroupUserAllowed_AllowlistPolicyFailsClosed(t *testing.T) {
	store := mustOpenStore(t)
	w := NewWhatsAppChannel(config.WhatsAppChannelConfig{
		GroupPolicy:  "allowlist",
		AllowedUsers: []string{},
		DM:           config.WhatsAppDMConfig{Policy: "allow"},
	}, "", store)

	if w.isGroupUserAllowed("15550000001") {
		t.Fatalf("expected group sender to be denied when group_policy=allowlist and user is not allowlisted")
	}
}

func TestWhatsAppGroupUserAllowed_AllowlistPolicyAllowsListedUser(t *testing.T) {
	store := mustOpenStore(t)
	store.AddAllowedUser("whatsapp", "15550000002")

	w := NewWhatsAppChannel(config.WhatsAppChannelConfig{
		GroupPolicy:  "allowlist",
		AllowedUsers: []string{"15550000002"},
		DM:           config.WhatsAppDMConfig{Policy: "allow"},
	}, "", store)

	if !w.isGroupUserAllowed("15550000002") {
		t.Fatalf("expected allowlisted group sender to be allowed")
	}
}

func TestWhatsAppGroupUserAllowed_OpenPolicyAllowsWhenNoAllowlist(t *testing.T) {
	store := mustOpenStore(t)
	w := NewWhatsAppChannel(config.WhatsAppChannelConfig{
		GroupPolicy:  "open",
		AllowedUsers: []string{},
		DM:           config.WhatsAppDMConfig{Policy: "allow"},
	}, "", store)

	if !w.isGroupUserAllowed("15550000003") {
		t.Fatalf("expected group sender to be allowed in open policy without user allowlist")
	}
}

func TestWhatsAppDisallowedGroupCreatesPendingApproval(t *testing.T) {
	store := mustOpenStore(t)
	w := NewWhatsAppChannel(config.WhatsAppChannelConfig{
		GroupPolicy:  "allowlist",
		AllowedUsers: []string{},
		DM:           config.WhatsAppDMConfig{Policy: "allow"},
	}, "", store)

	w.handleMessage(newWhatsAppTextEvent(
		"15550000004",
		"120363025555555555",
		true,
		"group hello",
		time.Now().UTC(),
	))

	state := w.Snapshot()
	if len(state.Pending) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(state.Pending))
	}
	if state.Pending[0].UserID != "15550000004" {
		t.Fatalf("unexpected pending user id: %s", state.Pending[0].UserID)
	}
}

func newWhatsAppTextEvent(senderUser, chatUser string, isGroup bool, text string, at time.Time) *events.Message {
	chatServer := "s.whatsapp.net"
	if isGroup {
		chatServer = "g.us"
	}

	return &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Sender:  types.NewJID(senderUser, "s.whatsapp.net"),
				Chat:    types.NewJID(chatUser, chatServer),
				IsGroup: isGroup,
			},
			ID:        "m-test-1",
			Timestamp: at,
		},
		Message: &waE2E.Message{
			Conversation: proto.String(text),
		},
	}
}
