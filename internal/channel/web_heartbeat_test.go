package channel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/eazyclaw/eazyclaw/internal/agent"
	"github.com/eazyclaw/eazyclaw/internal/bus"
	"github.com/eazyclaw/eazyclaw/internal/config"
	"github.com/eazyclaw/eazyclaw/internal/state"
)

func TestHandleHeartbeatPutPersistsDeliveryTargetWhenRunnerDisabled(t *testing.T) {
	store, err := state.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	w := &WebChannel{
		store:        store,
		heartbeatCfg: &config.HeartbeatConfig{Enabled: false, Interval: time.Minute},
	}

	req := httptest.NewRequest(http.MethodPut, "/api/heartbeat", strings.NewReader(`{"delivery_channel":"telegram","delivery_chat_id":"123"}`))
	rr := httptest.NewRecorder()
	w.handleHeartbeat(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/heartbeat", nil)
	w.handleHeartbeat(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}

	var status agent.HeartbeatStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.DeliveryChannel != "telegram" || status.DeliveryChatID != "123" {
		t.Fatalf("unexpected heartbeat delivery target: %#v", status)
	}
}

func TestHandleHeartbeatPutUpdatesRunnerDeliveryTarget(t *testing.T) {
	runner := agent.NewHeartbeatRunner(time.Minute, bus.New(2), "", "", "")
	w := &WebChannel{heartbeatRunner: runner}

	req := httptest.NewRequest(http.MethodPut, "/api/heartbeat", strings.NewReader(`{"delivery_channel":"discord","delivery_chat_id":"chan-1"}`))
	rr := httptest.NewRecorder()
	w.handleHeartbeat(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}

	status := runner.Status()
	if status.DeliveryChannel != "discord" || status.DeliveryChatID != "chan-1" {
		t.Fatalf("unexpected heartbeat delivery target: %#v", status)
	}
}

func TestHandleHeartbeatPutRejectsPartialDeliveryTarget(t *testing.T) {
	w := &WebChannel{}

	req := httptest.NewRequest(http.MethodPut, "/api/heartbeat", strings.NewReader(`{"delivery_channel":"discord"}`))
	rr := httptest.NewRecorder()
	w.handleHeartbeat(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", rr.Code, rr.Body.String())
	}
}
