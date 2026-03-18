package web

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	wsHandler     *WSHandler
	metricsStore  *RuntimeMetricsStore
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

// VoicevoxStackchanTestRequest は WebUI から Stackchan 連携テストを実行する入力です。
type VoicevoxStackchanTestRequest struct {
	Text         string `json:"text"`
	Speaker      int    `json:"speaker"`
	Motion       string `json:"motion,omitempty"`
	Expression   string `json:"expression,omitempty"`
	Codec        string `json:"codec,omitempty"`
	ChunkVersion string `json:"chunk_version,omitempty"`
}

type voicevoxSynthesisResult struct {
	Text        string
	Speaker     int
	ContentType string
	AudioBase64 string
	Bytes       int
	LatencyMs   int64
}

func resolveVoicevoxHTTPTimeout() time.Duration {
	timeoutSec := 45
	if raw := strings.TrimSpace(os.Getenv("VOICEVOX_HTTP_TIMEOUT_SEC")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			timeoutSec = parsed
		}
	}
	return time.Duration(timeoutSec) * time.Second
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
		httpClient:    &http.Client{Timeout: resolveVoicevoxHTTPTimeout()},
	}
}

// AttachWSHandler は Stackchan 連携テスト向けに WSHandler 参照を追加します。
func (h *APIHandler) AttachWSHandler(handler *WSHandler) {
	h.wsHandler = handler
}

// AttachRuntimeMetricsStore は runtime_metrics 取得 API で利用する永続ストアを設定します。
func (h *APIHandler) AttachRuntimeMetricsStore(store *RuntimeMetricsStore) {
	h.metricsStore = store
}

// RegisterRoutes は API ルートを登録します。
func (h *APIHandler) RegisterRoutes(r gin.IRouter) {
	r.GET("/runtime/overview", h.GetRuntimeOverview)
	r.GET("/runtime/metrics", h.GetRuntimeMetrics)
	r.GET("/settings", h.GetSettings)
	r.PUT("/settings", h.UpdateSettings)
	r.POST("/tests/pipeline", h.RunPipelineTest)
	r.POST("/tests/voicevox/ui", h.RunVoicevoxUITest)
	r.POST("/tests/voicevox/stackchan", h.RunVoicevoxStackchanTest)
}

// GetRuntimeOverview は可観測性スナップショットを返します。
func (h *APIHandler) GetRuntimeOverview(c *gin.Context) {
	c.JSON(http.StatusOK, h.runtimeState.Snapshot())
}

// GetRuntimeMetrics は runtime_metrics の履歴を返します。
func (h *APIHandler) GetRuntimeMetrics(c *gin.Context) {
	if h.metricsStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime metrics store is not configured"})
		return
	}

	query := RuntimeMetricsQuery{
		SessionID:  c.Query("session_id"),
		RequestID:  c.Query("request_id"),
		MetricName: c.Query("metric_name"),
	}

	if limitRaw := strings.TrimSpace(c.Query("limit")); limitRaw != "" {
		limit, err := strconv.Atoi(limitRaw)
		if err != nil || limit <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a positive integer"})
			return
		}
		query.Limit = limit
	}

	if fromRaw := strings.TrimSpace(c.Query("from")); fromRaw != "" {
		from, err := time.Parse(time.RFC3339, fromRaw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from must be RFC3339"})
			return
		}
		query.From = &from
	}

	if toRaw := strings.TrimSpace(c.Query("to")); toRaw != "" {
		to, err := time.Parse(time.RFC3339, toRaw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "to must be RFC3339"})
			return
		}
		query.To = &to
	}

	rows, err := h.metricsStore.ListMetrics(query)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": rows,
		"count": len(rows),
	})
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

	result, err := h.synthesizeVoicevox(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"text":         result.Text,
		"speaker":      result.Speaker,
		"content_type": result.ContentType,
		"audio_base64": result.AudioBase64,
		"bytes":        result.Bytes,
		"latency_ms":   result.LatencyMs,
	})
}

// RunVoicevoxStackchanTest は Voicevox で生成した音声を接続中 Stackchan へ送信します。
func (h *APIHandler) RunVoicevoxStackchanTest(c *gin.Context) {
	if h.wsHandler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ws handler is not configured"})
		return
	}

	var req VoicevoxStackchanTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	uiReq := VoicevoxUITestRequest{Text: req.Text, Speaker: req.Speaker}
	result, err := h.synthesizeVoicevox(c.Request.Context(), uiReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	requestID := "stackchan-test-" + uuid.NewString()
	expression := strings.TrimSpace(req.Expression)
	if expression == "" {
		expression = "happy"
	}
	motion := strings.TrimSpace(req.Motion)
	if motion == "" {
		motion = "nod"
	}
	chunkVersion := strings.TrimSpace(req.ChunkVersion)
	if chunkVersion == "" {
		chunkVersion = "1.1"
	}
	if chunkVersion != "1.0" && chunkVersion != "1.1" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chunk_version must be one of: 1.0, 1.1"})
		return
	}

	codec := strings.ToLower(strings.TrimSpace(req.Codec))
	if codec == "" {
		codec = "opus"
	}
	if codec != "pcm" && codec != "opus" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "codec must be one of: pcm, opus"})
		return
	}
	if codec == "opus" && chunkVersion != "1.1" {
		chunkVersion = "1.1"
	}

	sessionID, err := h.wsHandler.SendTTSTestToActive(
		requestID,
		result.AudioBase64,
		0,
		24000,
		codec,
		expression,
		motion,
		chunkVersion,
	)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"request_id":          requestID,
		"session_id":          sessionID,
		"speaker":             result.Speaker,
		"text":                result.Text,
		"voicevox_bytes":      result.Bytes,
		"voicevox_latency_ms": result.LatencyMs,
		"sent_event":          "tts.chunk + tts.end",
		"codec":               codec,
		"chunk_version":       chunkVersion,
		"expression":          expression,
		"motion":              motion,
	})
}

func (h *APIHandler) synthesizeVoicevox(ctx context.Context, req VoicevoxUITestRequest) (voicevoxSynthesisResult, error) {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		text = "こんにちは、Stackchan の Voicevox テストです。"
	}
	speaker := req.Speaker
	if speaker <= 0 {
		speaker = 1
	}

	queryURL := h.voicevoxBase + "/audio_query?text=" + url.QueryEscape(text) + "&speaker=" + url.QueryEscape(strconv.Itoa(speaker))
	queryReq, err := http.NewRequestWithContext(ctx, http.MethodPost, queryURL, nil)
	if err != nil {
		return voicevoxSynthesisResult{}, fmt.Errorf("failed to build voicevox query request")
	}

	start := time.Now()
	queryResp, err := h.httpClient.Do(queryReq)
	if err != nil {
		return voicevoxSynthesisResult{}, fmt.Errorf("voicevox audio_query request failed: %w", err)
	}
	defer queryResp.Body.Close()

	if queryResp.StatusCode < 200 || queryResp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(queryResp.Body, 2048))
		return voicevoxSynthesisResult{}, fmt.Errorf("voicevox audio_query failed: %s", strings.TrimSpace(string(body)))
	}

	var audioQuery map[string]any
	if err := json.NewDecoder(queryResp.Body).Decode(&audioQuery); err != nil {
		return voicevoxSynthesisResult{}, fmt.Errorf("failed to decode voicevox audio_query response")
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
		return voicevoxSynthesisResult{}, fmt.Errorf("failed to encode synthesis payload")
	}

	synthURL := h.voicevoxBase + "/synthesis?speaker=" + url.QueryEscape(strconv.Itoa(speaker))
	synthReq, err := http.NewRequestWithContext(ctx, http.MethodPost, synthURL, bytes.NewReader(queryBody))
	if err != nil {
		return voicevoxSynthesisResult{}, fmt.Errorf("failed to build voicevox synthesis request")
	}
	synthReq.Header.Set("Content-Type", "application/json")

	synthResp, err := h.httpClient.Do(synthReq)
	if err != nil {
		return voicevoxSynthesisResult{}, fmt.Errorf("voicevox synthesis request failed: %w", err)
	}
	defer synthResp.Body.Close()

	if synthResp.StatusCode < 200 || synthResp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(synthResp.Body, 2048))
		return voicevoxSynthesisResult{}, fmt.Errorf("voicevox synthesis failed: %s", strings.TrimSpace(string(body)))
	}

	audioBytes, err := io.ReadAll(synthResp.Body)
	if err != nil {
		return voicevoxSynthesisResult{}, fmt.Errorf("failed to read voicevox synthesis response")
	}

	elapsed := time.Since(start).Milliseconds()
	contentType := synthResp.Header.Get("Content-Type")
	if strings.TrimSpace(contentType) == "" {
		contentType = "audio/wav"
	}

	return voicevoxSynthesisResult{
		Text:        text,
		Speaker:     speaker,
		ContentType: contentType,
		AudioBase64: base64.StdEncoding.EncodeToString(audioBytes),
		Bytes:       len(audioBytes),
		LatencyMs:   elapsed,
	}, nil
}
