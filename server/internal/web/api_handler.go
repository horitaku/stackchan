package web

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stackchan/server/internal/conversation"
	"github.com/stackchan/server/internal/providers"
)

// APIHandler はフェーズ7の可観測性・設定更新・疎通テスト API を提供します。
type APIHandler struct {
	runtimeState  *RuntimeState
	settingsStore *SettingsStore
	orchestrator  *conversation.Orchestrator
}

// TestPipelineRequest は疎通テスト API の入力です。
type TestPipelineRequest struct {
	SessionID string `json:"session_id"`
	StreamID  string `json:"stream_id"`
}

// NewAPIHandler は APIHandler を初期化します。
func NewAPIHandler(runtimeState *RuntimeState, settingsStore *SettingsStore, orchestrator *conversation.Orchestrator) *APIHandler {
	if runtimeState == nil {
		runtimeState = NewRuntimeState()
	}
	if settingsStore == nil {
		settingsStore = NewSettingsStore()
	}
	return &APIHandler{runtimeState: runtimeState, settingsStore: settingsStore, orchestrator: orchestrator}
}

// RegisterRoutes は API ルートを登録します。
func (h *APIHandler) RegisterRoutes(r gin.IRouter) {
	r.GET("/runtime/overview", h.GetRuntimeOverview)
	r.GET("/settings", h.GetSettings)
	r.PUT("/settings", h.UpdateSettings)
	r.POST("/tests/pipeline", h.RunPipelineTest)
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
