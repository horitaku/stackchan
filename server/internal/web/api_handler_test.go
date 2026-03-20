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
	return newAPITestServerWithVoicevox(t, "http://voicevox:50021")
}

func newAPITestServerWithVoicevox(t *testing.T, voicevoxBaseURL string) *httptest.Server {
	t.Helper()
	runtimeState := web.NewRuntimeState()
	settingsStore := web.NewSettingsStore()
	orchestrator := conversation.NewOrchestrator(
		&mock.STT{},
		&mock.LLM{},
		&mock.TTS{},
		providers.CallPolicy{Timeout: 2 * time.Second, MaxAttempts: 1, BaseDelay: 10 * time.Millisecond},
	)
	api := web.NewAPIHandlerWithVoicevox(runtimeState, settingsStore, orchestrator, voicevoxBaseURL)

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

func TestGetRuntimeMetrics_StoreUnavailable(t *testing.T) {
	ts := newAPITestServer(t)
	resp, err := http.Get(ts.URL + "/api/runtime/metrics")
	if err != nil {
		t.Fatalf("failed to get runtime metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
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

func TestRunVoicevoxUITest(t *testing.T) {
	voicevox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/audio_query":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"accent_phrases":[],"speedScale":1.0,"pitchScale":0.0,"intonationScale":1.0,"volumeScale":1.0}`))
		case "/synthesis":
			w.Header().Set("Content-Type", "audio/wav")
			_, _ = w.Write([]byte("RIFFmockwav"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer voicevox.Close()

	ts := newAPITestServerWithVoicevox(t, voicevox.URL)
	body, _ := json.Marshal(map[string]any{"text": "てすと", "speaker": 1})

	resp, err := http.Post(ts.URL+"/api/tests/voicevox/ui", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to run voicevox ui test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["audio_base64"] == "" {
		t.Fatal("expected audio_base64 in result")
	}
}

func TestRunVoicevoxStackchanTest_WSHandlerUnavailable(t *testing.T) {
	ts := newAPITestServer(t)
	body, _ := json.Marshal(map[string]any{"text": "てすと", "speaker": 1})

	resp, err := http.Post(ts.URL+"/api/tests/voicevox/stackchan", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to run voicevox stackchan test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
}

func TestLLMSettingsCRUD(t *testing.T) {
	ts := newAPITestServer(t)

	resp, err := http.Get(ts.URL + "/api/settings/llm")
	if err != nil {
		t.Fatalf("failed to get llm settings: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := json.Marshal(map[string]any{"system_prompt": "あなたは丁寧な案内役です。"})
	updatedResp, err := http.Post(ts.URL+"/api/settings/llm", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to update llm settings: %v", err)
	}
	defer updatedResp.Body.Close()
	if updatedResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", updatedResp.StatusCode)
	}

	var updated map[string]any
	if err := json.NewDecoder(updatedResp.Body).Decode(&updated); err != nil {
		t.Fatalf("failed to decode update response: %v", err)
	}
	if updated["system_prompt"] == "" {
		t.Fatal("expected system_prompt in update response")
	}
}

func TestRunLLMUITest(t *testing.T) {
	ts := newAPITestServer(t)
	body, _ := json.Marshal(map[string]any{"text": "こんにちは"})

	resp, err := http.Post(ts.URL+"/api/tests/llm/ui", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to run llm ui test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["reply_text"] == "" {
		t.Fatal("expected reply_text in response")
	}
}

func TestRunLLMStackchanTest_WSUnavailable(t *testing.T) {
	ts := newAPITestServer(t)
	body, _ := json.Marshal(map[string]any{"text": "こんにちは", "speaker": 1})

	resp, err := http.Post(ts.URL+"/api/tests/llm/stackchan", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to run llm stackchan test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
}

func TestHardwareAPIs_WSUnavailable(t *testing.T) {
	ts := newAPITestServer(t)

	testCases := []struct {
		name   string
		method string
		path   string
		body   map[string]any
	}{
		{name: "servo", method: http.MethodPost, path: "/api/tests/hardware/servo", body: map[string]any{"command": "move", "axis": "x", "angle_x_deg": 10}},
		{name: "led", method: http.MethodPost, path: "/api/tests/hardware/led", body: map[string]any{"mode": "solid", "color": "#00FF00"}},
		{name: "ears", method: http.MethodPost, path: "/api/tests/hardware/ears", body: map[string]any{"mode": "rainbow"}},
		{name: "audio", method: http.MethodPost, path: "/api/tests/hardware/audio/play", body: map[string]any{"tone_hz": 440, "duration_ms": 500}},
		{name: "mic", method: http.MethodPost, path: "/api/tests/hardware/mic/start", body: map[string]any{"duration_ms": 1000}},
		{name: "camera", method: http.MethodPost, path: "/api/tests/hardware/camera/capture", body: map[string]any{"resolution": "qvga"}},
		{name: "state", method: http.MethodGet, path: "/api/tests/hardware/state", body: nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			var err error
			if tc.method == http.MethodGet {
				req, err = http.NewRequest(tc.method, ts.URL+tc.path, nil)
			} else {
				body, _ := json.Marshal(tc.body)
				req, err = http.NewRequest(tc.method, ts.URL+tc.path, bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
			}
			if err != nil {
				t.Fatalf("failed to build request: %v", err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusServiceUnavailable {
				t.Fatalf("expected 503, got %d", resp.StatusCode)
			}

			var body map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			errBody, ok := body["error"].(map[string]any)
			if !ok {
				t.Fatalf("expected error object in response")
			}
			if errBody["code"] != "ws_unavailable" {
				t.Fatalf("expected code=ws_unavailable, got %v", errBody["code"])
			}
		})
	}
}
