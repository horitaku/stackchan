package mock

import (
	"context"
	"fmt"
	"time"

	"github.com/horitaku/stackchan/server/internal/providers"
)

// STT はテスト向け固定応答 STT provider です。
type STT struct {
	Delay time.Duration
}

// Name は provider 名を返します。
func (m *STT) Name() string { return "mock-stt" }

// Transcribe は入力チャンク数に応じた決定的な文字列を返します。
func (m *STT) Transcribe(ctx context.Context, req providers.STTRequest) (providers.STTResult, error) {
	if req.StreamID == "" {
		return providers.STTResult{}, providers.NewError(m.Name(), providers.CodeInvalidInput, "stream_id is required", false, nil)
	}
	if m.Delay > 0 {
		timer := time.NewTimer(m.Delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return providers.STTResult{}, providers.NewError(m.Name(), providers.CodeTimeout, "stt timeout", true, ctx.Err())
		case <-timer.C:
		}
	}
	return providers.STTResult{Transcript: fmt.Sprintf("transcript:%s:chunks=%d", req.StreamID, len(req.Chunks))}, nil
}
