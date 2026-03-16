package audio

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestBuildWAV は BuildWAV が有効な WAV ヘッダを生成するかを検証します。
func TestBuildWAV(t *testing.T) {
	// 16kHz、モノラル、100ms 分のゼロ PCM（640 bytes = 16000 * 0.1 * 1 * 2）
	pcm := make([]byte, 640)
	wav := BuildWAV(pcm, 16000, 1)

	// ── RIFF チャンク ────────────────────────────────────────────────────
	t.Run("RIFF_signature", func(t *testing.T) {
		if !bytes.HasPrefix(wav, []byte("RIFF")) {
			t.Errorf("WAV does not start with 'RIFF'")
		}
	})

	t.Run("WAVE_signature", func(t *testing.T) {
		if string(wav[8:12]) != "WAVE" {
			t.Errorf("WAV[8:12] = %q, want 'WAVE'", string(wav[8:12]))
		}
	})

	t.Run("RIFF_chunk_size", func(t *testing.T) {
		// RIFF チャンクサイズ = 36 + len(pcm)
		want := uint32(36 + len(pcm))
		got := binary.LittleEndian.Uint32(wav[4:8])
		if got != want {
			t.Errorf("RIFF size = %d, want %d", got, want)
		}
	})

	// ── fmt サブチャンク ─────────────────────────────────────────────────
	t.Run("fmt_signature", func(t *testing.T) {
		if string(wav[12:16]) != "fmt " {
			t.Errorf("WAV[12:16] = %q, want 'fmt '", string(wav[12:16]))
		}
	})

	t.Run("audio_format_PCM", func(t *testing.T) {
		// AudioFormat = 1 (PCM), オフセット 20
		got := binary.LittleEndian.Uint16(wav[20:22])
		if got != 1 {
			t.Errorf("AudioFormat = %d, want 1 (PCM)", got)
		}
	})

	t.Run("sample_rate_recorded", func(t *testing.T) {
		// SampleRate, オフセット 24
		got := binary.LittleEndian.Uint32(wav[24:28])
		if got != 16000 {
			t.Errorf("SampleRate = %d, want 16000", got)
		}
	})

	t.Run("channel_count", func(t *testing.T) {
		// NumChannels, オフセット 22
		got := binary.LittleEndian.Uint16(wav[22:24])
		if got != 1 {
			t.Errorf("NumChannels = %d, want 1", got)
		}
	})

	// ── data サブチャンク ────────────────────────────────────────────────
	t.Run("data_signature", func(t *testing.T) {
		if string(wav[36:40]) != "data" {
			t.Errorf("WAV[36:40] = %q, want 'data'", string(wav[36:40]))
		}
	})

	t.Run("data_size", func(t *testing.T) {
		got := binary.LittleEndian.Uint32(wav[40:44])
		if got != uint32(len(pcm)) {
			t.Errorf("data size = %d, want %d", got, len(pcm))
		}
	})

	t.Run("total_size", func(t *testing.T) {
		// WAV トータルサイズ = 44 (ヘッダ) + len(pcm)
		want := 44 + len(pcm)
		if len(wav) != want {
			t.Errorf("WAV total size = %d, want %d", len(wav), want)
		}
	})
}

// TestBuildWAVEmpty は空の PCM データでクラッシュしないことを確認します。
func TestBuildWAVEmpty(t *testing.T) {
	wav := BuildWAV(nil, 16000, 1)
	// 空データでも最低限のヘッダ（44 bytes）が生成されます
	if len(wav) < 44 {
		t.Errorf("empty WAV size = %d, want >= 44", len(wav))
	}
}
