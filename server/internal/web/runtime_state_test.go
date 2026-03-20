package web

import (
	"testing"
	"time"

	"github.com/horitaku/stackchan/server/internal/protocol"
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

func TestRuntimeState_OnDeviceStateReportUpdatesSnapshot(t *testing.T) {
	state := NewRuntimeState()
	state.OnConnected("session-002")

	payload := protocol.DeviceStateReportPayload{
		RequestID:        "hw-state-001",
		Source:           "webui.hardware_test",
		UptimeMs:         123456,
		RSSI:             -52,
		FreeHeapBytes:    456789,
		CurrentAngleXDeg: 12.5,
		CurrentAngleYDeg: -7.5,
		Calibration: protocol.DeviceServoCalibrationBundle{
			X: protocol.DeviceServoAxisCalibration{MinDeg: -45, MaxDeg: 45},
			Y: protocol.DeviceServoAxisCalibration{MinDeg: -30, MaxDeg: 30},
		},
		MicLevel:        0.25,
		SpeakerBusy:     true,
		CameraAvailable: false,
		FirmwareVersion: "stackchan-cores3-01",
	}

	state.OnDeviceStateReport(payload)
	snapshot := state.Snapshot()

	if snapshot.Hardware.RequestID != payload.RequestID {
		t.Fatalf("expected request_id=%s, got %s", payload.RequestID, snapshot.Hardware.RequestID)
	}
	if snapshot.Hardware.RSSI != payload.RSSI {
		t.Fatalf("expected rssi=%d, got %d", payload.RSSI, snapshot.Hardware.RSSI)
	}
	if snapshot.Hardware.FreeHeapBytes != payload.FreeHeapBytes {
		t.Fatalf("expected free_heap_bytes=%d, got %d", payload.FreeHeapBytes, snapshot.Hardware.FreeHeapBytes)
	}
	if snapshot.Hardware.CurrentAngleXDeg != payload.CurrentAngleXDeg {
		t.Fatalf("expected current_angle_x_deg=%v, got %v", payload.CurrentAngleXDeg, snapshot.Hardware.CurrentAngleXDeg)
	}
	if snapshot.Hardware.CurrentAngleYDeg != payload.CurrentAngleYDeg {
		t.Fatalf("expected current_angle_y_deg=%v, got %v", payload.CurrentAngleYDeg, snapshot.Hardware.CurrentAngleYDeg)
	}
	if snapshot.Hardware.Calibration.X.MaxDeg != 45 || snapshot.Hardware.Calibration.Y.MaxDeg != 30 {
		t.Fatalf("expected calibration ranges to be mapped, got x=%v y=%v", snapshot.Hardware.Calibration.X.MaxDeg, snapshot.Hardware.Calibration.Y.MaxDeg)
	}
	if !snapshot.Hardware.SpeakerBusy {
		t.Fatalf("expected speaker_busy=true")
	}
	if snapshot.Hardware.LastReportAt == "" || snapshot.Hardware.LastReportAt == "-" {
		t.Fatalf("expected last_report_at to be set, got %q", snapshot.Hardware.LastReportAt)
	}
}
