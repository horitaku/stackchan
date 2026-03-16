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
	StreamID       string `json:"stream_id,omitempty"`
	RequestID      string `json:"request_id,omitempty"`
	QueueWaitMs    int64  `json:"queue_wait_ms"`
	STTLatencyMs   int64  `json:"stt_latency_ms"`
	LLMLatencyMs   int64  `json:"llm_latency_ms"`
	TTSLatencyMs   int64  `json:"tts_latency_ms"`
	TotalLatencyMs int64  `json:"total_latency_ms"`
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
