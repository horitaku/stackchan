package web

import (
	"testing"
	"time"

	"github.com/horitaku/stackchan/server/internal/providers"
)

func TestRuntimeState_OnOpusMetricsUpdatesSnapshot(t *testing.T) {
	state := NewRuntimeState()
	state.OnConnected("session-001")
	state.OnOpusMetrics("request-001", "stream-001", 42, 3, 280)

	snapshot := state.Snapshot()
	if snapshot.Pipeline.RequestID != "request-001" {
		t.Fatalf("expected request_id=request-001, got %s", snapshot.Pipeline.RequestID)
	}
	if snapshot.Pipeline.StreamID != "stream-001" {
		t.Fatalf("expected stream_id=stream-001, got %s", snapshot.Pipeline.StreamID)
	}
	if snapshot.Pipeline.FirstFrameLatencyMs != 42 {
		t.Fatalf("expected first_frame_latency_ms=42, got %d", snapshot.Pipeline.FirstFrameLatencyMs)
	}
	if snapshot.Pipeline.CadenceJitterMs != 3 {
		t.Fatalf("expected cadence_jitter_ms=3, got %d", snapshot.Pipeline.CadenceJitterMs)
	}
	if snapshot.Pipeline.E2ELatencyMs != 280 {
		t.Fatalf("expected e2e_latency_ms=280, got %d", snapshot.Pipeline.E2ELatencyMs)
	}
}

func TestCalculateCadenceJitterMs(t *testing.T) {
	chunks := []struct {
		receivedAt string
	}{
		{receivedAt: "2026-03-18T10:00:00.000Z"},
		{receivedAt: "2026-03-18T10:00:00.020Z"},
		{receivedAt: "2026-03-18T10:00:00.043Z"},
		{receivedAt: "2026-03-18T10:00:00.062Z"},
	}

	parsed := make([]providers.AudioChunk, 0, len(chunks))
	for _, c := range chunks {
		tm, err := time.Parse(time.RFC3339Nano, c.receivedAt)
		if err != nil {
			t.Fatalf("failed to parse time: %v", err)
		}
		parsed = append(parsed, providers.AudioChunk{ReceivedAt: tm})
	}

	got := calculateCadenceJitterMs(parsed, 20)
	if got != 1 {
		t.Fatalf("expected cadence jitter=1, got %d", got)
	}
}
