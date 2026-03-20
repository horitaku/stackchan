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
	"github.com/horitaku/stackchan/server/internal/logging"
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

// LLMUITestRequest は WebUI から LLM 単体テストを実行する入力です。
type LLMUITestRequest struct {
	SessionID       string `json:"session_id,omitempty"`
	Text            string `json:"text"`
	PersonaOverride string `json:"persona_override,omitempty"`
}

// LLMStackchanTestRequest は LLM 応答を Stackchan へ送信する連携テスト入力です。
type LLMStackchanTestRequest struct {
	SessionID       string `json:"session_id,omitempty"`
	Text            string `json:"text"`
	PersonaOverride string `json:"persona_override,omitempty"`
	Speaker         int    `json:"speaker"`
	Motion          string `json:"motion,omitempty"`
	Expression      string `json:"expression,omitempty"`
	Codec           string `json:"codec,omitempty"`
	ChunkVersion    string `json:"chunk_version,omitempty"`
}

const defaultHardwareDispatchTimeoutMs = 1500

const (
	hwEventServoMove           = "device.servo.move"
	hwEventServoCalibrationGet = "device.servo.calibration.get"
	hwEventServoCalibrationSet = "device.servo.calibration.set"
	hwEventLedSet              = "device.led.set"
	hwEventEarsSet             = "device.ears.set"
	hwEventAudioTestPlay       = "device.audio.test.play"
	hwEventMicTestStart        = "device.mic.test.start"
	hwEventCameraCapture       = "device.camera.capture"
	hwEventStateReport         = "device.state.report"
)

// HardwareServoTestRequest は /api/tests/hardware/servo の入力です。
// command:
//   - move (default)
//   - calibration_get
//   - calibration_set
type HardwareServoTestRequest struct {
	RequestID           string   `json:"request_id,omitempty"`
	Command             string   `json:"command,omitempty"`
	Axis                string   `json:"axis,omitempty"`
	AngleXDeg           *float64 `json:"angle_x_deg,omitempty"`
	AngleYDeg           *float64 `json:"angle_y_deg,omitempty"`
	Speed               *float64 `json:"speed,omitempty"`
	CenterOffsetDeg     *float64 `json:"center_offset_deg,omitempty"`
	MinDeg              *float64 `json:"min_deg,omitempty"`
	MaxDeg              *float64 `json:"max_deg,omitempty"`
	Invert              *bool    `json:"invert,omitempty"`
	SpeedLimitDegPerSec *float64 `json:"speed_limit_deg_per_sec,omitempty"`
	SoftStart           *bool    `json:"soft_start,omitempty"`
	HomeDeg             *float64 `json:"home_deg,omitempty"`
	TimeoutMs           int      `json:"timeout_ms,omitempty"`
}

// HardwareLedTestRequest は /api/tests/hardware/led の入力です。
type HardwareLedTestRequest struct {
	RequestID       string `json:"request_id,omitempty"`
	Mode            string `json:"mode"`
	Color           string `json:"color,omitempty"`
	Brightness      *int   `json:"brightness,omitempty"`
	BlinkIntervalMs *int   `json:"blink_interval_ms,omitempty"`
	BreathePeriodMs *int   `json:"breathe_period_ms,omitempty"`
	TimeoutMs       int    `json:"timeout_ms,omitempty"`
}

// HardwareEarsTestRequest は /api/tests/hardware/ears の入力です。
type HardwareEarsTestRequest struct {
	RequestID       string `json:"request_id,omitempty"`
	Mode            string `json:"mode"`
	Color           string `json:"color,omitempty"`
	Brightness      *int   `json:"brightness,omitempty"`
	BlinkIntervalMs *int   `json:"blink_interval_ms,omitempty"`
	BreathePeriodMs *int   `json:"breathe_period_ms,omitempty"`
	RainbowPeriodMs *int   `json:"rainbow_period_ms,omitempty"`
	TimeoutMs       int    `json:"timeout_ms,omitempty"`
}

// HardwareAudioPlayTestRequest は /api/tests/hardware/audio/play の入力です。
type HardwareAudioPlayTestRequest struct {
	RequestID  string   `json:"request_id,omitempty"`
	ToneHz     *int     `json:"tone_hz,omitempty"`
	DurationMs *int     `json:"duration_ms,omitempty"`
	Volume     *float64 `json:"volume,omitempty"`
	TimeoutMs  int      `json:"timeout_ms,omitempty"`
}

// HardwareMicStartTestRequest は /api/tests/hardware/mic/start の入力です。
type HardwareMicStartTestRequest struct {
	RequestID       string `json:"request_id,omitempty"`
	DurationMs      *int   `json:"duration_ms,omitempty"`
	SampleRateHz    *int   `json:"sample_rate_hz,omitempty"`
	FrameDurationMs *int   `json:"frame_duration_ms,omitempty"`
	TimeoutMs       int    `json:"timeout_ms,omitempty"`
}

// HardwareCameraCaptureTestRequest は /api/tests/hardware/camera/capture の入力です。
type HardwareCameraCaptureTestRequest struct {
	RequestID  string `json:"request_id,omitempty"`
	Resolution string `json:"resolution,omitempty"`
	Quality    *int   `json:"quality,omitempty"`
	TimeoutMs  int    `json:"timeout_ms,omitempty"`
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
	r.GET("/settings/llm", h.GetLLMSettings)
	r.POST("/settings/llm", h.UpdateLLMSettings)
	r.POST("/tests/pipeline", h.RunPipelineTest)
	r.POST("/tests/llm/ui", h.RunLLMUITest)
	r.POST("/tests/llm/stackchan", h.RunLLMStackchanTest)
	r.POST("/tests/voicevox/ui", h.RunVoicevoxUITest)
	r.POST("/tests/voicevox/stackchan", h.RunVoicevoxStackchanTest)
	r.POST("/tests/hardware/servo", h.RunHardwareServoTest)
	r.POST("/tests/hardware/led", h.RunHardwareLedTest)
	r.POST("/tests/hardware/ears", h.RunHardwareEarsTest)
	r.POST("/tests/hardware/audio/play", h.RunHardwareAudioPlayTest)
	r.POST("/tests/hardware/mic/start", h.RunHardwareMicStartTest)
	r.POST("/tests/hardware/camera/capture", h.RunHardwareCameraCaptureTest)
	r.GET("/tests/hardware/state", h.GetHardwareState)
}

func (h *APIHandler) ensureHardwareWS(c *gin.Context) bool {
	if h.wsHandler == nil {
		h.writeHardwareError(c, http.StatusServiceUnavailable, "ws_unavailable", "ws handler is not configured", true)
		return false
	}
	return true
}

func (h *APIHandler) writeHardwareError(c *gin.Context, status int, code, message string, retryable bool) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":      code,
			"message":   message,
			"retryable": retryable,
		},
	})
}

func (h *APIHandler) sendHardwareEventWithTimeout(ctx context.Context, eventType string, payload map[string]any, timeoutMs int) (string, error) {
	if timeoutMs <= 0 {
		timeoutMs = defaultHardwareDispatchTimeoutMs
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	type sendResult struct {
		sessionID string
		err       error
	}
	resCh := make(chan sendResult, 1)
	go func() {
		sid, err := h.wsHandler.SendControlEventToActive(eventType, payload)
		resCh <- sendResult{sessionID: sid, err: err}
	}()

	select {
	case <-timeoutCtx.Done():
		return "", fmt.Errorf("hardware dispatch timeout")
	case res := <-resCh:
		return res.sessionID, res.err
	}
}

func (h *APIHandler) respondHardwareDispatch(c *gin.Context, eventType, requestID, command string, timeoutMs int, payload map[string]any) {
	sessionID, err := h.sendHardwareEventWithTimeout(c.Request.Context(), eventType, payload, timeoutMs)
	if err != nil {
		errMessage := err.Error()
		errorCode := "dispatch_failed"
		statusCode := http.StatusBadGateway
		switch {
		case strings.Contains(errMessage, "timeout"):
			errorCode = "dispatch_timeout"
			statusCode = http.StatusGatewayTimeout
		case strings.Contains(errMessage, "no active Stackchan session"):
			errorCode = "stackchan_not_connected"
			statusCode = http.StatusConflict
		}
		h.writeHardwareError(c, statusCode, errorCode, errMessage, true)

		logging.Logger.Warn().
			Str("component", "hardware_dispatch").
			Str("event_type", eventType).
			Str("request_id", requestID).
			Str("command", command).
			Int("dispatch_timeout_ms", timeoutMs).
			Str("error_code", errorCode).
			Str("error_message", errMessage).
			Msg("hardware control dispatch failed")
		return
	}

	resp := gin.H{
		"status":              "sent",
		"event_type":          eventType,
		"request_id":          requestID,
		"target_session_id":   sessionID,
		"dispatch_timeout_ms": timeoutMs,
		"sent_at":             time.Now().UTC().Format(time.RFC3339),
	}
	if strings.TrimSpace(command) != "" {
		resp["command"] = command
	}

	logging.Logger.Info().
		Str("component", "hardware_dispatch").
		Str("event_type", eventType).
		Str("request_id", requestID).
		Str("command", command).
		Str("session_id", sessionID).
		Int("dispatch_timeout_ms", timeoutMs).
		Msg("hardware control dispatched")

	c.JSON(http.StatusOK, resp)
}

// RunHardwareServoTest はサーボ系の診断コマンドを active session へ送信します。
func (h *APIHandler) RunHardwareServoTest(c *gin.Context) {
	if !h.ensureHardwareWS(c) {
		return
	}

	var req HardwareServoTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeHardwareError(c, http.StatusBadRequest, "invalid_request", "invalid request body", false)
		return
	}

	reqID := strings.TrimSpace(req.RequestID)
	if reqID == "" {
		reqID = "hw-servo-" + uuid.NewString()
	}
	command := strings.TrimSpace(req.Command)
	if command == "" {
		command = "move"
	}

	payload := map[string]any{"request_id": reqID}
	eventType := ""

	switch command {
	case "move":
		eventType = hwEventServoMove
		axis := strings.TrimSpace(req.Axis)
		if axis == "" {
			h.writeHardwareError(c, http.StatusBadRequest, "invalid_request", "axis is required for servo move", false)
			return
		}
		payload["axis"] = axis
		if req.AngleXDeg != nil {
			payload["angle_x_deg"] = *req.AngleXDeg
		}
		if req.AngleYDeg != nil {
			payload["angle_y_deg"] = *req.AngleYDeg
		}
		if req.Speed != nil {
			payload["speed"] = *req.Speed
		}
	case "calibration_get":
		eventType = hwEventServoCalibrationGet
	case "calibration_set":
		eventType = hwEventServoCalibrationSet
		axis := strings.TrimSpace(req.Axis)
		if axis != "x" && axis != "y" {
			h.writeHardwareError(c, http.StatusBadRequest, "invalid_request", "axis must be x or y for calibration_set", false)
			return
		}
		payload["axis"] = axis
		if req.CenterOffsetDeg != nil {
			payload["center_offset_deg"] = *req.CenterOffsetDeg
		}
		if req.MinDeg != nil {
			payload["min_deg"] = *req.MinDeg
		}
		if req.MaxDeg != nil {
			payload["max_deg"] = *req.MaxDeg
		}
		if req.Invert != nil {
			payload["invert"] = *req.Invert
		}
		if req.SpeedLimitDegPerSec != nil {
			payload["speed_limit_deg_per_sec"] = *req.SpeedLimitDegPerSec
		}
		if req.SoftStart != nil {
			payload["soft_start"] = *req.SoftStart
		}
		if req.HomeDeg != nil {
			payload["home_deg"] = *req.HomeDeg
		}
	default:
		h.writeHardwareError(c, http.StatusBadRequest, "invalid_request", "command must be one of: move, calibration_get, calibration_set", false)
		return
	}

	h.respondHardwareDispatch(c, eventType, reqID, command, req.TimeoutMs, payload)
}

// RunHardwareLedTest は LED 診断イベントを active session へ送信します。
func (h *APIHandler) RunHardwareLedTest(c *gin.Context) {
	if !h.ensureHardwareWS(c) {
		return
	}

	var req HardwareLedTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeHardwareError(c, http.StatusBadRequest, "invalid_request", "invalid request body", false)
		return
	}

	reqID := strings.TrimSpace(req.RequestID)
	if reqID == "" {
		reqID = "hw-led-" + uuid.NewString()
	}
	payload := map[string]any{
		"request_id": reqID,
		"mode":       strings.TrimSpace(req.Mode),
	}
	if req.Color != "" {
		payload["color"] = strings.TrimSpace(req.Color)
	}
	if req.Brightness != nil {
		payload["brightness"] = *req.Brightness
	}
	if req.BlinkIntervalMs != nil {
		payload["blink_interval_ms"] = *req.BlinkIntervalMs
	}
	if req.BreathePeriodMs != nil {
		payload["breathe_period_ms"] = *req.BreathePeriodMs
	}

	h.respondHardwareDispatch(c, hwEventLedSet, reqID, "set", req.TimeoutMs, payload)
}

// RunHardwareEarsTest は耳 NeoPixel 診断イベントを active session へ送信します。
func (h *APIHandler) RunHardwareEarsTest(c *gin.Context) {
	if !h.ensureHardwareWS(c) {
		return
	}

	var req HardwareEarsTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeHardwareError(c, http.StatusBadRequest, "invalid_request", "invalid request body", false)
		return
	}

	reqID := strings.TrimSpace(req.RequestID)
	if reqID == "" {
		reqID = "hw-ears-" + uuid.NewString()
	}
	payload := map[string]any{
		"request_id": reqID,
		"mode":       strings.TrimSpace(req.Mode),
	}
	if req.Color != "" {
		payload["color"] = strings.TrimSpace(req.Color)
	}
	if req.Brightness != nil {
		payload["brightness"] = *req.Brightness
	}
	if req.BlinkIntervalMs != nil {
		payload["blink_interval_ms"] = *req.BlinkIntervalMs
	}
	if req.BreathePeriodMs != nil {
		payload["breathe_period_ms"] = *req.BreathePeriodMs
	}
	if req.RainbowPeriodMs != nil {
		payload["rainbow_period_ms"] = *req.RainbowPeriodMs
	}

	h.respondHardwareDispatch(c, hwEventEarsSet, reqID, "set", req.TimeoutMs, payload)
}

// RunHardwareAudioPlayTest はスピーカーテスト再生イベントを active session へ送信します。
func (h *APIHandler) RunHardwareAudioPlayTest(c *gin.Context) {
	if !h.ensureHardwareWS(c) {
		return
	}

	var req HardwareAudioPlayTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeHardwareError(c, http.StatusBadRequest, "invalid_request", "invalid request body", false)
		return
	}

	reqID := strings.TrimSpace(req.RequestID)
	if reqID == "" {
		reqID = "hw-audio-" + uuid.NewString()
	}
	payload := map[string]any{"request_id": reqID}
	if req.ToneHz != nil {
		payload["tone_hz"] = *req.ToneHz
	}
	if req.DurationMs != nil {
		payload["duration_ms"] = *req.DurationMs
	}
	if req.Volume != nil {
		payload["volume"] = *req.Volume
	}

	h.respondHardwareDispatch(c, hwEventAudioTestPlay, reqID, "play", req.TimeoutMs, payload)
}

// RunHardwareMicStartTest はマイクテスト開始イベントを active session へ送信します。
func (h *APIHandler) RunHardwareMicStartTest(c *gin.Context) {
	if !h.ensureHardwareWS(c) {
		return
	}

	var req HardwareMicStartTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeHardwareError(c, http.StatusBadRequest, "invalid_request", "invalid request body", false)
		return
	}

	reqID := strings.TrimSpace(req.RequestID)
	if reqID == "" {
		reqID = "hw-mic-" + uuid.NewString()
	}
	payload := map[string]any{"request_id": reqID}
	if req.DurationMs != nil {
		payload["duration_ms"] = *req.DurationMs
	}
	if req.SampleRateHz != nil {
		payload["sample_rate_hz"] = *req.SampleRateHz
	}
	if req.FrameDurationMs != nil {
		payload["frame_duration_ms"] = *req.FrameDurationMs
	}

	h.respondHardwareDispatch(c, hwEventMicTestStart, reqID, "start", req.TimeoutMs, payload)
}

// RunHardwareCameraCaptureTest はカメラ静止画取得イベントを active session へ送信します。
func (h *APIHandler) RunHardwareCameraCaptureTest(c *gin.Context) {
	if !h.ensureHardwareWS(c) {
		return
	}

	var req HardwareCameraCaptureTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.writeHardwareError(c, http.StatusBadRequest, "invalid_request", "invalid request body", false)
		return
	}

	reqID := strings.TrimSpace(req.RequestID)
	if reqID == "" {
		reqID = "hw-camera-" + uuid.NewString()
	}
	payload := map[string]any{"request_id": reqID}
	if strings.TrimSpace(req.Resolution) != "" {
		payload["resolution"] = strings.TrimSpace(req.Resolution)
	}
	if req.Quality != nil {
		payload["quality"] = *req.Quality
	}

	timeoutMs := req.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = defaultHardwareDispatchTimeoutMs
	}

	sessionID, err := h.sendHardwareEventWithTimeout(c.Request.Context(), hwEventCameraCapture, payload, timeoutMs)
	if err != nil {
		errMessage := err.Error()
		errorCode := "dispatch_failed"
		statusCode := http.StatusBadGateway
		switch {
		case strings.Contains(errMessage, "timeout"):
			errorCode = "dispatch_timeout"
			statusCode = http.StatusGatewayTimeout
		case strings.Contains(errMessage, "no active Stackchan session"):
			errorCode = "stackchan_not_connected"
			statusCode = http.StatusConflict
		}
		h.writeHardwareError(c, statusCode, errorCode, errMessage, true)
		return
	}

	result, err := h.wsHandler.AwaitCameraCaptureResult(reqID, timeoutMs)
	if err != nil {
		h.writeHardwareError(c, http.StatusGatewayTimeout, "camera_capture_timeout", err.Error(), true)
		return
	}

	if !result.OK {
		reason := strings.TrimSpace(result.Reason)
		if reason == "" {
			reason = "camera capture failed"
		}
		h.writeHardwareError(c, http.StatusBadGateway, "camera_capture_failed", reason, true)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":            "completed",
		"event_type":        hwEventCameraCapture,
		"request_id":        reqID,
		"target_session_id": sessionID,
		"command":           "capture",
		"result": gin.H{
			"ok":                   result.OK,
			"capture_id":           result.CaptureID,
			"captured_at_ms":       result.CapturedAtMs,
			"image_bytes":          result.ImageBytes,
			"width":                result.Width,
			"height":               result.Height,
			"requested_resolution": result.RequestedResolution,
			"requested_quality":    result.RequestedQuality,
			"camera_available":     result.CameraAvailable,
		},
		"timeout_ms": timeoutMs,
		"sent_at":     time.Now().UTC().Format(time.RFC3339),
	})
}

// GetHardwareState は診断向け state.report 要求を active session へ送信します。
func (h *APIHandler) GetHardwareState(c *gin.Context) {
	if !h.ensureHardwareWS(c) {
		return
	}

	reqID := strings.TrimSpace(c.Query("request_id"))
	if reqID == "" {
		reqID = "hw-state-" + uuid.NewString()
	}
	timeoutMs := 0
	if raw := strings.TrimSpace(c.Query("timeout_ms")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			timeoutMs = parsed
		}
	}
	payload := map[string]any{
		"request_id": reqID,
		"source":     "webui.hardware_test",
	}

	h.respondHardwareDispatch(c, hwEventStateReport, reqID, "report", timeoutMs, payload)
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

// GetLLMSettings は現在の LLM 設定を返します。
func (h *APIHandler) GetLLMSettings(c *gin.Context) {
	c.JSON(http.StatusOK, h.settingsStore.GetLLMSettings())
}

// UpdateLLMSettings は LLM 設定を更新します。
func (h *APIHandler) UpdateLLMSettings(c *gin.Context) {
	var req LLMSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	updated, err := h.settingsStore.UpdateLLMSettings(req)
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
	h.runtimeState.OnLLMTokenMetrics(result.RequestID, result.LLMInputTokens, result.LLMOutputTokens, result.LLMTotalTokens, result.LLMEffectiveTurns)
	h.runtimeState.OnPlaybackQueued(result.RequestID, time.Since(start).Milliseconds(), result.TTSDuration)
	h.runtimeState.OnPlaybackSent()
	h.runtimeState.OnPlaybackCompleted()

	c.JSON(http.StatusOK, gin.H{
		"request_id":                     result.RequestID,
		"transcript":                     result.Transcript,
		"reply_text":                     result.ReplyText,
		"provider_path":                  result.ProviderPath,
		"stt_latency_ms":                 result.STTLatencyMs,
		"llm_latency_ms":                 result.LLMLatencyMs,
		"llm_input_token_count":          result.LLMInputTokens,
		"llm_output_token_count":         result.LLMOutputTokens,
		"llm_total_token_count":          result.LLMTotalTokens,
		"llm_effective_turns_in_context": result.LLMEffectiveTurns,
		"tts_latency_ms":                 result.TTSLatencyMs,
		"total_latency_ms":               result.TotalLatencyMs,
		"duration_ms":                    result.TTSDuration,
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

// RunLLMUITest は WebUI から LLM 単体テストを実行します。
func (h *APIHandler) RunLLMUITest(c *gin.Context) {
	if h.orchestrator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "orchestrator is not configured"})
		return
	}

	var req LLMUITestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		text = "こんにちは、連携テストです。"
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		sessionID = "webui-llm-ui"
	}
	requestID := "llm-ui-" + uuid.NewString()

	res, err := h.orchestrator.ProcessText(c.Request.Context(), sessionID, requestID, text, strings.TrimSpace(req.PersonaOverride))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	h.runtimeState.OnPipeline(res.RequestID, "", 0, 0, res.LLMLatencyMs, res.TTSLatencyMs, res.TotalLatencyMs)
	h.runtimeState.OnLLMTokenMetrics(res.RequestID, res.LLMInputTokens, res.LLMOutputTokens, res.LLMTotalTokens, res.LLMEffectiveTurns)

	c.JSON(http.StatusOK, gin.H{
		"request_id":                     res.RequestID,
		"session_id":                     sessionID,
		"input_text":                     text,
		"reply_text":                     res.ReplyText,
		"llm_latency_ms":                 res.LLMLatencyMs,
		"llm_input_token_count":          res.LLMInputTokens,
		"llm_output_token_count":         res.LLMOutputTokens,
		"llm_total_token_count":          res.LLMTotalTokens,
		"llm_effective_turns_in_context": res.LLMEffectiveTurns,
	})
}

// RunLLMStackchanTest は LLM 応答を Voicevox で音声化して Stackchan へ送信します。
func (h *APIHandler) RunLLMStackchanTest(c *gin.Context) {
	if h.orchestrator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "orchestrator is not configured"})
		return
	}
	if h.wsHandler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ws handler is not configured"})
		return
	}

	var req LLMStackchanTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		text = "こんにちは、Stackchan 連携テストです。"
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		sessionID = "webui-llm-stackchan"
	}
	requestID := "llm-stackchan-" + uuid.NewString()

	llmRes, err := h.orchestrator.ProcessText(c.Request.Context(), sessionID, requestID, text, strings.TrimSpace(req.PersonaOverride))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	uiReq := VoicevoxUITestRequest{Text: llmRes.ReplyText, Speaker: req.Speaker}
	voiceRes, err := h.synthesizeVoicevox(c.Request.Context(), uiReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

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
	codec := strings.TrimSpace(req.Codec)
	if codec == "" {
		codec = "opus"
	}

	activeSessionID, err := h.wsHandler.SendTTSTestToActive(
		requestID,
		voiceRes.AudioBase64,
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

	h.runtimeState.OnPipeline(llmRes.RequestID, "", 0, 0, llmRes.LLMLatencyMs, llmRes.TTSLatencyMs, llmRes.TotalLatencyMs)
	h.runtimeState.OnLLMTokenMetrics(llmRes.RequestID, llmRes.LLMInputTokens, llmRes.LLMOutputTokens, llmRes.LLMTotalTokens, llmRes.LLMEffectiveTurns)
	h.runtimeState.OnPlaybackQueued(llmRes.RequestID, llmRes.TotalLatencyMs, llmRes.TTSDuration)
	h.runtimeState.OnPlaybackSent()
	h.runtimeState.OnPlaybackCompleted()

	c.JSON(http.StatusOK, gin.H{
		"request_id":                     requestID,
		"session_id":                     sessionID,
		"active_stackchan_session_id":    activeSessionID,
		"input_text":                     text,
		"reply_text":                     llmRes.ReplyText,
		"voicevox_latency_ms":            voiceRes.LatencyMs,
		"voicevox_bytes":                 voiceRes.Bytes,
		"llm_latency_ms":                 llmRes.LLMLatencyMs,
		"llm_input_token_count":          llmRes.LLMInputTokens,
		"llm_output_token_count":         llmRes.LLMOutputTokens,
		"llm_total_token_count":          llmRes.LLMTotalTokens,
		"llm_effective_turns_in_context": llmRes.LLMEffectiveTurns,
		"expression":                     expression,
		"motion":                         motion,
		"codec":                          codec,
		"chunk_version":                  chunkVersion,
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
