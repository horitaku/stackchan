package mock

import (
	"context"
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
	durationMs := len([]rune(text)) * 60
	if durationMs < 300 {
		durationMs = 300
	}
	return providers.TTSResult{
		SampleRateHz: 24000,
		DurationMs:   durationMs,
		AudioBase64:  "bW9jay10dHMtYXVkaW8=",
	}, nil
}
