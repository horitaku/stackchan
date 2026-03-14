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
	Transcript   string
	ReplyText    string
	TTSDuration  int
	TTSSampleHz  int
	ProviderPath string
}

// NewOrchestrator は provider 群を受け取り初期化します。
func NewOrchestrator(stt providers.STTProvider, llm providers.LLMProvider, tts providers.TTSProvider, policy providers.CallPolicy) *Orchestrator {
	return &Orchestrator{stt: stt, llm: llm, tts: tts, policy: policy.Normalize()}
}

// ProcessAudioStream は STT -> LLM -> TTS を順に実行します。
func (o *Orchestrator) ProcessAudioStream(ctx context.Context, sessionID, streamID string, chunks []providers.AudioChunk) (Result, error) {
	log := logging.FromContext(ctx)
	start := time.Now()

	sttReq := providers.STTRequest{SessionID: sessionID, StreamID: streamID, Chunks: chunks}
	sttRes, err := providers.CallWithRetry(ctx, o.policy, func(callCtx context.Context) (providers.STTResult, error) {
		return o.stt.Transcribe(callCtx, sttReq)
	})
	if err != nil {
		return Result{}, err
	}
	log.Info().Str("provider", o.stt.Name()).Str("transcript", sttRes.Transcript).Msg("stt completed")

	llmReq := providers.LLMRequest{SessionID: sessionID, Text: sttRes.Transcript}
	llmRes, err := providers.CallWithRetry(ctx, o.policy, func(callCtx context.Context) (providers.LLMResult, error) {
		return o.llm.Generate(callCtx, llmReq)
	})
	if err != nil {
		return Result{}, err
	}
	log.Info().Str("provider", o.llm.Name()).Str("reply_text", llmRes.ReplyText).Msg("llm completed")

	ttsReq := providers.TTSRequest{SessionID: sessionID, Text: llmRes.ReplyText}
	ttsRes, err := providers.CallWithRetry(ctx, o.policy, func(callCtx context.Context) (providers.TTSResult, error) {
		return o.tts.Synthesize(callCtx, ttsReq)
	})
	if err != nil {
		return Result{}, err
	}
	log.Info().
		Str("provider", o.tts.Name()).
		Int("tts_duration_ms", ttsRes.DurationMs).
		Int("tts_sample_rate_hz", ttsRes.SampleRateHz).
		Dur("latency_total", time.Since(start)).
		Msg("tts completed")

	return Result{
		Transcript:   sttRes.Transcript,
		ReplyText:    llmRes.ReplyText,
		TTSDuration:  ttsRes.DurationMs,
		TTSSampleHz:  ttsRes.SampleRateHz,
		ProviderPath: fmt.Sprintf("%s->%s->%s", o.stt.Name(), o.llm.Name(), o.tts.Name()),
	}, nil
}
