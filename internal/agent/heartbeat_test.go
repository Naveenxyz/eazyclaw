package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/bus"
)

func TestHeartbeatContentHasActionableTasks(t *testing.T) {
	if heartbeatContentHasActionableTasks("") {
		t.Fatalf("empty content should not be actionable")
	}
	if heartbeatContentHasActionableTasks(defaultHeartbeatTemplate) {
		t.Fatalf("default template should not be actionable")
	}
	if heartbeatContentHasActionableTasks("# HEARTBEAT.md\n\n<!-- note -->\n") {
		t.Fatalf("heading/comment only content should not be actionable")
	}
	if !heartbeatContentHasActionableTasks("# HEARTBEAT.md\n\n- [ ] Send daily report\n") {
		t.Fatalf("task checkbox content should be actionable")
	}
}

func TestHeartbeatRunnerSkipsWhenNoActionableTasks(t *testing.T) {
	tmp := t.TempDir()
	heartbeatPath := filepath.Join(tmp, "HEARTBEAT.md")
	if err := os.WriteFile(heartbeatPath, []byte(defaultHeartbeatTemplate), 0o644); err != nil {
		t.Fatalf("write heartbeat: %v", err)
	}

	b := bus.New(4)
	runner := NewHeartbeatRunner(15*time.Millisecond, b, heartbeatPath, "", "")
	ctx, cancel := context.WithCancel(context.Background())
	go runner.Start(ctx)
	time.Sleep(70 * time.Millisecond)
	cancel()
	time.Sleep(20 * time.Millisecond)

	select {
	case <-b.Inbound:
		t.Fatalf("did not expect heartbeat message when file has no actionable tasks")
	default:
	}
}

func TestHeartbeatRunnerSendsWhenActionableTasksExist(t *testing.T) {
	tmp := t.TempDir()
	heartbeatPath := filepath.Join(tmp, "HEARTBEAT.md")
	if err := os.WriteFile(heartbeatPath, []byte("# HEARTBEAT.md\n\n- [ ] Check backups\n"), 0o644); err != nil {
		t.Fatalf("write heartbeat: %v", err)
	}

	b := bus.New(4)
	runner := NewHeartbeatRunner(15*time.Millisecond, b, heartbeatPath, "", "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runner.Start(ctx)

	deadline := time.After(300 * time.Millisecond)
	for {
		select {
		case msg := <-b.Inbound:
			if msg.ChannelID == "heartbeat" {
				return
			}
		case <-deadline:
			t.Fatalf("expected heartbeat message when actionable tasks exist")
		}
	}
}

func TestHeartbeatRunnerIncludesDeliveryTarget(t *testing.T) {
	tmp := t.TempDir()
	heartbeatPath := filepath.Join(tmp, "HEARTBEAT.md")
	if err := os.WriteFile(heartbeatPath, []byte("# HEARTBEAT.md\n\n- [ ] Check backups\n"), 0o644); err != nil {
		t.Fatalf("write heartbeat: %v", err)
	}

	b := bus.New(4)
	runner := NewHeartbeatRunner(15*time.Millisecond, b, heartbeatPath, "discord", "chan-1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runner.Start(ctx)

	deadline := time.After(300 * time.Millisecond)
	for {
		select {
		case msg := <-b.Inbound:
			if msg.ChannelID != "heartbeat" {
				continue
			}
			if msg.ReplyChannelID != "discord" || msg.ReplyChatID != "chan-1" {
				t.Fatalf("unexpected delivery target: %#v", msg)
			}
			return
		case <-deadline:
			t.Fatalf("expected heartbeat message when actionable tasks exist")
		}
	}
}
