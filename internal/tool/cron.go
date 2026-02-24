package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// CronJob represents a scheduled task.
type CronJob struct {
	ID              string    `json:"id"`
	Schedule        string    `json:"schedule"`
	Task            string    `json:"task"`
	DeliveryChannel string    `json:"delivery_channel,omitempty"`
	DeliveryChatID  string    `json:"delivery_chat_id,omitempty"`
	Enabled         bool      `json:"enabled"`
	LastRun         time.Time `json:"last_run"`
	NextRun         time.Time `json:"next_run"`
}

// CronManager is the interface for managing cron jobs.
type CronManager interface {
	AddJob(schedule, task, deliveryChannel, deliveryChatID string) (string, error)
	RemoveJob(id string) error
	ListJobs() []CronJob
	UpdateJob(id, schedule, task string, enabled bool, deliveryChannel, deliveryChatID *string) error
}

// CronTool implements the Tool interface for cron operations.
type CronTool struct {
	manager CronManager
}

// NewCronTool creates a new CronTool with the given manager.
func NewCronTool(manager CronManager) *CronTool {
	return &CronTool{manager: manager}
}

func (t *CronTool) Name() string        { return "cron" }
func (t *CronTool) Description() string { return "Manage scheduled cron jobs" }
func (t *CronTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"action":   {"type": "string", "enum": ["list", "add", "remove", "update", "enable", "disable"]},
			"id":       {"type": "string"},
			"schedule": {"type": "string"},
			"task":     {"type": "string"},
			"delivery_channel": {"type": "string"},
			"delivery_chat_id": {"type": "string"},
			"enabled":  {"type": "boolean"}
		},
		"required": ["action"]
	}`)
}

type cronArgs struct {
	Action          string  `json:"action"`
	ID              string  `json:"id"`
	Schedule        string  `json:"schedule"`
	Task            string  `json:"task"`
	DeliveryChannel *string `json:"delivery_channel"`
	DeliveryChatID  *string `json:"delivery_chat_id"`
	Enabled         *bool   `json:"enabled"`
}

func (t *CronTool) Execute(_ context.Context, args json.RawMessage) (*Result, error) {
	var a cronArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return &Result{Error: "invalid arguments: " + err.Error(), IsError: true}, nil
	}

	switch a.Action {
	case "list":
		jobs := t.manager.ListJobs()
		data, _ := json.Marshal(jobs)
		return &Result{Content: string(data)}, nil

	case "add":
		if a.Schedule == "" || a.Task == "" {
			return &Result{Error: "add requires 'schedule' and 'task'", IsError: true}, nil
		}
		deliveryChannel, deliveryChatID, err := resolveDeliveryForAdd(a.DeliveryChannel, a.DeliveryChatID)
		if err != nil {
			return &Result{Error: err.Error(), IsError: true}, nil
		}
		id, err := t.manager.AddJob(a.Schedule, a.Task, deliveryChannel, deliveryChatID)
		if err != nil {
			return &Result{Error: err.Error(), IsError: true}, nil
		}
		resp, _ := json.Marshal(map[string]string{"id": id, "status": "added"})
		return &Result{Content: string(resp)}, nil

	case "remove":
		if a.ID == "" {
			return &Result{Error: "remove requires 'id'", IsError: true}, nil
		}
		if err := t.manager.RemoveJob(a.ID); err != nil {
			return &Result{Error: err.Error(), IsError: true}, nil
		}
		resp, _ := json.Marshal(map[string]string{"id": a.ID, "status": "removed"})
		return &Result{Content: string(resp)}, nil

	case "update":
		if a.ID == "" {
			return &Result{Error: "update requires 'id'", IsError: true}, nil
		}
		enabled := false
		if a.Enabled == nil {
			found := false
			for _, j := range t.manager.ListJobs() {
				if j.ID == a.ID {
					enabled = j.Enabled
					found = true
					break
				}
			}
			if !found {
				return &Result{Error: fmt.Sprintf("job not found: %s", a.ID), IsError: true}, nil
			}
		} else {
			enabled = *a.Enabled
		}
		deliveryChannel, deliveryChatID, err := resolveDeliveryForUpdate(a.DeliveryChannel, a.DeliveryChatID)
		if err != nil {
			return &Result{Error: err.Error(), IsError: true}, nil
		}
		if err := t.manager.UpdateJob(a.ID, a.Schedule, a.Task, enabled, deliveryChannel, deliveryChatID); err != nil {
			return &Result{Error: err.Error(), IsError: true}, nil
		}
		resp, _ := json.Marshal(map[string]string{"id": a.ID, "status": "updated"})
		return &Result{Content: string(resp)}, nil

	case "enable", "disable":
		if a.ID == "" {
			return &Result{Error: a.Action + " requires 'id'", IsError: true}, nil
		}
		enabled := a.Action == "enable"
		if err := t.manager.UpdateJob(a.ID, "", "", enabled, nil, nil); err != nil {
			return &Result{Error: err.Error(), IsError: true}, nil
		}
		resp, _ := json.Marshal(map[string]string{"id": a.ID, "status": a.Action + "d"})
		return &Result{Content: string(resp)}, nil

	default:
		return &Result{Error: fmt.Sprintf("unknown action: %s", a.Action), IsError: true}, nil
	}
}

func resolveDeliveryForAdd(channel, chatID *string) (string, string, error) {
	channelValue, channelSet := normalizeOptionalString(channel)
	chatValue, chatSet := normalizeOptionalString(chatID)

	if channelSet != chatSet {
		return "", "", fmt.Errorf("delivery_channel and delivery_chat_id must be provided together")
	}
	if !channelSet {
		return "", "", nil
	}
	if (channelValue == "") != (chatValue == "") {
		return "", "", fmt.Errorf("delivery_channel and delivery_chat_id must both be empty or both non-empty")
	}
	return channelValue, chatValue, nil
}

func resolveDeliveryForUpdate(channel, chatID *string) (*string, *string, error) {
	channelValue, channelSet := normalizeOptionalString(channel)
	chatValue, chatSet := normalizeOptionalString(chatID)

	if channelSet != chatSet {
		return nil, nil, fmt.Errorf("delivery_channel and delivery_chat_id must be provided together")
	}
	if !channelSet {
		return nil, nil, nil
	}
	if (channelValue == "") != (chatValue == "") {
		return nil, nil, fmt.Errorf("delivery_channel and delivery_chat_id must both be empty or both non-empty")
	}
	return &channelValue, &chatValue, nil
}

func normalizeOptionalString(v *string) (string, bool) {
	if v == nil {
		return "", false
	}
	return strings.TrimSpace(*v), true
}
