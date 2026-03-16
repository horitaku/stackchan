package audio

import (
	"strings"
	"testing"
)

func TestExpectedPCMFrameBytes(t *testing.T) {
	tests := []struct {
		name            string
		codec           string
		sampleRateHz    int
		frameDurationMs int
		channelCount    int
		want            int
	}{
		{
			name:            "16kHz 20ms mono (標準設定)",
			codec:           CodecPCM,
			sampleRateHz:    16000,
			frameDurationMs: 20,
			channelCount:    1,
			want:            640, // 16000 * 20/1000 * 1 * 2
		},
		{
			name:            "16kHz 10ms mono",
			codec:           CodecPCM,
			sampleRateHz:    16000,
			frameDurationMs: 10,
			channelCount:    1,
			want:            320,
		},
		{
			name:            "codec=opus は -1",
			codec:           CodecOpus,
			sampleRateHz:    16000,
			frameDurationMs: 20,
			channelCount:    1,
			want:            -1,
		},
		{
			name:            "sampleRateHz=0 は -1",
			codec:           CodecPCM,
			sampleRateHz:    0,
			frameDurationMs: 20,
			channelCount:    1,
			want:            -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpectedPCMFrameBytes(tt.codec, tt.sampleRateHz, tt.frameDurationMs, tt.channelCount)
			if got != tt.want {
				t.Errorf("ExpectedPCMFrameBytes() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestValidateCodec(t *testing.T) {
	if err := ValidateCodec(CodecPCM); err != nil {
		t.Errorf("ValidateCodec(%q) unexpected error: %v", CodecPCM, err)
	}
	if err := ValidateCodec(CodecOpus); err != nil {
		t.Errorf("ValidateCodec(%q) unexpected error: %v", CodecOpus, err)
	}
	if err := ValidateCodec("mp3"); err == nil {
		t.Error("ValidateCodec(\"mp3\") expected error, got nil")
	}
}

func TestValidateFramePayload(t *testing.T) {
	tests := []struct {
		name          string
		codec         string
		payloadBytes  int
		expectedBytes int
		wantOK        bool
		warnContains  string
	}{
		{
			name:          "PCM サイズ一致",
			codec:         CodecPCM,
			payloadBytes:  640,
			expectedBytes: 640,
			wantOK:        true,
		},
		{
			name:          "PCM サイズ不一致",
			codec:         CodecPCM,
			payloadBytes:  320,
			expectedBytes: 640,
			wantOK:        false,
			warnContains:  "mismatch",
		},
		{
			name:          "PCM expected<=0 はスキップ",
			codec:         CodecPCM,
			payloadBytes:  100,
			expectedBytes: 0,
			wantOK:        true,
		},
		{
			name:          "Opus 通常サイズ",
			codec:         CodecOpus,
			payloadBytes:  60,
			expectedBytes: 0,
			wantOK:        true,
		},
		{
			name:          "Opus 小さすぎる",
			codec:         CodecOpus,
			payloadBytes:  2,
			expectedBytes: 0,
			wantOK:        false,
			warnContains:  "small",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, warn := ValidateFramePayload(tt.codec, tt.payloadBytes, tt.expectedBytes)
			if ok != tt.wantOK {
				t.Errorf("ValidateFramePayload() ok=%v, want %v (warn=%q)", ok, tt.wantOK, warn)
			}
			if tt.warnContains != "" && !strings.Contains(warn, tt.warnContains) {
				t.Errorf("ValidateFramePayload() warn=%q, want to contain %q", warn, tt.warnContains)
			}
		})
	}
}

func TestOpus48HzSamplesPerFrame(t *testing.T) {
	tests := []struct {
		frameDurationMs int
		want            int64
	}{
		{20, 960},
		{10, 480},
		{40, 1920},
	}
	for _, tt := range tests {
		got := Opus48HzSamplesPerFrame(tt.frameDurationMs)
		if got != tt.want {
			t.Errorf("Opus48HzSamplesPerFrame(%d) = %d, want %d", tt.frameDurationMs, got, tt.want)
		}
	}
}
