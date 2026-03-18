package web

import (
	"sync"
	"time"

	"github.com/horitaku/stackchan/server/internal/logging"
)

// RuntimeSnapshot は WebUI/API 向けに公開するランタイム状態のスナップショットです。
type RuntimeSnapshot struct {
	Connection ConnectionSnapshot `json:"connection"`
	Playback   PlaybackSnapshot   `json:"playback"`
	Pipeline   PipelineSnapshot   `json:"pipeline"`
	Avatar     AvatarSnapshot     `json:"avatar"`
	UpdatedAt  string             `json:"updated_at"`
}

// ConnectionSnapshot は接続・セッション関連の状態です。
type ConnectionSnapshot struct {
	Status              string `json:"status"`
	SessionID           string `json:"session_id"`
	ConnectionCount     int    `json:"connection_count"`
	ReconnectCount      int    `json:"reconnect_count"`
	HeartbeatIntervalMs int    `json:"heartbeat_interval_ms"`
	LastHeartbeatAt     string `json:"last_heartbeat_at,omitempty"`
	LastConnectedAt     string `json:"last_connected_at,omitempty"`
	LastDisconnectedAt  string `json:"last_disconnected_at,omitempty"`
}

// PlaybackSnapshot は再生関連の状態です。
type PlaybackSnapshot struct {
	State                  string `json:"state"`
	RequestID              string `json:"request_id,omitempty"`
	PlaybackStartLatencyMs int64  `json:"playback_start_latency_ms"`
	PlaybackDurationMs     int    `json:"playback_duration_ms"`
	DecodeErrorCount       int    `json:"decode_error_count"`
	OutputErrorCount       int    `json:"output_error_count"`
}

// PipelineSnapshot は会話パイプラインの遅延情報です。
type PipelineSnapshot struct {
	StreamID                   string `json:"stream_id,omitempty"`
	RequestID                  string `json:"request_id,omitempty"`
	QueueWaitMs                int64  `json:"queue_wait_ms"`
	STTLatencyMs               int64  `json:"stt_latency_ms"`
	LLMLatencyMs               int64  `json:"llm_latency_ms"`
	TTSLatencyMs               int64  `json:"tts_latency_ms"`
	TotalLatencyMs             int64  `json:"total_latency_ms"`
	FirstFrameLatencyMs        int64  `json:"first_frame_latency_ms"`
	CadenceJitterMs            int64  `json:"cadence_jitter_ms"`
	E2ELatencyMs               int64  `json:"e2e_latency_ms"`
	LLMInputTokenCount         int    `json:"llm_input_token_count"`
	LLMOutputTokenCount        int    `json:"llm_output_token_count"`
	LLMTotalTokenCount         int    `json:"llm_total_token_count"`
	LLMEffectiveTurnsInContext int    `json:"llm_effective_turns_in_context"`
	// P8-16: tts.chunk 送信失敗の累積カウント（downlink 配信異常の検知指標）
	TTSChunkSendFailCount int `json:"tts_chunk_send_fail_count"`
	// P8-19: TTS バッファ watermark 統計
	TTSWatermarkStatus    string `json:"tts_watermark_status"`      // "normal" | "low_water" | "high_water"
	TTSBufferedMs         int    `json:"tts_buffered_ms"`           // 最新バッファ深さ（ms）
	TTSLowWaterCount      int    `json:"tts_low_water_count"`       // low-water 累積カウント
	TTSHighWaterDropCount int    `json:"tts_high_water_drop_count"` // high-water ドロップ累積カウント
}

// AvatarSnapshot はアバター同期の状態です。
type AvatarSnapshot struct {
	Expression              string  `json:"expression"`
	Motion                  string  `json:"motion"`
	LipSyncLevel            float64 `json:"lip_sync_level"`
	LipSyncUpdateIntervalMs int     `json:"lip_sync_update_interval_ms"`
}

// RuntimeState は WebSocket ハンドラで観測した状態を集約するスレッドセーフなストアです。
type RuntimeState struct {
	mu           sync.RWMutex
	snapshot     RuntimeSnapshot
	metricsStore *RuntimeMetricsStore
}

// NewRuntimeState は初期値入りの RuntimeState を返します。
func NewRuntimeState() *RuntimeState {
	return &RuntimeState{
		snapshot: RuntimeSnapshot{
			Connection: ConnectionSnapshot{
				Status:              "disconnected",
				HeartbeatIntervalMs: 15000,
			},
			Playback: PlaybackSnapshot{State: "idle"},
			Avatar: AvatarSnapshot{
				Expression:              "neutral",
				Motion:                  "idle",
				LipSyncUpdateIntervalMs: 50,
			},
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}
}

// SetMetricsStore は runtime_metrics 保存先を設定します。
func (s *RuntimeState) SetMetricsStore(store *RuntimeMetricsStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metricsStore = store
}

func (s *RuntimeState) touchLocked() {
	s.snapshot.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func (s *RuntimeState) OnConnected(sessionID string) {
	now := time.Now().UTC()
	s.mu.Lock()
	s.snapshot.Connection.Status = "connected"
	s.snapshot.Connection.SessionID = sessionID
	s.snapshot.Connection.ConnectionCount++
	s.snapshot.Connection.LastConnectedAt = now.Format(time.RFC3339)
	s.touchLocked()
	store := s.metricsStore
	metrics := []RuntimeMetricWrite{
		{SessionID: sessionID, MetricName: "connection.connection_count", MetricValue: float64(s.snapshot.Connection.ConnectionCount), MetricUnit: "count", ObservedAt: now},
		{SessionID: sessionID, MetricName: "connection.reconnect_count", MetricValue: float64(s.snapshot.Connection.ReconnectCount), MetricUnit: "count", ObservedAt: now},
	}
	s.mu.Unlock()
	s.persistMetrics(store, metrics)
}

func (s *RuntimeState) OnDisconnected() {
	now := time.Now().UTC()
	s.mu.Lock()
	sessionID := s.snapshot.Connection.SessionID
	s.snapshot.Connection.Status = "disconnected"
	s.snapshot.Connection.ReconnectCount++
	s.snapshot.Connection.LastDisconnectedAt = now.Format(time.RFC3339)
	s.touchLocked()
	store := s.metricsStore
	metrics := []RuntimeMetricWrite{
		{SessionID: sessionID, MetricName: "connection.reconnect_count", MetricValue: float64(s.snapshot.Connection.ReconnectCount), MetricUnit: "count", ObservedAt: now},
	}
	s.mu.Unlock()
	s.persistMetrics(store, metrics)
}

func (s *RuntimeState) OnHeartbeat() {
	now := time.Now().UTC()
	s.mu.Lock()
	s.snapshot.Connection.LastHeartbeatAt = now.Format(time.RFC3339)
	s.touchLocked()
	store := s.metricsStore
	sessionID := s.snapshot.Connection.SessionID
	metrics := []RuntimeMetricWrite{
		{SessionID: sessionID, MetricName: "connection.heartbeat_interval_ms", MetricValue: float64(s.snapshot.Connection.HeartbeatIntervalMs), MetricUnit: "ms", ObservedAt: now},
	}
	s.mu.Unlock()
	s.persistMetrics(store, metrics)
}

func (s *RuntimeState) OnPipeline(requestID, streamID string, queueWaitMs int64, sttMs, llmMs, ttsMs, totalMs int64) {
	now := time.Now().UTC()
	s.mu.Lock()
	sessionID := s.snapshot.Connection.SessionID
	s.snapshot.Pipeline.RequestID = requestID
	s.snapshot.Pipeline.StreamID = streamID
	s.snapshot.Pipeline.QueueWaitMs = queueWaitMs
	s.snapshot.Pipeline.STTLatencyMs = sttMs
	s.snapshot.Pipeline.LLMLatencyMs = llmMs
	s.snapshot.Pipeline.TTSLatencyMs = ttsMs
	s.snapshot.Pipeline.TotalLatencyMs = totalMs
	s.touchLocked()
	store := s.metricsStore
	metrics := []RuntimeMetricWrite{
		{SessionID: sessionID, RequestID: requestID, MetricName: "pipeline.queue_wait_ms", MetricValue: float64(queueWaitMs), MetricUnit: "ms", ObservedAt: now},
		{SessionID: sessionID, RequestID: requestID, MetricName: "pipeline.stt_latency_ms", MetricValue: float64(sttMs), MetricUnit: "ms", ObservedAt: now},
		{SessionID: sessionID, RequestID: requestID, MetricName: "pipeline.llm_latency_ms", MetricValue: float64(llmMs), MetricUnit: "ms", ObservedAt: now},
		{SessionID: sessionID, RequestID: requestID, MetricName: "pipeline.tts_latency_ms", MetricValue: float64(ttsMs), MetricUnit: "ms", ObservedAt: now},
		{SessionID: sessionID, RequestID: requestID, MetricName: "pipeline.total_latency_ms", MetricValue: float64(totalMs), MetricUnit: "ms", ObservedAt: now},
	}
	s.mu.Unlock()
	s.persistMetrics(store, metrics)
}

// OnLLMTokenMetrics は LLM の token 使用量と context 有効ターン数を記録します。
func (s *RuntimeState) OnLLMTokenMetrics(requestID string, inputTokens, outputTokens, totalTokens, effectiveTurns int) {
	now := time.Now().UTC()
	s.mu.Lock()
	sessionID := s.snapshot.Connection.SessionID
	s.snapshot.Pipeline.RequestID = requestID
	s.snapshot.Pipeline.LLMInputTokenCount = inputTokens
	s.snapshot.Pipeline.LLMOutputTokenCount = outputTokens
	s.snapshot.Pipeline.LLMTotalTokenCount = totalTokens
	s.snapshot.Pipeline.LLMEffectiveTurnsInContext = effectiveTurns
	s.touchLocked()
	store := s.metricsStore
	metrics := []RuntimeMetricWrite{
		{SessionID: sessionID, RequestID: requestID, MetricName: "llm_input_token_count", MetricValue: float64(inputTokens), MetricUnit: "tokens", ObservedAt: now},
		{SessionID: sessionID, RequestID: requestID, MetricName: "llm_output_token_count", MetricValue: float64(outputTokens), MetricUnit: "tokens", ObservedAt: now},
		{SessionID: sessionID, RequestID: requestID, MetricName: "llm_total_token_count", MetricValue: float64(totalTokens), MetricUnit: "tokens", ObservedAt: now},
		{SessionID: sessionID, RequestID: requestID, MetricName: "llm_effective_turns_in_context", MetricValue: float64(effectiveTurns), MetricUnit: "count", ObservedAt: now},
	}
	s.mu.Unlock()
	s.persistMetrics(store, metrics)
}

func (s *RuntimeState) OnOpusMetrics(requestID, streamID string, firstFrameLatencyMs, cadenceJitterMs, e2eLatencyMs int64) {
	now := time.Now().UTC()
	s.mu.Lock()
	sessionID := s.snapshot.Connection.SessionID
	s.snapshot.Pipeline.RequestID = requestID
	s.snapshot.Pipeline.StreamID = streamID
	s.snapshot.Pipeline.FirstFrameLatencyMs = firstFrameLatencyMs
	s.snapshot.Pipeline.CadenceJitterMs = cadenceJitterMs
	s.snapshot.Pipeline.E2ELatencyMs = e2eLatencyMs
	s.touchLocked()
	store := s.metricsStore
	metrics := []RuntimeMetricWrite{
		{SessionID: sessionID, RequestID: requestID, MetricName: "pipeline.first_frame_latency_ms", MetricValue: float64(firstFrameLatencyMs), MetricUnit: "ms", ObservedAt: now},
		{SessionID: sessionID, RequestID: requestID, MetricName: "pipeline.cadence_jitter_ms", MetricValue: float64(cadenceJitterMs), MetricUnit: "ms", ObservedAt: now},
		{SessionID: sessionID, RequestID: requestID, MetricName: "pipeline.e2e_latency_ms", MetricValue: float64(e2eLatencyMs), MetricUnit: "ms", ObservedAt: now},
	}
	s.mu.Unlock()
	s.persistMetrics(store, metrics)
}

func (s *RuntimeState) OnPlaybackQueued(requestID string, startLatencyMs int64, durationMs int) {
	now := time.Now().UTC()
	s.mu.Lock()
	sessionID := s.snapshot.Connection.SessionID
	s.snapshot.Playback.State = "buffering"
	s.snapshot.Playback.RequestID = requestID
	s.snapshot.Playback.PlaybackStartLatencyMs = startLatencyMs
	s.snapshot.Playback.PlaybackDurationMs = durationMs
	s.touchLocked()
	store := s.metricsStore
	metrics := []RuntimeMetricWrite{
		{SessionID: sessionID, RequestID: requestID, MetricName: "playback.playback_start_latency_ms", MetricValue: float64(startLatencyMs), MetricUnit: "ms", ObservedAt: now},
		{SessionID: sessionID, RequestID: requestID, MetricName: "playback.playback_duration_ms", MetricValue: float64(durationMs), MetricUnit: "ms", ObservedAt: now},
	}
	s.mu.Unlock()
	s.persistMetrics(store, metrics)
}

func (s *RuntimeState) OnPlaybackSent() {
	s.mu.Lock()
	s.snapshot.Playback.State = "playing"
	s.touchLocked()
	s.mu.Unlock()
}

func (s *RuntimeState) OnPlaybackCompleted() {
	s.mu.Lock()
	s.snapshot.Playback.State = "idle"
	s.touchLocked()
	s.mu.Unlock()
}

func (s *RuntimeState) OnDecodeError() {
	now := time.Now().UTC()
	s.mu.Lock()
	sessionID := s.snapshot.Connection.SessionID
	requestID := s.snapshot.Playback.RequestID
	s.snapshot.Playback.DecodeErrorCount++
	s.snapshot.Playback.State = "error"
	s.touchLocked()
	store := s.metricsStore
	metrics := []RuntimeMetricWrite{
		{SessionID: sessionID, RequestID: requestID, MetricName: "playback.decode_error_count", MetricValue: float64(s.snapshot.Playback.DecodeErrorCount), MetricUnit: "count", ObservedAt: now},
	}
	s.mu.Unlock()
	s.persistMetrics(store, metrics)
}

func (s *RuntimeState) OnOutputError() {
	now := time.Now().UTC()
	s.mu.Lock()
	sessionID := s.snapshot.Connection.SessionID
	requestID := s.snapshot.Playback.RequestID
	s.snapshot.Playback.OutputErrorCount++
	s.snapshot.Playback.State = "error"
	s.touchLocked()
	store := s.metricsStore
	metrics := []RuntimeMetricWrite{
		{SessionID: sessionID, RequestID: requestID, MetricName: "playback.output_error_count", MetricValue: float64(s.snapshot.Playback.OutputErrorCount), MetricUnit: "count", ObservedAt: now},
	}
	s.mu.Unlock()
	s.persistMetrics(store, metrics)
}

// OnTTSChunkSendFail は tts.chunk 送信失敗を記録します（P8-16 downlink 欠落検知指標）。
func (s *RuntimeState) OnTTSChunkSendFail(requestID, streamID string) {
	now := time.Now().UTC()
	s.mu.Lock()
	sessionID := s.snapshot.Connection.SessionID
	s.snapshot.Pipeline.RequestID = requestID
	s.snapshot.Pipeline.StreamID = streamID
	s.snapshot.Pipeline.TTSChunkSendFailCount++
	s.touchLocked()
	store := s.metricsStore
	metrics := []RuntimeMetricWrite{
		{SessionID: sessionID, RequestID: requestID, MetricName: "pipeline.tts_chunk_send_fail_count", MetricValue: float64(s.snapshot.Pipeline.TTSChunkSendFailCount), MetricUnit: "count", ObservedAt: now},
	}
	s.mu.Unlock()
	s.persistMetrics(store, metrics)
}

// OnTTSWatermark は firmware から通知された TTS バッファ watermark 状態変化を記録します（P8-19）。
// status: "normal" | "low_water" | "high_water"
func (s *RuntimeState) OnTTSWatermark(requestID, streamID, status string, bufferedMs, thresholdMs, framesInQueue int) {
	now := time.Now().UTC()
	s.mu.Lock()
	sessionID := s.snapshot.Connection.SessionID
	s.snapshot.Pipeline.RequestID = requestID
	s.snapshot.Pipeline.StreamID = streamID
	s.snapshot.Pipeline.TTSWatermarkStatus = status
	s.snapshot.Pipeline.TTSBufferedMs = bufferedMs
	switch status {
	case "low_water":
		s.snapshot.Pipeline.TTSLowWaterCount++
	case "high_water":
		s.snapshot.Pipeline.TTSHighWaterDropCount++
	}
	s.touchLocked()
	store := s.metricsStore
	lowWaterCount := s.snapshot.Pipeline.TTSLowWaterCount
	highWaterDropCount := s.snapshot.Pipeline.TTSHighWaterDropCount
	s.mu.Unlock()
	s.persistMetrics(store, []RuntimeMetricWrite{
		{SessionID: sessionID, RequestID: requestID, MetricName: "pipeline.tts_buffer_watermark_status", MetricValue: 0, MetricUnit: "enum:" + status, ObservedAt: now},
		{SessionID: sessionID, RequestID: requestID, MetricName: "pipeline.tts_buffered_ms", MetricValue: float64(bufferedMs), MetricUnit: "ms", ObservedAt: now},
		{SessionID: sessionID, RequestID: requestID, MetricName: "pipeline.tts_low_water_count", MetricValue: float64(lowWaterCount), MetricUnit: "count", ObservedAt: now},
		{SessionID: sessionID, RequestID: requestID, MetricName: "pipeline.tts_high_water_drop_count", MetricValue: float64(highWaterDropCount), MetricUnit: "count", ObservedAt: now},
	})
	logging.Logger.Debug().
		Str("session_id", sessionID).
		Str("request_id", requestID).
		Str("stream_id", streamID).
		Str("status", status).
		Int("buffered_ms", bufferedMs).
		Int("threshold_ms", thresholdMs).
		Int("frames_in_queue", framesInQueue).
		Int("low_water_count", lowWaterCount).
		Int("high_water_drop_count", highWaterDropCount).
		Msg("tts buffer watermark received")
}

func (s *RuntimeState) OnAvatarExpression(expression string) {
	s.mu.Lock()
	s.snapshot.Avatar.Expression = expression
	s.touchLocked()
	s.mu.Unlock()
}

func (s *RuntimeState) OnAvatarMotion(motion string) {
	s.mu.Lock()
	s.snapshot.Avatar.Motion = motion
	s.touchLocked()
	s.mu.Unlock()
}

// Snapshot は現在状態のコピーを返します。
func (s *RuntimeState) Snapshot() RuntimeSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

func (s *RuntimeState) persistMetrics(store *RuntimeMetricsStore, metrics []RuntimeMetricWrite) {
	if store == nil || len(metrics) == 0 {
		return
	}
	if err := store.InsertMetrics(metrics); err != nil {
		logging.Logger.Warn().Err(err).Msg("failed to persist runtime metrics")
	}
}
