package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/horitaku/stackchan/server/internal/providers"
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
	defaultModel   = "gpt-4o-mini"
)

// LLMConfig は OpenAI LLM provider の設定です。
type LLMConfig struct {
	APIKey      string
	BaseURL     string
	Model       string
	Temperature float64
}

// LLM は OpenAI Chat Completions API を呼び出す provider 実装です。
type LLM struct {
	apiKey      string
	baseURL     string
	model       string
	temperature float64
	client      *http.Client
}

// NewLLM は OpenAI LLM provider を初期化します。
func NewLLM(cfg LLMConfig, client *http.Client) (*LLM, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, providers.NewError("openai-llm", providers.CodeInvalidInput, "OPENAI_API_KEY is required", false, nil)
	}

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = defaultModel
	}

	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}

	return &LLM{
		apiKey:      apiKey,
		baseURL:     baseURL,
		model:       model,
		temperature: cfg.Temperature,
		client:      client,
	}, nil
}

// Name は provider 名を返します。
func (l *LLM) Name() string { return "openai-llm" }

// Generate は Chat Completions API で応答を生成します。
func (l *LLM) Generate(ctx context.Context, req providers.LLMRequest) (providers.LLMResult, error) {
	if strings.TrimSpace(req.Text) == "" {
		return providers.LLMResult{}, providers.NewError(l.Name(), providers.CodeInvalidInput, "input text is empty", false, nil)
	}

	messages := make([]chatMessage, 0, 2+len(req.History))
	systemPrompt := strings.TrimSpace(req.SystemPrompt)
	if systemPrompt != "" {
		messages = append(messages, chatMessage{Role: "system", Content: systemPrompt})
	}
	for _, h := range req.History {
		role := strings.TrimSpace(h.Role)
		if role == "" {
			continue
		}
		content := strings.TrimSpace(h.Content)
		if content == "" {
			continue
		}
		messages = append(messages, chatMessage{Role: role, Content: content})
	}
	messages = append(messages, chatMessage{Role: "user", Content: req.Text})

	payload := chatCompletionsRequest{
		Model:       l.model,
		Messages:    messages,
		Temperature: l.temperature,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return providers.LLMResult{}, providers.NewError(l.Name(), providers.CodeInternal, "failed to encode openai request", false, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, l.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return providers.LLMResult{}, providers.NewError(l.Name(), providers.CodeInternal, "failed to build openai request", false, err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+l.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(httpReq)
	if err != nil {
		if errorsIsTimeout(err) || ctx.Err() != nil {
			return providers.LLMResult{}, providers.NewError(l.Name(), providers.CodeTimeout, "openai request timeout", true, err)
		}
		return providers.LLMResult{}, providers.NewError(l.Name(), providers.CodeTemporary, "openai request failed", true, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return providers.LLMResult{}, providers.NewError(l.Name(), providers.CodeInternal, "failed to read openai response", false, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := extractErrorMessage(respBody)
		switch {
		case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
			return providers.LLMResult{}, providers.NewError(l.Name(), providers.CodeInvalidInput, "openai authentication failed: "+msg, false, nil)
		case resp.StatusCode == http.StatusTooManyRequests:
			return providers.LLMResult{}, providers.NewError(l.Name(), providers.CodeTemporary, "openai rate limited: "+msg, true, nil)
		case resp.StatusCode >= 500:
			return providers.LLMResult{}, providers.NewError(l.Name(), providers.CodeTemporary, "openai upstream error: "+msg, true, nil)
		default:
			return providers.LLMResult{}, providers.NewError(l.Name(), providers.CodeInvalidInput, "openai rejected request: "+msg, false, nil)
		}
	}

	var parsed chatCompletionsResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return providers.LLMResult{}, providers.NewError(l.Name(), providers.CodeInternal, "failed to decode openai response", false, err)
	}
	if len(parsed.Choices) == 0 {
		return providers.LLMResult{}, providers.NewError(l.Name(), providers.CodeInternal, "openai response has no choices", false, nil)
	}

	reply := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if reply == "" {
		reply = "申し訳ありません。今は返答できません。"
	}

	inputTokens := parsed.Usage.PromptTokens
	outputTokens := parsed.Usage.CompletionTokens
	if inputTokens <= 0 {
		inputTokens = estimateTokens(systemPrompt) + estimateTokens(req.Text)
		for _, h := range req.History {
			inputTokens += estimateTokens(h.Content)
		}
	}
	if outputTokens <= 0 {
		outputTokens = estimateTokens(reply)
	}

	return providers.LLMResult{
		ReplyText:        reply,
		InputTokenCount:  inputTokens,
		OutputTokenCount: outputTokens,
		TotalTokenCount:  inputTokens + outputTokens,
		EffectiveTurns:   len(req.History) / 2,
	}, nil
}

type chatCompletionsRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionsResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

type chatErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func extractErrorMessage(body []byte) string {
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		return "unknown error"
	}
	var parsed chatErrorResponse
	if err := json.Unmarshal(body, &parsed); err == nil {
		if m := strings.TrimSpace(parsed.Error.Message); m != "" {
			return m
		}
	}
	if len(msg) > 220 {
		return msg[:220]
	}
	return msg
}

func errorsIsTimeout(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded")
}

func estimateTokens(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	return utf8.RuneCountInString(trimmed)/4 + 1
}
