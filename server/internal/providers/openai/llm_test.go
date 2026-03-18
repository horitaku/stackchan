package openai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/horitaku/stackchan/server/internal/providers"
	"github.com/horitaku/stackchan/server/internal/providers/openai"
)

func TestNewLLM_RequiresAPIKey(t *testing.T) {
	_, err := openai.NewLLM(openai.LLMConfig{}, nil)
	if err == nil {
		t.Fatal("expected error when API key is missing")
	}
}

func TestGenerate_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got == "" {
			t.Fatal("missing authorization header")
		}
		_ = json.NewDecoder(r.Body).Decode(&map[string]any{})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{"message": {"role": "assistant", "content": "こんにちは！"}}],
			"usage": {"prompt_tokens": 33, "completion_tokens": 9}
		}`))
	}))
	defer ts.Close()

	p, err := openai.NewLLM(openai.LLMConfig{
		APIKey:  "test-key",
		BaseURL: ts.URL,
		Model:   "gpt-4o-mini",
	}, &http.Client{Timeout: 3 * time.Second})
	if err != nil {
		t.Fatalf("failed to init provider: %v", err)
	}

	res, err := p.Generate(context.Background(), providers.LLMRequest{
		SessionID:    "sess-1",
		RequestID:    "req-1",
		Text:         "元気？",
		SystemPrompt: "あなたは親切です",
		History: []providers.LLMMessage{
			{Role: "user", Content: "おはよう"},
			{Role: "assistant", Content: "おはようございます"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ReplyText == "" {
		t.Fatal("reply text must not be empty")
	}
	if res.TotalTokenCount <= 0 {
		t.Fatal("token count must be positive")
	}
}

func TestGenerate_MapsRateLimitAsTemporary(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"too many requests"}}`))
	}))
	defer ts.Close()

	p, err := openai.NewLLM(openai.LLMConfig{APIKey: "test-key", BaseURL: ts.URL}, nil)
	if err != nil {
		t.Fatalf("failed to init provider: %v", err)
	}

	_, err = p.Generate(context.Background(), providers.LLMRequest{Text: "こんにちは"})
	if err == nil {
		t.Fatal("expected error")
	}
	pe, ok := providers.AsProviderError(err)
	if !ok {
		t.Fatalf("expected provider error, got %T", err)
	}
	if pe.Code != providers.CodeTemporary {
		t.Fatalf("unexpected code: %s", pe.Code)
	}
}
