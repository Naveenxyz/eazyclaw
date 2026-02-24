package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/bus"
	"github.com/eazyclaw/eazyclaw/internal/tool"
)

func TestCronRunnerSaveJobsCreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	jobsPath := filepath.Join(tmp, "nested", "cron", "jobs.json")
	cr := NewCronRunner(jobsPath, bus.New(8))

	if _, err := cr.AddJob("*/5 * * * *", "ping", "", ""); err != nil {
		t.Fatalf("add job: %v", err)
	}
	if _, err := os.Stat(jobsPath); err != nil {
		t.Fatalf("expected jobs file to exist at %s: %v", jobsPath, err)
	}
}

func TestCronRunnerStartProcessesDueJobsImmediately(t *testing.T) {
	b := bus.New(8)
	cr := &CronRunner{
		jobsPath: filepath.Join(t.TempDir(), "cron", "jobs.json"),
		bus:      b,
		jobs: []tool.CronJob{
			{
				ID:       "job-1",
				Schedule: "*/5 * * * *",
				Task:     "hello",
				Enabled:  true,
				NextRun:  time.Now().Add(-time.Minute),
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go cr.Start(ctx)

	select {
	case msg := <-b.Inbound:
		if msg.ChannelID != "cron" || msg.Text != "hello" {
			t.Fatalf("unexpected message: %#v", msg)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("expected immediate cron dispatch on start")
	}
}

func TestCronRunnerTickAttachesDeliveryTarget(t *testing.T) {
	b := bus.New(8)
	cr := &CronRunner{
		jobsPath: filepath.Join(t.TempDir(), "cron", "jobs.json"),
		bus:      b,
		jobs: []tool.CronJob{
			{
				ID:              "job-1",
				Schedule:        "*/5 * * * *",
				Task:            "hello",
				DeliveryChannel: "telegram",
				DeliveryChatID:  "123",
				Enabled:         true,
				NextRun:         time.Now().Add(-time.Minute),
			},
		},
	}

	cr.tick()

	select {
	case msg := <-b.Inbound:
		if msg.ReplyChannelID != "telegram" || msg.ReplyChatID != "123" {
			t.Fatalf("unexpected delivery target: %#v", msg)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("expected cron dispatch")
	}
}
