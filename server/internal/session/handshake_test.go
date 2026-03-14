package session_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stackchan/server/internal/protocol"
	"github.com/stackchan/server/internal/session"
)

// newTestSession はテスト用セッションを生成するヘルパーです。
func newTestSession() (*session.Manager, *session.Session) {
	m := session.NewManager()
	s := m.Create(context.Background())
	return m, s
}

// buildHelloEnvelope は session.hello テスト用エンベロープを組み立てるヘルパーです。
func buildHelloEnvelope(t *testing.T, payload any) *protocol.Envelope {
	t.Helper()
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal hello payload: %v", err)
	}
	return &protocol.Envelope{
		Type:      "session.hello",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		SessionID: "",
		Sequence:  1,
		Version:   "1.0",
		Payload:   payloadBytes,
	}
}

// TestHandleHello_Valid は有効な session.hello に対して session.welcome が返ることを確認します。
func TestHandleHello_Valid(t *testing.T) {
	_, s := newTestSession()
	env := buildHelloEnvelope(t, map[string]any{
		"device_id":   "device-001",
		"client_type": "test_harness",
	})

	result := session.HandleHello(context.Background(), s, env)

	if result.Fatal {
		t.Fatal("expected Fatal=false, got true")
	}
	if result.Response == nil {
		t.Fatal("expected Response, got nil")
	}

	// レスポンスが session.welcome であることを確認します
	var respEnv protocol.Envelope
	if err := json.Unmarshal(result.Response, &respEnv); err != nil {
		t.Fatalf("failed to unmarshal response envelope: %v", err)
	}
	if respEnv.Type != "session.welcome" {
		t.Errorf("expected type=session.welcome, got %s", respEnv.Type)
	}
	if respEnv.SessionID == "" {
		t.Error("expected non-empty session_id in welcome")
	}

	// WelcomePayload の内容を確認します
	var welcome session.WelcomePayload
	if err := json.Unmarshal(respEnv.Payload, &welcome); err != nil {
		t.Fatalf("failed to unmarshal welcome payload: %v", err)
	}
	if !welcome.Accepted {
		t.Error("expected Accepted=true")
	}
	if welcome.ServerTime == "" {
		t.Error("expected non-empty ServerTime")
	}
}

// TestHandleHello_MissingDeviceID は device_id 欠落時に error が返ることを確認します。
func TestHandleHello_MissingDeviceID(t *testing.T) {
	_, s := newTestSession()
	env := buildHelloEnvelope(t, map[string]any{
		"client_type": "firmware",
		// device_id を意図的に省略
	})

	result := session.HandleHello(context.Background(), s, env)

	if !result.Fatal {
		t.Fatal("expected Fatal=true, got false")
	}
	assertErrorCode(t, result.Response, protocol.ErrCodeInvalidPayload)
}

// TestHandleHello_InvalidClientType は不正な client_type に対して error が返ることを確認します。
func TestHandleHello_InvalidClientType(t *testing.T) {
	_, s := newTestSession()
	env := buildHelloEnvelope(t, map[string]any{
		"device_id":   "device-001",
		"client_type": "unknown_type",
	})

	result := session.HandleHello(context.Background(), s, env)

	if !result.Fatal {
		t.Fatal("expected Fatal=true, got false")
	}
	assertErrorCode(t, result.Response, protocol.ErrCodeInvalidPayload)
}

// TestHandleHello_FirmwareClientType は client_type=firmware が有効であることを確認します。
func TestHandleHello_FirmwareClientType(t *testing.T) {
	_, s := newTestSession()
	env := buildHelloEnvelope(t, map[string]any{
		"device_id":   "cores3-01",
		"client_type": "firmware",
	})

	result := session.HandleHello(context.Background(), s, env)

	if result.Fatal {
		t.Fatalf("expected Fatal=false, got true (response=%s)", result.Response)
	}
}

// assertErrorCode は response が指定した error コードを含む error エンベロープであることを検証します。
func assertErrorCode(t *testing.T, response []byte, expectedCode string) {
	t.Helper()
	if response == nil {
		t.Fatal("expected Response, got nil")
	}
	var env protocol.Envelope
	if err := json.Unmarshal(response, &env); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if env.Type != "error" {
		t.Errorf("expected type=error, got %s", env.Type)
	}
	var errPayload protocol.ErrorPayload
	if err := json.Unmarshal(env.Payload, &errPayload); err != nil {
		t.Fatalf("failed to unmarshal error payload: %v", err)
	}
	if errPayload.Code != expectedCode {
		t.Errorf("expected code=%s, got %s", expectedCode, errPayload.Code)
	}
}
