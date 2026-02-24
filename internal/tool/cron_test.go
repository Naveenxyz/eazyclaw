package tool

import (
	"context"
	"encoding/json"
	"testing"
)

type stubCronManager struct {
	jobs []CronJob

	updateCalled          bool
	updateID              string
	updateEnabled         bool
	updateDeliveryChannel *string
	updateDeliveryChatID  *string
	addDeliveryChannel    string
	addDeliveryChatID     string
}

func (m *stubCronManager) AddJob(schedule, task, deliveryChannel, deliveryChatID string) (string, error) {
	m.addDeliveryChannel = deliveryChannel
	m.addDeliveryChatID = deliveryChatID
	return "new", nil
}
func (m *stubCronManager) RemoveJob(id string) error { return nil }
func (m *stubCronManager) ListJobs() []CronJob       { return append([]CronJob(nil), m.jobs...) }
func (m *stubCronManager) UpdateJob(id, schedule, task string, enabled bool, deliveryChannel, deliveryChatID *string) error {
	m.updateCalled = true
	m.updateID = id
	m.updateEnabled = enabled
	m.updateDeliveryChannel = deliveryChannel
	m.updateDeliveryChatID = deliveryChatID
	return nil
}

func TestCronToolUpdatePreservesEnabledWhenOmitted(t *testing.T) {
	mgr := &stubCronManager{
		jobs: []CronJob{{ID: "job-1", Enabled: false}},
	}
	tool := NewCronTool(mgr)

	args, _ := json.Marshal(map[string]any{
		"action":   "update",
		"id":       "job-1",
		"schedule": "*/5 * * * *",
		"task":     "check",
	})
	res, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %+v", res)
	}
	if !mgr.updateCalled {
		t.Fatalf("expected update to be called")
	}
	if mgr.updateID != "job-1" {
		t.Fatalf("unexpected update id: %q", mgr.updateID)
	}
	if mgr.updateEnabled {
		t.Fatalf("expected enabled=false to be preserved when omitted")
	}
	if mgr.updateDeliveryChannel != nil || mgr.updateDeliveryChatID != nil {
		t.Fatalf("did not expect delivery target patch when omitted")
	}
}

func TestCronToolUpdateReturnsErrorWhenJobMissingAndEnabledOmitted(t *testing.T) {
	mgr := &stubCronManager{}
	tool := NewCronTool(mgr)

	args, _ := json.Marshal(map[string]any{
		"action": "update",
		"id":     "missing",
	})
	res, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected error result for missing job")
	}
}

func TestCronToolAddAcceptsDeliveryTarget(t *testing.T) {
	mgr := &stubCronManager{}
	tool := NewCronTool(mgr)

	args, _ := json.Marshal(map[string]any{
		"action":           "add",
		"schedule":         "*/5 * * * *",
		"task":             "check",
		"delivery_channel": "telegram",
		"delivery_chat_id": "123",
	})
	res, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %+v", res)
	}
	if mgr.addDeliveryChannel != "telegram" || mgr.addDeliveryChatID != "123" {
		t.Fatalf("unexpected delivery target: %q %q", mgr.addDeliveryChannel, mgr.addDeliveryChatID)
	}
}

func TestCronToolUpdateRejectsPartialDeliveryTarget(t *testing.T) {
	mgr := &stubCronManager{
		jobs: []CronJob{{ID: "job-1", Enabled: true}},
	}
	tool := NewCronTool(mgr)

	args, _ := json.Marshal(map[string]any{
		"action":           "update",
		"id":               "job-1",
		"delivery_channel": "telegram",
	})
	res, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected error result for partial delivery target")
	}
}
