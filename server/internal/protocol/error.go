// Package protocol - error イベントのコード定数とビルダーを定義します。
package protocol

import (
	"encoding/json"
	"fmt"
)

// error コード定数です（protocol/websocket/events.md § 4 Error Handling 参照）。
const (
	ErrCodeInvalidMessage      = "invalid_message"
	ErrCodeUnsupportedVersion  = "unsupported_version"
	ErrCodeInvalidSequence     = "invalid_sequence"
	ErrCodeInvalidPayload      = "invalid_payload"
	ErrCodeSessionRequired     = "session_required"
	ErrCodeProviderUnavailable = "provider_unavailable"
	ErrCodeProviderTimeout     = "provider_timeout"
	ErrCodeProviderFailed      = "provider_failed"
)

// ErrorPayload は error イベントの payload 定義です。
type ErrorPayload struct {
	Code            string  `json:"code"`
	Message         string  `json:"message"`
	Retryable       bool    `json:"retryable"`
	RequestType     *string `json:"request_type,omitempty"`
	RequestSequence *int64  `json:"request_sequence,omitempty"`
}

// NewErrorEnvelope は error イベントのエンベロープを JSON バイト列として生成します。
// 全ハンドラがこの関数を経由して error を送信することで一貫したフォーマットを保証します。
func NewErrorEnvelope(sessionID string, sequence int64, payload ErrorPayload) ([]byte, error) {
	env, err := NewEnvelope("error", sessionID, sequence, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to build error envelope: %w", err)
	}
	data, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal error envelope: %w", err)
	}
	return data, nil
}
