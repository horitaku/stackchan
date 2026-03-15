package web

import (
	"sync"
	"time"
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
	mu       sync.RWMutex
	snapshot RuntimeSnapshot
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

func (s *RuntimeState) touchLocked() {
	s.snapshot.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

func (s *RuntimeState) OnConnected(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Connection.Status = "connected"
	s.snapshot.Connection.SessionID = sessionID
	s.snapshot.Connection.ConnectionCount++
	s.snapshot.Connection.LastConnectedAt = time.Now().UTC().Format(time.RFC3339)
	s.touchLocked()
}

func (s *RuntimeState) OnDisconnected() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Connection.Status = "disconnected"
	s.snapshot.Connection.ReconnectCount++
	s.snapshot.Connection.LastDisconnectedAt = time.Now().UTC().Format(time.RFC3339)
	s.touchLocked()
}

func (s *RuntimeState) OnHeartbeat() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Connection.LastHeartbeatAt = time.Now().UTC().Format(time.RFC3339)
	s.touchLocked()
}

func (s *RuntimeState) OnPipeline(requestID, streamID string, queueWaitMs int64, sttMs, llmMs, ttsMs, totalMs int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Pipeline.RequestID = requestID
	s.snapshot.Pipeline.StreamID = streamID
	s.snapshot.Pipeline.QueueWaitMs = queueWaitMs
	s.snapshot.Pipeline.STTLatencyMs = sttMs
	s.snapshot.Pipeline.LLMLatencyMs = llmMs
	s.snapshot.Pipeline.TTSLatencyMs = ttsMs
	s.snapshot.Pipeline.TotalLatencyMs = totalMs
	s.touchLocked()
}

func (s *RuntimeState) OnPlaybackQueued(requestID string, startLatencyMs int64, durationMs int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Playback.State = "buffering"
	s.snapshot.Playback.RequestID = requestID
	s.snapshot.Playback.PlaybackStartLatencyMs = startLatencyMs
	s.snapshot.Playback.PlaybackDurationMs = durationMs
	s.touchLocked()
}

func (s *RuntimeState) OnPlaybackSent() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Playback.State = "playing"
	s.touchLocked()
}

func (s *RuntimeState) OnPlaybackCompleted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Playback.State = "idle"
	s.touchLocked()
}

func (s *RuntimeState) OnDecodeError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Playback.DecodeErrorCount++
	s.snapshot.Playback.State = "error"
	s.touchLocked()
}

func (s *RuntimeState) OnOutputError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Playback.OutputErrorCount++
	s.snapshot.Playback.State = "error"
	s.touchLocked()
}

func (s *RuntimeState) OnAvatarExpression(expression string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Avatar.Expression = expression
	s.touchLocked()
}

func (s *RuntimeState) OnAvatarMotion(motion string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Avatar.Motion = motion
	s.touchLocked()
}

// Snapshot は現在状態のコピーを返します。
func (s *RuntimeState) Snapshot() RuntimeSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}
