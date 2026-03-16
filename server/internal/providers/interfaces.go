// Package providers は STT/LLM/TTS の provider 境界インターフェースを定義します。
package providers

import (
	"context"
	"time"
)

// AudioChunk は音声チャンクの最小メタデータです。
type AudioChunk struct {
	StreamID        string
	ChunkIndex      int
	Codec           string
	SampleRateHz    int
	FrameDurationMs int
	ChannelCount    int
	DataBase64      string
	// ReceivedAt はバイナリフレームをサーバーが受信した時刻です。
	// P8-10 の計測項目（first frame latency）に使用します。
	// JSON/wire プロトコルには含まれません（サーバー内部のみ）。
	ReceivedAt time.Time
}

// STTRequest は STT へ渡す入力です。
type STTRequest struct {
	SessionID string
	StreamID  string
	Chunks    []AudioChunk
}

// STTResult は STT の出力です。
type STTResult struct {
	Transcript string
}

// LLMRequest は LLM へ渡す入力です。
type LLMRequest struct {
	SessionID string
	Text      string
}

// LLMResult は LLM の出力です。
type LLMResult struct {
	ReplyText string
}

// TTSRequest は TTS へ渡す入力です。
type TTSRequest struct {
	SessionID string
	Text      string
}

// TTSResult は TTS の出力です。
type TTSResult struct {
	SampleRateHz int
	DurationMs   int
	AudioBase64  string
}

// STTProvider は音声入力をテキストへ変換します。
type STTProvider interface {
	Name() string
	Transcribe(ctx context.Context, req STTRequest) (STTResult, error)
}

// LLMProvider は入力テキストを応答テキストへ変換します。
type LLMProvider interface {
	Name() string
	Generate(ctx context.Context, req LLMRequest) (LLMResult, error)
}

// TTSProvider は応答テキストを音声へ変換します。
type TTSProvider interface {
	Name() string
	Synthesize(ctx context.Context, req TTSRequest) (TTSResult, error)
}
