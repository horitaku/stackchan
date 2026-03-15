package web_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/horitaku/stackchan/server/internal/conversation"
	"github.com/horitaku/stackchan/server/internal/providers"
	"github.com/horitaku/stackchan/server/internal/providers/mock"
	"github.com/horitaku/stackchan/server/internal/web"
)

func newAPITestServer(t *testing.T) *httptest.Server {
	t.Helper()
	runtimeState := web.NewRuntimeState()
	settingsStore := web.NewSettingsStore()
	orchestrator := conversation.NewOrchestrator(
		&mock.STT{},
		&mock.LLM{},
		&mock.TTS{},
		providers.CallPolicy{Timeout: 2 * time.Second, MaxAttempts: 1, BaseDelay: 10 * time.Millisecond},
	)
	api := web.NewAPIHandler(runtimeState, settingsStore, orchestrator)

	r := gin.New()
	group := r.Group("/api")
	api.RegisterRoutes(group)

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	return ts
}

func TestGetRuntimeOverview(t *testing.T) {
	ts := newAPITestServer(t)
	resp, err := http.Get(ts.URL + "/api/runtime/overview")
	if err != nil {
		t.Fatalf("failed to get runtime overview: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := body["connection"]; !ok {
		t.Fatal("expected connection section")
	}
}

func TestUpdateSettings(t *testing.T) {
	ts := newAPITestServer(t)
	payload := map[string]any{
		"playback_volume":      55,
		"expression_preset":    "happy",
		"lip_sync_sensitivity": 1.2,
		"lip_sync_damping":     0.4,
		"motion_enabled":       true,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to put settings: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestUpdateSettings_InvalidValue(t *testing.T) {
	ts := newAPITestServer(t)
	payload := map[string]any{
		"playback_volume":      120,
		"expression_preset":    "happy",
		"lip_sync_sensitivity": 1.2,
		"lip_sync_damping":     0.4,
		"motion_enabled":       true,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to put settings: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRunPipelineTest(t *testing.T) {
	ts := newAPITestServer(t)
	body, _ := json.Marshal(map[string]any{})

	resp, err := http.Post(ts.URL+"/api/tests/pipeline", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to run pipeline test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["request_id"] == "" {
		t.Fatal("expected request_id in result")
	}
}
