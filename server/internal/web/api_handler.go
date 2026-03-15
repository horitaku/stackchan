package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/horitaku/stackchan/server/internal/conversation"
	"github.com/horitaku/stackchan/server/internal/providers"
)

// APIHandler はフェーズ7の可観測性・設定更新・疎通テスト API を提供します。
type APIHandler struct {
	runtimeState  *RuntimeState
	settingsStore *SettingsStore
	orchestrator  *conversation.Orchestrator
	voicevoxBase  string
	httpClient    *http.Client
}

// TestPipelineRequest は疎通テスト API の入力です。
type TestPipelineRequest struct {
	SessionID string `json:"session_id"`
	StreamID  string `json:"stream_id"`
}

// VoicevoxUITestRequest は WebUI 単体 TTS テスト API の入力です。
type VoicevoxUITestRequest struct {
	Text            string   `json:"text"`
	Speaker         int      `json:"speaker"`
	SpeedScale      *float64 `json:"speed_scale,omitempty"`
	PitchScale      *float64 `json:"pitch_scale,omitempty"`
	IntonationScale *float64 `json:"intonation_scale,omitempty"`
	VolumeScale     *float64 `json:"volume_scale,omitempty"`
}

// NewAPIHandler は APIHandler を初期化します。
func NewAPIHandler(runtimeState *RuntimeState, settingsStore *SettingsStore, orchestrator *conversation.Orchestrator) *APIHandler {
	baseURL := strings.TrimRight(os.Getenv("VOICEVOX_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "http://voicevox:50021"
	}
	return NewAPIHandlerWithVoicevox(runtimeState, settingsStore, orchestrator, baseURL)
}

// NewAPIHandlerWithVoicevox は Voicevox URL を指定して APIHandler を初期化します。
func NewAPIHandlerWithVoicevox(runtimeState *RuntimeState, settingsStore *SettingsStore, orchestrator *conversation.Orchestrator, voicevoxBase string) *APIHandler {
	if runtimeState == nil {
		runtimeState = NewRuntimeState()
	}
	if settingsStore == nil {
		settingsStore = NewSettingsStore()
	}
	trimmed := strings.TrimRight(strings.TrimSpace(voicevoxBase), "/")
	if trimmed == "" {
		trimmed = "http://voicevox:50021"
	}
	return &APIHandler{
		runtimeState:  runtimeState,
		settingsStore: settingsStore,
		orchestrator:  orchestrator,
		voicevoxBase:  trimmed,
		httpClient:    &http.Client{Timeout: 15 * time.Second},
	}
}

// RegisterRoutes は API ルートを登録します。
func (h *APIHandler) RegisterRoutes(r gin.IRouter) {
	r.GET("/runtime/overview", h.GetRuntimeOverview)
	r.GET("/settings", h.GetSettings)
	r.PUT("/settings", h.UpdateSettings)
	r.POST("/tests/pipeline", h.RunPipelineTest)
	r.POST("/tests/voicevox/ui", h.RunVoicevoxUITest)
}

// GetRuntimeOverview は可観測性スナップショットを返します。
func (h *APIHandler) GetRuntimeOverview(c *gin.Context) {
	c.JSON(http.StatusOK, h.runtimeState.Snapshot())
}

// GetSettings は現在の設定値を返します。
func (h *APIHandler) GetSettings(c *gin.Context) {
	c.JSON(http.StatusOK, h.settingsStore.Get())
}

// UpdateSettings は設定値を更新します。
func (h *APIHandler) UpdateSettings(c *gin.Context) {
	var req RuntimeSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	updated, err := h.settingsStore.Update(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

// RunPipelineTest は STT/LLM/TTS の最小疎通テストを実行します。
func (h *APIHandler) RunPipelineTest(c *gin.Context) {
	if h.orchestrator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "orchestrator is not configured"})
		return
	}

	var req TestPipelineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.SessionID == "" {
		req.SessionID = "webui-diagnostic-session"
	}
	if req.StreamID == "" {
		req.StreamID = "diag-" + uuid.NewString()
	}

	start := time.Now()
	result, err := h.orchestrator.ProcessAudioStream(c.Request.Context(), req.SessionID, req.StreamID, req.StreamID, []providers.AudioChunk{
		{
			StreamID:        req.StreamID,
			ChunkIndex:      0,
			Codec:           "opus",
			SampleRateHz:    16000,
			FrameDurationMs: 20,
			ChannelCount:    1,
			DataBase64:      "AAAA",
		},
	})
	if err != nil {
		h.runtimeState.OnOutputError()
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	queueWaitMs := int64(0)
	h.runtimeState.OnPipeline(result.RequestID, req.StreamID, queueWaitMs, result.STTLatencyMs, result.LLMLatencyMs, result.TTSLatencyMs, result.TotalLatencyMs)
	h.runtimeState.OnPlaybackQueued(result.RequestID, time.Since(start).Milliseconds(), result.TTSDuration)
	h.runtimeState.OnPlaybackSent()
	h.runtimeState.OnPlaybackCompleted()

	c.JSON(http.StatusOK, gin.H{
		"request_id":       result.RequestID,
		"transcript":       result.Transcript,
		"reply_text":       result.ReplyText,
		"provider_path":    result.ProviderPath,
		"stt_latency_ms":   result.STTLatencyMs,
		"llm_latency_ms":   result.LLMLatencyMs,
		"tts_latency_ms":   result.TTSLatencyMs,
		"total_latency_ms": result.TotalLatencyMs,
		"duration_ms":      result.TTSDuration,
	})
}

// RunVoicevoxUITest は WebUI から Voicevox の UI 単体テストを実行します。
func (h *APIHandler) RunVoicevoxUITest(c *gin.Context) {
	var req VoicevoxUITestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		text = "こんにちは、Stackchan の Voicevox テストです。"
	}
	speaker := req.Speaker
	if speaker <= 0 {
		speaker = 1
	}

	queryURL := h.voicevoxBase + "/audio_query?text=" + url.QueryEscape(text) + "&speaker=" + url.QueryEscape(strconv.Itoa(speaker))
	queryReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, queryURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build voicevox query request"})
		return
	}

	start := time.Now()
	queryResp, err := h.httpClient.Do(queryReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "voicevox audio_query request failed"})
		return
	}
	defer queryResp.Body.Close()

	if queryResp.StatusCode < 200 || queryResp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(queryResp.Body, 2048))
		c.JSON(http.StatusBadGateway, gin.H{"error": "voicevox audio_query failed", "detail": string(body)})
		return
	}

	var audioQuery map[string]any
	if err := json.NewDecoder(queryResp.Body).Decode(&audioQuery); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to decode voicevox audio_query response"})
		return
	}

	if req.SpeedScale != nil {
		audioQuery["speedScale"] = *req.SpeedScale
	}
	if req.PitchScale != nil {
		audioQuery["pitchScale"] = *req.PitchScale
	}
	if req.IntonationScale != nil {
		audioQuery["intonationScale"] = *req.IntonationScale
	}
	if req.VolumeScale != nil {
		audioQuery["volumeScale"] = *req.VolumeScale
	}

	queryBody, err := json.Marshal(audioQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode synthesis payload"})
		return
	}

	synthURL := h.voicevoxBase + "/synthesis?speaker=" + url.QueryEscape(strconv.Itoa(speaker))
	synthReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, synthURL, bytes.NewReader(queryBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build voicevox synthesis request"})
		return
	}
	synthReq.Header.Set("Content-Type", "application/json")

	synthResp, err := h.httpClient.Do(synthReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "voicevox synthesis request failed"})
		return
	}
	defer synthResp.Body.Close()

	if synthResp.StatusCode < 200 || synthResp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(synthResp.Body, 2048))
		c.JSON(http.StatusBadGateway, gin.H{"error": "voicevox synthesis failed", "detail": string(body)})
		return
	}

	audioBytes, err := io.ReadAll(synthResp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read voicevox synthesis response"})
		return
	}

	elapsed := time.Since(start).Milliseconds()
	contentType := synthResp.Header.Get("Content-Type")
	if strings.TrimSpace(contentType) == "" {
		contentType = "audio/wav"
	}

	c.JSON(http.StatusOK, gin.H{
		"text":         text,
		"speaker":      speaker,
		"content_type": contentType,
		"audio_base64": base64.StdEncoding.EncodeToString(audioBytes),
		"bytes":        len(audioBytes),
		"latency_ms":   elapsed,
	})
}
