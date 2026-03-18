// Package conversation は STT/LLM/TTS を接続する最小オーケストレーションを提供します。
package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/horitaku/stackchan/server/internal/logging"
	"github.com/horitaku/stackchan/server/internal/providers"
	"github.com/horitaku/stackchan/server/internal/session"
)

const defaultSystemPrompt = "Stack-chan です。話しかけてくれてありがとう。"

// Orchestrator は provider 群を束ねて最小会話フローを実行します。
type Orchestrator struct {
	stt                providers.STTProvider
	llm                providers.LLMProvider
	tts                providers.TTSProvider
	policy             providers.CallPolicy
	llmPolicy          *providers.CallPolicy // nil の場合は policy を使用します
	contextManager     *session.ConversationContextManager
	systemPromptLoader func(sessionID string) string
}

// Result は最小会話フローの結果です。
type Result struct {
	RequestID         string
	Transcript        string
	ReplyText         string
	TTSDuration       int
	TTSSampleHz       int
	TTSAudioBase64    string
	ProviderPath      string
	STTLatencyMs      int64
	LLMLatencyMs      int64
	TTSLatencyMs      int64
	TotalLatencyMs    int64
	LLMInputTokens    int
	LLMOutputTokens   int
	LLMTotalTokens    int
	LLMEffectiveTurns int
}

// NewOrchestrator は provider 群を受け取り初期化します。
func NewOrchestrator(stt providers.STTProvider, llm providers.LLMProvider, tts providers.TTSProvider, policy providers.CallPolicy) *Orchestrator {
	return &Orchestrator{stt: stt, llm: llm, tts: tts, policy: policy.Normalize()}
}

// SetLLMPolicy は LLM 呼び出し専用ポリシーを設定します。
// 未設定の場合は共通の policy が使われます。LLM は STT/TTS より応答が遅いため、
// タイムアウトを長く設定することを推奨します。
func (o *Orchestrator) SetLLMPolicy(p providers.CallPolicy) {
	normalized := p.Normalize()
	o.llmPolicy = &normalized
}

// SetConversationContext は会話履歴コンテキスト管理を設定します。
func (o *Orchestrator) SetConversationContext(manager *session.ConversationContextManager) {
	o.contextManager = manager
}

// SetSystemPromptLoader はセッション向け system prompt 取得関数を設定します。
func (o *Orchestrator) SetSystemPromptLoader(loader func(sessionID string) string) {
	o.systemPromptLoader = loader
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

	return o.processWithTranscript(ctx, sessionID, requestID, streamID, sttRes.Transcript, sttLatencyMs, start)
}

// ProcessText は STT を通さずテキスト入力を会話処理します（UI テスト用）。
func (o *Orchestrator) ProcessText(ctx context.Context, sessionID, requestID, text string, systemPromptOverride string) (Result, error) {
	start := time.Now()
	return o.processWithTranscript(ctx, sessionID, requestID, "", text, 0, start, systemPromptOverride)
}

func (o *Orchestrator) processWithTranscript(
	ctx context.Context,
	sessionID, requestID, streamID, transcript string,
	sttLatencyMs int64,
	start time.Time,
	systemPromptOverride ...string,
) (Result, error) {
	log := logging.FromContext(ctx)

	systemPrompt := o.resolveSystemPrompt(sessionID)
	if len(systemPromptOverride) > 0 {
		override := strings.TrimSpace(systemPromptOverride[0])
		if override != "" {
			systemPrompt = override
		}
	}

	var history []providers.LLMMessage
	inputTokens := 0
	effectiveTurns := 0
	if o.contextManager != nil {
		contextRes := o.contextManager.Build(sessionID, systemPrompt, transcript)
		history = contextRes.History
		inputTokens = contextRes.InputTokens
		effectiveTurns = contextRes.EffectiveTurns
	}

	llmReq := providers.LLMRequest{
		SessionID:    sessionID,
		RequestID:    requestID,
		Text:         transcript,
		SystemPrompt: systemPrompt,
		History:      history,
	}
	// LLM 専用ポリシーが設定されている場合はそちらを優先します
	effectiveLLMPolicy := o.policy
	if o.llmPolicy != nil {
		effectiveLLMPolicy = *o.llmPolicy
	}
	t1 := time.Now()
	llmRes, err := providers.CallWithRetry(ctx, effectiveLLMPolicy, func(callCtx context.Context) (providers.LLMResult, error) {
		return o.llm.Generate(callCtx, llmReq)
	})
	llmLatencyMs := time.Since(t1).Milliseconds()
	if err != nil {
		pe, ok := providers.AsProviderError(err)
		if !ok || pe.Code != providers.CodeTemporary {
			return Result{}, err
		}
		llmRes = providers.LLMResult{
			ReplyText:        "申し訳ありません。今は返答できません。",
			InputTokenCount:  inputTokens,
			OutputTokenCount: 8,
			TotalTokenCount:  inputTokens + 8,
			EffectiveTurns:   effectiveTurns,
		}
	}
	if llmRes.InputTokenCount <= 0 {
		llmRes.InputTokenCount = inputTokens
	}
	if llmRes.TotalTokenCount <= 0 {
		if llmRes.OutputTokenCount <= 0 {
			llmRes.OutputTokenCount = 1
		}
		llmRes.TotalTokenCount = llmRes.InputTokenCount + llmRes.OutputTokenCount
	}
	if llmRes.EffectiveTurns <= 0 {
		llmRes.EffectiveTurns = effectiveTurns
	}
	log.Info().
		Str("provider", o.llm.Name()).
		Str("request_id", requestID).
		Str("reply_text", llmRes.ReplyText).
		Int64("llm_latency_ms", llmLatencyMs).
		Int("llm_input_token_count", llmRes.InputTokenCount).
		Int("llm_output_token_count", llmRes.OutputTokenCount).
		Int("llm_total_token_count", llmRes.TotalTokenCount).
		Int("llm_effective_turns_in_context", llmRes.EffectiveTurns).
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
		Int("llm_input_token_count", llmRes.InputTokenCount).
		Int("llm_output_token_count", llmRes.OutputTokenCount).
		Int("llm_total_token_count", llmRes.TotalTokenCount).
		Int("llm_effective_turns_in_context", llmRes.EffectiveTurns).
		Msg("orchestration completed")

	if o.contextManager != nil {
		o.contextManager.AppendTurn(sessionID, requestID, transcript, llmRes.ReplyText, sttLatencyMs, llmLatencyMs, ttsLatencyMs, totalLatencyMs)
	}

	return Result{
		RequestID:         requestID,
		Transcript:        transcript,
		ReplyText:         llmRes.ReplyText,
		TTSDuration:       ttsRes.DurationMs,
		TTSSampleHz:       ttsRes.SampleRateHz,
		TTSAudioBase64:    ttsRes.AudioBase64,
		ProviderPath:      fmt.Sprintf("%s->%s->%s", o.stt.Name(), o.llm.Name(), o.tts.Name()),
		STTLatencyMs:      sttLatencyMs,
		LLMLatencyMs:      llmLatencyMs,
		TTSLatencyMs:      ttsLatencyMs,
		TotalLatencyMs:    totalLatencyMs,
		LLMInputTokens:    llmRes.InputTokenCount,
		LLMOutputTokens:   llmRes.OutputTokenCount,
		LLMTotalTokens:    llmRes.TotalTokenCount,
		LLMEffectiveTurns: llmRes.EffectiveTurns,
	}, nil
}

func (o *Orchestrator) resolveSystemPrompt(sessionID string) string {
	if o.systemPromptLoader == nil {
		return defaultSystemPrompt
	}
	prompt := strings.TrimSpace(o.systemPromptLoader(sessionID))
	if prompt == "" {
		return defaultSystemPrompt
	}
	return prompt
}
