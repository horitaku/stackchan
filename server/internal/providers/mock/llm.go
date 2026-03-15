package mock

import (
	"context"
	"strings"

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
	return providers.LLMResult{ReplyText: "mock-reply:" + req.Text}, nil
}
