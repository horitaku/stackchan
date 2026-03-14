// Package conversation は STT/LLM/TTS を接続する最小オーケストレーションを提供します。
package conversation

import (
	"context"
	"fmt"
	"time"

	"github.com/stackchan/server/internal/logging"
	"github.com/stackchan/server/internal/providers"
)

// Orchestrator は provider 群を束ねて最小会話フローを実行します。
type Orchestrator struct {
	stt    providers.STTProvider
	llm    providers.LLMProvider
	tts    providers.TTSProvider
	policy providers.CallPolicy
}

// Result は最小会話フローの結果です。
type Result struct {
	RequestID      string
	Transcript     string
	ReplyText      string
	TTSDuration    int
	TTSSampleHz    int
	TTSAudioBase64 string
	ProviderPath   string
	STTLatencyMs   int64
	LLMLatencyMs   int64
	TTSLatencyMs   int64
	TotalLatencyMs int64
}

// NewOrchestrator は provider 群を受け取り初期化します。
func NewOrchestrator(stt providers.STTProvider, llm providers.LLMProvider, tts providers.TTSProvider, policy providers.CallPolicy) *Orchestrator {
	return &Orchestrator{stt: stt, llm: llm, tts: tts, policy: policy.Normalize()}
}

// ProcessAudioStream は STT -> LLM -> TTS を順に実行します。
// requestID は request_id として全ログ・結果に伝搞されます。
func (o *Orchestrator) ProcessAudioStream(ctx context.Context, sessionID, requestID, streamID string, chunks []providers.AudioChunk) (Result, error) {
	log := logging.FromContext(ctx)
	start := time.Now()

	sttReq := providers.STTRequest{SessionID: sessionID, StreamID: streamID, Chunks: chunks}
	t0 := time.Now()
	sttRes, err := providers.CallWithRetry(ctx, o.policy, func(callCtx context.Context) (providers.STTResult, error) {
		return o.stt.Transcribe(callCtx, sttReq)
	})
	sttLatencyMs := time.Since(t0).Milliseconds()
	if err != nil {
		return Result{}, err
	}
	log.Info().
		Str("provider", o.stt.Name()).
		Str("request_id", requestID).
		Str("transcript", sttRes.Transcript).
		Int64("stt_latency_ms", sttLatencyMs).
		Msg("stt completed")

	llmReq := providers.LLMRequest{SessionID: sessionID, Text: sttRes.Transcript}
	t1 := time.Now()
	llmRes, err := providers.CallWithRetry(ctx, o.policy, func(callCtx context.Context) (providers.LLMResult, error) {
		return o.llm.Generate(callCtx, llmReq)
	})
	llmLatencyMs := time.Since(t1).Milliseconds()
	if err != nil {
		return Result{}, err
	}
	log.Info().
		Str("provider", o.llm.Name()).
		Str("request_id", requestID).
		Str("reply_text", llmRes.ReplyText).
		Int64("llm_latency_ms", llmLatencyMs).
		Msg("llm completed")

	ttsReq := providers.TTSRequest{SessionID: sessionID, Text: llmRes.ReplyText}
	t2 := time.Now()
	ttsRes, err := providers.CallWithRetry(ctx, o.policy, func(callCtx context.Context) (providers.TTSResult, error) {
		return o.tts.Synthesize(callCtx, ttsReq)
	})
	ttsLatencyMs := time.Since(t2).Milliseconds()
	totalLatencyMs := time.Since(start).Milliseconds()
	if err != nil {
		return Result{}, err
	}
	log.Info().
		Str("provider", o.tts.Name()).
		Str("request_id", requestID).
		Int("tts_duration_ms", ttsRes.DurationMs).
		Int("tts_sample_rate_hz", ttsRes.SampleRateHz).
		Int64("stt_latency_ms", sttLatencyMs).
		Int64("llm_latency_ms", llmLatencyMs).
		Int64("tts_latency_ms", ttsLatencyMs).
		Int64("total_latency_ms", totalLatencyMs).
		Msg("orchestration completed")

	return Result{
		RequestID:      requestID,
		Transcript:     sttRes.Transcript,
		ReplyText:      llmRes.ReplyText,
		TTSDuration:    ttsRes.DurationMs,
		TTSSampleHz:    ttsRes.SampleRateHz,
		TTSAudioBase64: ttsRes.AudioBase64,
		ProviderPath:   fmt.Sprintf("%s->%s->%s", o.stt.Name(), o.llm.Name(), o.tts.Name()),
		STTLatencyMs:   sttLatencyMs,
		LLMLatencyMs:   llmLatencyMs,
		TTSLatencyMs:   ttsLatencyMs,
		TotalLatencyMs: totalLatencyMs,
	}, nil
}
