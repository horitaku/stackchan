// Package session - session.hello の受信処理と session.welcome の返却ロジックを定義します。
// HandleHello は WebSocket I/O に依存せず、返送すべきバイト列と致命的エラーフラグを返します。
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/horitaku/stackchan/server/internal/logging"
	"github.com/horitaku/stackchan/server/internal/protocol"
)

// HelloPayload は session.hello の payload 定義です（events.md § 3.1 参照）。
type HelloPayload struct {
	DeviceID             string                `json:"device_id"`
	ClientType           string                `json:"client_type"`
	ProtocolCapabilities *ProtocolCapabilities `json:"protocol_capabilities,omitempty"`
}

// ProtocolCapabilities はクライアントが対応するプロトコル機能を示します。
type ProtocolCapabilities struct {
	AudioChunk bool `json:"audio_chunk,omitempty"`
	AudioEnd   bool `json:"audio_end,omitempty"`
}

// WelcomePayload は session.welcome の payload 定義です（events.md § 3.2 参照）。
type WelcomePayload struct {
	Accepted            bool    `json:"accepted"`
	ServerTime          string  `json:"server_time"`
	HeartbeatIntervalMs *int    `json:"heartbeat_interval_ms,omitempty"`
	Message             *string `json:"message,omitempty"`
}

// HelloResult は HandleHello の処理結果を表します。
type HelloResult struct {
	// Response は送信すべきメッセージのバイト列です（nil の場合は何も送りません）。
	Response []byte
	// Fatal が true の場合、送信後に接続を切断してください。
	Fatal bool
}

// validClientTypes は session.hello で許容される client_type の値セットです。
var validClientTypes = map[string]struct{}{
	"firmware":     {},
	"test_harness": {},
}

// HandleHello は session.hello エンベロープを検証し、返送すべきメッセージを構築します。
// 成功時は session.welcome を、失敗時は error メッセージを Response に格納します。
// Fatal=true の場合、呼び出し元は Response 送信後に接続を切断してください。
func HandleHello(ctx context.Context, s *Session, env *protocol.Envelope) HelloResult {
	log := logging.FromContext(ctx)

	// payload のパース
	var hello HelloPayload
	if err := json.Unmarshal(env.Payload, &hello); err != nil {
		log.Error().Err(err).Msg("failed to parse session.hello payload")
		return buildHelloError(s, env, protocol.ErrCodeInvalidPayload, "failed to parse session.hello payload", false)
	}

	// device_id 必須チェック
	if hello.DeviceID == "" {
		return buildHelloError(s, env, protocol.ErrCodeInvalidPayload, "device_id is required", false)
	}

	// client_type の値検証
	if _, ok := validClientTypes[hello.ClientType]; !ok {
		return buildHelloError(s, env, protocol.ErrCodeInvalidPayload,
			fmt.Sprintf("invalid client_type: %s (allowed: firmware, test_harness)", hello.ClientType), false)
	}

	// セッション情報を更新します
	s.DeviceID = hello.DeviceID
	s.State = StateHandshaked

	log.Info().
		Str("device_id", hello.DeviceID).
		Str("client_type", hello.ClientType).
		Msg("session.hello accepted")

	return buildWelcome(s)
}

// buildHelloError は検証エラー時の error メッセージを含む HelloResult を構築します。
func buildHelloError(s *Session, env *protocol.Envelope, code, message string, retryable bool) HelloResult {
	reqType := env.Type
	reqSeq := env.Sequence
	payload := protocol.ErrorPayload{
		Code:            code,
		Message:         message,
		Retryable:       retryable,
		RequestType:     &reqType,
		RequestSequence: &reqSeq,
	}
	seq := s.Sequence.NextOutbound()
	data, err := protocol.NewErrorEnvelope(s.ID, seq, payload)
	if err != nil {
		return HelloResult{Fatal: true}
	}
	return HelloResult{Response: data, Fatal: true}
}

// buildWelcome は session.welcome メッセージを含む HelloResult を構築します。
func buildWelcome(s *Session) HelloResult {
	// heartbeat_interval_ms を明示的に設定します（firmware 側のデフォルトより優先されます）
	heartbeatMs := 15000
	welcome := WelcomePayload{
		Accepted:            true,
		ServerTime:          time.Now().UTC().Format(time.RFC3339),
		HeartbeatIntervalMs: &heartbeatMs,
	}
	seq := s.Sequence.NextOutbound()
	env, err := protocol.NewEnvelope("session.welcome", s.ID, seq, welcome)
	if err != nil {
		return HelloResult{Fatal: true}
	}
	data, err := json.Marshal(env)
	if err != nil {
		return HelloResult{Fatal: true}
	}
	return HelloResult{Response: data, Fatal: false}
}
