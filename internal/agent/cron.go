package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/bus"
	"github.com/eazyclaw/eazyclaw/internal/tool"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// CronRunner implements tool.CronManager and runs scheduled jobs.
type CronRunner struct {
	jobs     []tool.CronJob
	jobsPath string
	bus      *bus.Bus
	mu       sync.RWMutex
}

// NewCronRunner creates a CronRunner, loading existing jobs from the JSON file.
func NewCronRunner(jobsPath string, b *bus.Bus) *CronRunner {
	cr := &CronRunner{
		jobsPath: jobsPath,
		bus:      b,
	}
	cr.loadJobs()
	return cr
}

func (cr *CronRunner) loadJobs() {
	data, err := os.ReadFile(cr.jobsPath)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &cr.jobs)
}

func (cr *CronRunner) saveJobs() error {
	if err := os.MkdirAll(filepath.Dir(cr.jobsPath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cr.jobs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cr.jobsPath, data, 0644)
}

func (cr *CronRunner) AddJob(schedule, task, deliveryChannel, deliveryChatID string) (string, error) {
	nextRun, err := nextRunTimeFrom(schedule, time.Now())
	if err != nil {
		return "", fmt.Errorf("invalid schedule %q: %w", schedule, err)
	}
	deliveryChannel, deliveryChatID, err = normalizeCronDeliveryTarget(deliveryChannel, deliveryChatID)
	if err != nil {
		return "", err
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	job := tool.CronJob{
		ID:              uuid.New().String(),
		Schedule:        schedule,
		Task:            task,
		DeliveryChannel: deliveryChannel,
		DeliveryChatID:  deliveryChatID,
		Enabled:         true,
		NextRun:         nextRun,
	}
	cr.jobs = append(cr.jobs, job)
	if err := cr.saveJobs(); err != nil {
		return "", err
	}
	return job.ID, nil
}

func (cr *CronRunner) RemoveJob(id string) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	for i, j := range cr.jobs {
		if j.ID == id {
			cr.jobs = append(cr.jobs[:i], cr.jobs[i+1:]...)
			return cr.saveJobs()
		}
	}
	return fmt.Errorf("job not found: %s", id)
}

func (cr *CronRunner) ListJobs() []tool.CronJob {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	out := make([]tool.CronJob, len(cr.jobs))
	copy(out, cr.jobs)
	return out
}

func (cr *CronRunner) UpdateJob(id, schedule, task string, enabled bool, deliveryChannel, deliveryChatID *string) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	for i, j := range cr.jobs {
		if j.ID == id {
			if schedule != "" {
				next, err := nextRunTimeFrom(schedule, time.Now())
				if err != nil {
					return fmt.Errorf("invalid schedule %q: %w", schedule, err)
				}
				cr.jobs[i].Schedule = schedule
				cr.jobs[i].NextRun = next
			}
			if task != "" {
				cr.jobs[i].Task = task
			}
			if deliveryChannel != nil || deliveryChatID != nil {
				var channelValue, chatValue string
				if deliveryChannel != nil {
					channelValue = *deliveryChannel
				}
				if deliveryChatID != nil {
					chatValue = *deliveryChatID
				}
				normalizedChannel, normalizedChat, err := normalizeCronDeliveryTarget(channelValue, chatValue)
				if err != nil {
					return err
				}
				cr.jobs[i].DeliveryChannel = normalizedChannel
				cr.jobs[i].DeliveryChatID = normalizedChat
			}
			cr.jobs[i].Enabled = enabled
			return cr.saveJobs()
		}
	}
	return fmt.Errorf("job not found: %s", id)
}

// Start runs the cron check loop until ctx is cancelled.
func (cr *CronRunner) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	cr.tick()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cr.tick()
		}
	}
}

func (cr *CronRunner) tick() {
	cr.mu.Lock()
	now := time.Now()
	changed := false
	dueMessages := make([]bus.Message, 0)

	for i := range cr.jobs {
		j := &cr.jobs[i]
		if !j.Enabled || now.Before(j.NextRun) {
			continue
		}

		dueMessages = append(dueMessages, bus.Message{
			ID:             uuid.New().String(),
			ChannelID:      "cron",
			SenderID:       j.ID,
			UserID:         "cron",
			ReplyChannelID: j.DeliveryChannel,
			ReplyChatID:    j.DeliveryChatID,
			Text:           j.Task,
			Timestamp:      now,
		})

		j.LastRun = now
		next, err := nextRunTimeFrom(j.Schedule, j.NextRun)
		if err != nil {
			log.Printf("cron: bad schedule for job %s: %v", j.ID, err)
			j.Enabled = false
		} else {
			for !next.After(now) {
				next = nextRunTimeOrZero(j.Schedule, next)
				if next.IsZero() {
					log.Printf("cron: bad schedule while catching up for job %s", j.ID)
					j.Enabled = false
					break
				}
			}
			j.NextRun = next
		}
		changed = true
	}

	if changed {
		if err := cr.saveJobs(); err != nil {
			log.Printf("cron: failed to save jobs: %v", err)
		}
	}
	cr.mu.Unlock()

	for _, msg := range dueMessages {
		cr.bus.Inbound <- msg
	}
}

func nextRunTimeFrom(schedule string, from time.Time) (time.Time, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(schedule)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(from), nil
}

func nextRunTimeOrZero(schedule string, from time.Time) time.Time {
	next, err := nextRunTimeFrom(schedule, from)
	if err != nil {
		return time.Time{}
	}
	return next
}

func normalizeCronDeliveryTarget(channel, chatID string) (string, string, error) {
	channel = strings.TrimSpace(channel)
	chatID = strings.TrimSpace(chatID)
	if (channel == "") != (chatID == "") {
		return "", "", fmt.Errorf("delivery_channel and delivery_chat_id must both be set or both empty")
	}
	return channel, chatID, nil
}
