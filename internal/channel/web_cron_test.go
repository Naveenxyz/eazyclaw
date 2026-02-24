package channel

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/eazyclaw/eazyclaw/internal/tool"
)

type stubWebCronManager struct {
	jobs []tool.CronJob

	updateCalled          bool
	updateID              string
	updateEnabled         bool
	updateDeliveryChannel *string
	updateDeliveryChatID  *string
}

func (m *stubWebCronManager) AddJob(schedule, task, deliveryChannel, deliveryChatID string) (string, error) {
	return "new", nil
}
func (m *stubWebCronManager) RemoveJob(id string) error { return nil }
func (m *stubWebCronManager) ListJobs() []tool.CronJob {
	return append([]tool.CronJob(nil), m.jobs...)
}
func (m *stubWebCronManager) UpdateJob(id, schedule, task string, enabled bool, deliveryChannel, deliveryChatID *string) error {
	m.updateCalled = true
	m.updateID = id
	m.updateEnabled = enabled
	m.updateDeliveryChannel = deliveryChannel
	m.updateDeliveryChatID = deliveryChatID
	return nil
}

func TestHandleCronJobPutPreservesEnabledWhenOmitted(t *testing.T) {
	mgr := &stubWebCronManager{
		jobs: []tool.CronJob{{ID: "job-1", Enabled: false}},
	}
	w := &WebChannel{cronManager: mgr}

	req := httptest.NewRequest(http.MethodPut, "/api/cron/job-1", strings.NewReader(`{"task":"updated"}`))
	rr := httptest.NewRecorder()
	w.handleCronJob(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if !mgr.updateCalled {
		t.Fatalf("expected update call")
	}
	if mgr.updateID != "job-1" {
		t.Fatalf("unexpected update id: %q", mgr.updateID)
	}
	if mgr.updateEnabled {
		t.Fatalf("expected existing enabled=false to be preserved")
	}
	if mgr.updateDeliveryChannel != nil || mgr.updateDeliveryChatID != nil {
		t.Fatalf("did not expect delivery target patch when omitted")
	}
}

func TestHandleCronJobPutMissingJobReturnsNotFound(t *testing.T) {
	mgr := &stubWebCronManager{}
	w := &WebChannel{cronManager: mgr}

	req := httptest.NewRequest(http.MethodPut, "/api/cron/missing", strings.NewReader(`{"task":"updated"}`))
	rr := httptest.NewRecorder()
	w.handleCronJob(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestHandleCronJobPutRejectsPartialDeliveryTarget(t *testing.T) {
	mgr := &stubWebCronManager{
		jobs: []tool.CronJob{{ID: "job-1", Enabled: true}},
	}
	w := &WebChannel{cronManager: mgr}

	req := httptest.NewRequest(http.MethodPut, "/api/cron/job-1", strings.NewReader(`{"delivery_channel":"telegram"}`))
	rr := httptest.NewRecorder()
	w.handleCronJob(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", rr.Code, rr.Body.String())
	}
}
