// Package session は WebSocket 接続ごとのセッションライフサイクルを管理します。
// session_id（UUID v4）の生成、セッション状態の保持、切断時のクリーンアップを担います。
package session

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/stackchan/server/internal/protocol"
	"github.com/stackchan/server/internal/providers"
)

// State はセッションの状態を表します。
type State int

const (
	// StateConnected は WebSocket 接続済みだが session.hello が未受信の状態です。
	StateConnected State = iota
	// StateHandshaked は session.hello を受信し welcome を返した後の確立済み状態です。
	StateHandshaked
)

// Session は 1 つの WebSocket 接続に対応するセッション情報です。
type Session struct {
	ID           string
	DeviceID     string
	State        State
	CreatedAt    time.Time
	Ctx          context.Context
	Cancel       context.CancelFunc
	Sequence     *protocol.SequenceTracker
	AudioStreams map[string][]providers.AudioChunk
}

// Manager はセッションのライフサイクルをインメモリで管理します。
// 複数 goroutine から安全に使用できます。
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewManager は Manager を初期化して返します。
func NewManager() *Manager {
	return &Manager{sessions: make(map[string]*Session)}
}

// Create は新しいセッションを生成してストアに追加します。
// parentCtx を元にキャンセル可能なコンテキストをセッションに付与します。
func (m *Manager) Create(parentCtx context.Context) *Session {
	ctx, cancel := context.WithCancel(parentCtx)
	s := &Session{
		ID:           uuid.NewString(),
		State:        StateConnected,
		CreatedAt:    time.Now().UTC(),
		Ctx:          ctx,
		Cancel:       cancel,
		Sequence:     protocol.NewSequenceTracker(),
		AudioStreams: make(map[string][]providers.AudioChunk),
	}
	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()
	return s
}

// Get は session_id でセッションを取得します。
// 存在しない場合は nil を返します。
func (m *Manager) Get(id string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

// Delete はセッションをストアから削除し、セッションのコンテキストをキャンセルします。
// 存在しないセッション ID を指定しても安全です。
func (m *Manager) Delete(id string) {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if ok {
		s.Cancel()
	}
}
