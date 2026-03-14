// Package protocol はWebSocketプロトコルの共通エンベロープ定義とユーティリティを提供します。
// protocol/websocket/events.md で定義された v0 プロトコル仕様に準拠します。
package protocol

import (
	"encoding/json"
	"fmt"
	"time"
)

// SupportedVersion はこのサーバーがサポートするプロトコルバージョンです。
const SupportedVersion = "1.0"

// Envelope はすべての WebSocket メッセージが持つ共通ラッパーです。
// server->firmware / firmware->server の双方向で使用します。
type Envelope struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	SessionID string          `json:"session_id"`
	Sequence  int64           `json:"sequence"`
	Version   string          `json:"version"`
	Payload   json.RawMessage `json:"payload"`
}

// ParseEnvelope は受信 JSON バイト列をエンベロープにパースします。
// JSON 構文エラーの場合はエラーを返します。
func ParseEnvelope(data []byte) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}
	return &env, nil
}

// Validate はエンベロープの必須フィールドと値の整合性を検証します。
// 以下を検証します: type/timestamp(RFC3339)/sequence(>=1)/version(==SupportedVersion)/payload の存在
func (e *Envelope) Validate() error {
	if e.Type == "" {
		return fmt.Errorf("missing required field: type")
	}
	if e.Timestamp == "" {
		return fmt.Errorf("missing required field: timestamp")
	}
	if _, err := time.Parse(time.RFC3339, e.Timestamp); err != nil {
		return fmt.Errorf("invalid timestamp format (RFC3339 required): %w", err)
	}
	if e.Sequence < 1 {
		return fmt.Errorf("sequence must be >= 1, got %d", e.Sequence)
	}
	if e.Version == "" {
		return fmt.Errorf("missing required field: version")
	}
	if e.Version != SupportedVersion {
		return fmt.Errorf("unsupported version: %s (expected %s)", e.Version, SupportedVersion)
	}
	if len(e.Payload) == 0 {
		return fmt.Errorf("missing required field: payload")
	}
	return nil
}

// NewEnvelope は送信用エンベロープを生成します。
// payload は任意の struct を指定可能で、JSON にシリアライズされます。
func NewEnvelope(msgType, sessionID string, sequence int64, payload any) (*Envelope, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	return &Envelope{
		Type:      msgType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		SessionID: sessionID,
		Sequence:  sequence,
		Version:   SupportedVersion,
		Payload:   payloadBytes,
	}, nil
}
