package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
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
	data, err := json.MarshalIndent(cr.jobs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cr.jobsPath, data, 0644)
}

func (cr *CronRunner) AddJob(schedule, task string) (string, error) {
	nextRun, err := nextRunTime(schedule)
	if err != nil {
		return "", fmt.Errorf("invalid schedule %q: %w", schedule, err)
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	job := tool.CronJob{
		ID:       uuid.New().String(),
		Schedule: schedule,
		Task:     task,
		Enabled:  true,
		NextRun:  nextRun,
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

func (cr *CronRunner) UpdateJob(id, schedule, task string, enabled bool) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	for i, j := range cr.jobs {
		if j.ID == id {
			if schedule != "" {
				next, err := nextRunTime(schedule)
				if err != nil {
					return fmt.Errorf("invalid schedule %q: %w", schedule, err)
				}
				cr.jobs[i].Schedule = schedule
				cr.jobs[i].NextRun = next
			}
			if task != "" {
				cr.jobs[i].Task = task
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
	defer cr.mu.Unlock()

	now := time.Now()
	changed := false

	for i := range cr.jobs {
		j := &cr.jobs[i]
		if !j.Enabled || now.Before(j.NextRun) {
			continue
		}

		cr.bus.Inbound <- bus.Message{
			ID:        uuid.New().String(),
			ChannelID: "cron",
			SenderID:  "cron",
			Text:      j.Task,
			Timestamp: now,
		}

		j.LastRun = now
		next, err := nextRunTime(j.Schedule)
		if err != nil {
			log.Printf("cron: bad schedule for job %s: %v", j.ID, err)
			j.Enabled = false
		} else {
			j.NextRun = next
		}
		changed = true
	}

	if changed {
		if err := cr.saveJobs(); err != nil {
			log.Printf("cron: failed to save jobs: %v", err)
		}
	}
}

func nextRunTime(schedule string) (time.Time, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(schedule)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(time.Now()), nil
}
