package mock

import (
	"context"
	"encoding/base64"
	"math"
	"strings"

	"github.com/stackchan/server/internal/providers"
)

// TTS はテスト向け固定出力 TTS provider です。
type TTS struct{}

// Name は provider 名を返します。
func (m *TTS) Name() string { return "mock-tts" }

// Synthesize はテキストを疑似音声メタデータへ変換します。
func (m *TTS) Synthesize(_ context.Context, req providers.TTSRequest) (providers.TTSResult, error) {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return providers.TTSResult{}, providers.NewError(m.Name(), providers.CodeInvalidInput, "tts input text is empty", false, nil)
	}
	// Firmware 受信バッファ上限を超えないよう、フェーズ 6 の mock 音声は短尺固定にします。
	const durationMs = 300
	const sampleRateHz = 8000

	pcmBytes := makeSinePCM16(durationMs, sampleRateHz, 440.0)
	return providers.TTSResult{
		SampleRateHz: sampleRateHz,
		DurationMs:   durationMs,
		AudioBase64:  base64.StdEncoding.EncodeToString(pcmBytes),
	}, nil
}

func makeSinePCM16(durationMs, sampleRateHz int, freqHz float64) []byte {
	totalSamples := durationMs * sampleRateHz / 1000
	if totalSamples < 1 {
		totalSamples = sampleRateHz / 10
	}

	pcm := make([]byte, totalSamples*2)
	const amplitude = 0.22
	for i := 0; i < totalSamples; i++ {
		t := float64(i) / float64(sampleRateHz)
		s := math.Sin(2.0 * math.Pi * freqHz * t)
		v := int16(s * 32767.0 * amplitude)
		pcm[i*2] = byte(v & 0xff)
		pcm[i*2+1] = byte((uint16(v) >> 8) & 0xff)
	}
	return pcm
}
