package session

import (
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/horitaku/stackchan/server/internal/providers"
)

const (
	defaultContextTurns = 5
	defaultMaxTokens    = 2000
)

// ConversationContextResult は LLM 呼び出し時に利用するコンテキストの計算結果です。
type ConversationContextResult struct {
	History        []providers.LLMMessage
	InputTokens    int
	EffectiveTurns int
}

// ConversationContextManager は session ごとの会話履歴を管理します。
type ConversationContextManager struct {
	mu            sync.RWMutex
	maxTurns      int
	maxTokens     int
	inMemoryTurns map[string][]providers.LLMMessage
	warmed        map[string]bool
	store         *UtteranceStore
}

// NewConversationContextManager は会話コンテキスト管理を初期化します。
func NewConversationContextManager(store *UtteranceStore, maxTurns, maxTokens int) *ConversationContextManager {
	if maxTurns <= 0 {
		maxTurns = defaultContextTurns
	}
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	return &ConversationContextManager{
		maxTurns:      maxTurns,
		maxTokens:     maxTokens,
		inMemoryTurns: make(map[string][]providers.LLMMessage),
		warmed:        make(map[string]bool),
		store:         store,
	}
}

// Build は system prompt / user text に対して利用可能な履歴を返します。
func (m *ConversationContextManager) Build(sessionID, systemPrompt, userText string) ConversationContextResult {
	m.warmFromStore(sessionID)

	m.mu.RLock()
	history := append([]providers.LLMMessage(nil), m.inMemoryTurns[sessionID]...)
	m.mu.RUnlock()

	if len(history) == 0 {
		return ConversationContextResult{InputTokens: estimateTokens(systemPrompt) + estimateTokens(userText)}
	}

	if maxMessages := m.maxTurns * 2; maxMessages > 0 && len(history) > maxMessages {
		history = history[len(history)-maxMessages:]
	}

	budget := m.maxTokens - estimateTokens(systemPrompt) - estimateTokens(userText)
	if budget < 0 {
		budget = 0
	}

	selected := make([]providers.LLMMessage, 0, len(history))
	used := 0
	for i := len(history) - 1; i >= 0; i-- {
		tok := estimateTokens(history[i].Content)
		if used+tok > budget {
			continue
		}
		used += tok
		selected = append(selected, history[i])
	}

	for i, j := 0, len(selected)-1; i < j; i, j = i+1, j-1 {
		selected[i], selected[j] = selected[j], selected[i]
	}

	effectiveTurns := len(selected) / 2
	return ConversationContextResult{
		History:        selected,
		InputTokens:    estimateTokens(systemPrompt) + estimateTokens(userText) + used,
		EffectiveTurns: effectiveTurns,
	}
}

// AppendTurn は user/assistant の会話ターンをメモリと DB へ保存します。
func (m *ConversationContextManager) AppendTurn(sessionID, requestID, userText, assistantText string, sttMs, llmMs, ttsMs, totalMs int64) {
	if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(requestID) == "" {
		return
	}
	userText = strings.TrimSpace(userText)
	assistantText = strings.TrimSpace(assistantText)

	m.mu.Lock()
	if userText != "" {
		m.inMemoryTurns[sessionID] = append(m.inMemoryTurns[sessionID], providers.LLMMessage{Role: "user", Content: userText})
	}
	if assistantText != "" {
		m.inMemoryTurns[sessionID] = append(m.inMemoryTurns[sessionID], providers.LLMMessage{Role: "assistant", Content: assistantText})
	}
	if maxMessages := m.maxTurns * 2; maxMessages > 0 && len(m.inMemoryTurns[sessionID]) > maxMessages {
		m.inMemoryTurns[sessionID] = m.inMemoryTurns[sessionID][len(m.inMemoryTurns[sessionID])-maxMessages:]
	}
	m.mu.Unlock()

	if m.store == nil {
		return
	}
	if userText != "" {
		_ = m.store.InsertUtterance(Utterance{
			SessionID:      sessionID,
			RequestID:      requestID,
			Role:           "user",
			Content:        userText,
			STTLatencyMs:   sttMs,
			LLMLatencyMs:   llmMs,
			TTSLatencyMs:   ttsMs,
			TotalLatencyMs: totalMs,
		})
	}
	if assistantText != "" {
		_ = m.store.InsertUtterance(Utterance{
			SessionID:      sessionID,
			RequestID:      requestID,
			Role:           "assistant",
			Content:        assistantText,
			STTLatencyMs:   sttMs,
			LLMLatencyMs:   llmMs,
			TTSLatencyMs:   ttsMs,
			TotalLatencyMs: totalMs,
		})
	}
}

func (m *ConversationContextManager) warmFromStore(sessionID string) {
	if m == nil || m.store == nil || strings.TrimSpace(sessionID) == "" {
		return
	}

	m.mu.RLock()
	already := m.warmed[sessionID]
	m.mu.RUnlock()
	if already {
		return
	}

	items, err := m.store.ListRecentUtterances(sessionID, m.maxTurns*2)
	if err != nil || len(items) == 0 {
		m.mu.Lock()
		m.warmed[sessionID] = true
		m.mu.Unlock()
		return
	}

	loaded := make([]providers.LLMMessage, 0, len(items))
	for _, item := range items {
		if item.Role != "user" && item.Role != "assistant" {
			continue
		}
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		loaded = append(loaded, providers.LLMMessage{Role: item.Role, Content: content})
	}

	m.mu.Lock()
	m.inMemoryTurns[sessionID] = loaded
	m.warmed[sessionID] = true
	m.mu.Unlock()
}

func estimateTokens(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	return utf8.RuneCountInString(trimmed)/4 + 1
}
