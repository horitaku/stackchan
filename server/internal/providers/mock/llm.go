package mock

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/horitaku/stackchan/server/internal/providers"
)

// LLM はテスト向けルールベース LLM provider です。
type LLM struct{}

// Name は provider 名を返します。
func (m *LLM) Name() string { return "mock-llm" }

// Generate は入力文字列から定型応答を返します。
func (m *LLM) Generate(_ context.Context, req providers.LLMRequest) (providers.LLMResult, error) {
	if strings.TrimSpace(req.Text) == "" {
		return providers.LLMResult{}, providers.NewError(m.Name(), providers.CodeInvalidInput, "input text is empty", false, nil)
	}
	if strings.Contains(req.Text, "temporary_fail") {
		return providers.LLMResult{}, providers.NewError(m.Name(), providers.CodeTemporary, "temporary llm error", true, nil)
	}
	if strings.Contains(req.Text, "internal_fail") {
		return providers.LLMResult{}, providers.NewError(m.Name(), providers.CodeInternal, "internal llm error", false, nil)
	}
	reply := "mock-reply:" + req.Text
	inputTokens := estimateTokens(req.SystemPrompt) + estimateTokens(req.Text)
	for _, msg := range req.History {
		inputTokens += estimateTokens(msg.Content)
	}
	outputTokens := estimateTokens(reply)
	if inputTokens < 1 {
		inputTokens = 1
	}
	if outputTokens < 1 {
		outputTokens = 1
	}
	return providers.LLMResult{
		ReplyText:        reply,
		InputTokenCount:  inputTokens,
		OutputTokenCount: outputTokens,
		TotalTokenCount:  inputTokens + outputTokens,
		EffectiveTurns:   len(req.History) / 2,
	}, nil
}

func estimateTokens(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	return utf8.RuneCountInString(trimmed)/4 + 1
}
